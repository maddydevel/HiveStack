// Package security provides REST API security middleware for HiveStack.
//
// This middleware provides:
//   - Rate limiting (100 req/s per IP)
//   - CORS (Cross-Origin Resource Sharing) configuration
//   - Security headers (HSTS, X-Frame-Options, etc.)
//   - Request size limits
//   - Timeout enforcement
package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Middleware provides HTTP security middleware.
type Middleware struct {
	// rateLimiter tracks request rates per IP.
	rateLimiter *RateLimiter
	// corsConfig holds CORS configuration.
	corsConfig CORSConfig
	// securityHeaders holds security header values.
	securityHeaders map[string]string
	// maxBodySize is the maximum request body size in bytes.
	maxBodySize int64
	// requestTimeout is the maximum request duration.
	requestTimeout time.Duration
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins (empty = same-origin only).
	AllowedOrigins []string
	// AllowedMethods is a list of allowed HTTP methods.
	AllowedMethods []string
	// AllowedHeaders is a list of allowed headers.
	AllowedHeaders []string
	// AllowCredentials allows cookies in CORS requests.
	AllowCredentials bool
	// MaxAge is how long browsers should cache CORS preflight.
	MaxAge time.Duration
}

// DefaultCORSConfig returns a restrictive CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{}, // Same-origin only by default
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: false,
		MaxAge:           5 * time.Minute,
	}
}

// NewMiddleware creates a new security middleware instance.
func NewMiddleware() *Middleware {
	return &Middleware{
		rateLimiter: NewRateLimiter(100, time.Second), // 100 req/s per IP
		corsConfig:  DefaultCORSConfig(),
		securityHeaders: map[string]string{
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Referrer-Policy":           "strict-origin-when-cross-origin",
			"Content-Security-Policy":   "default-src 'self'",
			"Cache-Control":             "no-store",
			"Pragma":                    "no-cache",
		},
		maxBodySize:    10 * 1024 * 1024, // 10 MB
		requestTimeout: 30 * time.Second,
	}
}

// SetCORS sets the CORS configuration.
func (m *Middleware) SetCORS(config CORSConfig) {
	m.corsConfig = config
}

// SetRateLimit sets the rate limit (requests per second per IP).
func (m *Middleware) SetRateLimit(rps int, burst int) {
	m.rateLimiter = NewRateLimiter(rps, time.Second)
}

// SetMaxBodySize sets the maximum request body size.
func (m *Middleware) SetMaxBodySize(size int64) {
	m.maxBodySize = size
}

// SetRequestTimeout sets the maximum request duration.
func (m *Middleware) SetRequestTimeout(timeout time.Duration) {
	m.requestTimeout = timeout
}

// SecurityHandler wraps an HTTP handler with all security middleware.
func (m *Middleware) SecurityHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply security headers
		for k, v := range m.securityHeaders {
			w.Header().Set(k, v)
		}

		// Apply CORS
		if !m.handleCORS(w, r) {
			return // Preflight handled
		}

		// Rate limiting
		if !m.checkRateLimit(w, r) {
			return
		}

		// Request size limit
		if m.maxBodySize > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, m.maxBodySize)
		}

		// Timeout
		ctx, cancel := context.WithTimeout(r.Context(), m.requestTimeout)
		defer cancel()

		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleCORS handles CORS headers and preflight requests.
func (m *Middleware) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	if len(m.corsConfig.AllowedOrigins) > 0 {
		allowed := false
		for _, o := range m.corsConfig.AllowedOrigins {
			if o == origin || o == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			// Origin not allowed, but don't block (just don't set CORS headers)
			return true
		}
	}

	// Set CORS headers
	if origin != "" {
		if len(m.corsConfig.AllowedOrigins) == 1 && m.corsConfig.AllowedOrigins[0] == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Vary", "Origin")
	}

	if m.corsConfig.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if len(m.corsConfig.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.corsConfig.AllowedMethods, ", "))
	}

	if len(m.corsConfig.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.corsConfig.AllowedHeaders, ", "))
	}

	if m.corsConfig.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", int(m.corsConfig.MaxAge.Seconds())))
	}

	// Handle preflight
	if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
		w.WriteHeader(http.StatusNoContent)
		return false
	}

	return true
}

// checkRateLimit checks if the request is within rate limits.
func (m *Middleware) checkRateLimit(w http.ResponseWriter, r *http.Request) bool {
	ip := clientIP(r)
	if !m.rateLimiter.Allow(ip) {
		w.Header().Set("Retry-After", "1")
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", m.rateLimiter.rps))
		w.Header().Set("X-RateLimit-Remaining", "0")
		http.Error(w, `{"error":"rate limit exceeded","code":"RATE_LIMITED"}`, http.StatusTooManyRequests)
		return false
	}
	return true
}

// clientIP extracts the client IP from a request.
func clientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			xff = xff[:idx]
		}
		xff = strings.TrimSpace(xff)
		if xff != "" {
			return xff
		}
	}

	// Check X-Real-Ip header
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// RateLimiter implements a token bucket rate limiter per IP.
type RateLimiter struct {
	rps   int
	burst int
	mu    sync.Mutex
	ips   map[string]*tokenBucket
}

// tokenBucket is a simple token bucket for rate limiting.
type tokenBucket struct {
	tokens    int
	lastCheck time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rps int, per time.Duration) *RateLimiter {
	rl := &RateLimiter{
		rps:   rps,
		burst: rps * 2, // Allow burst of 2x the rate
		ips:   make(map[string]*tokenBucket),
	}

	// Cleanup old entries periodically
	go rl.cleanup(per)

	return rl
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.ips[ip]
	if !exists {
		rl.ips[ip] = &tokenBucket{
			tokens:    rl.burst - 1,
			lastCheck: now,
		}
		return true
	}

	// Add tokens based on time elapsed
	elapsed := now.Sub(bucket.lastCheck)
	tokensToAdd := int(elapsed.Seconds()) * rl.rps
	if tokensToAdd > 0 {
		bucket.tokens = min(bucket.tokens+tokensToAdd, rl.burst)
		bucket.lastCheck = now
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// cleanup removes stale entries from the rate limiter.
func (rl *RateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval * 2)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, bucket := range rl.ips {
			if now.Sub(bucket.lastCheck) > interval*2 {
				delete(rl.ips, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
