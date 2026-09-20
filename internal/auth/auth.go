// Package auth provides authentication (Argon2id password hashing, JWT token
// issuance/validation) and role-based access control (RBAC) for HiveStack.
//
// All auth is tenant-scoped: every user, token, and RBAC check is associated
// with a tenant so a single Manager instance can serve multiple customers.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// Claims embedded in JWT tokens.
type Claims struct {
	UserID       string   `json:"uid"`
	TenantID     string   `json:"tid"`
	Scopes       []string `json:"scp"`
	RefreshToken string   `json:"rt,omitempty"`
	jwt.RegisteredClaims
}

// DefaultExpiry is the default token lifetime.
const DefaultExpiry = 24 * time.Hour

// ErrInvalidCredentials is returned when login fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUnauthorized is returned when a token is invalid or expired.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when the caller lacks required permissions.
var ErrForbidden = errors.New("forbidden")

// argon2Params holds the Argon2id parameters used for password hashing.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
)

// refreshTokenRecord holds the stored data for a refresh token.
type refreshTokenRecord struct {
	userID    string
	tenantID  string
	scopes    []string
	expiresAt time.Time
}

// refreshTokenStore is the in-memory store for refresh tokens (keyed by SHA-256 hash).
var (
	refreshTokenStore = make(map[string]refreshTokenRecord)
	refreshTokenMu    sync.RWMutex
)

// HashPassword hashes a plaintext password using Argon2id.
// Returns a string in the format: $argon2id$v=19$m=65536,t=3,p=4$salt$hash
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, uint32(argonMemory), uint8(argonThreads), argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// CheckPassword verifies a plaintext password against an Argon2id hash.
func CheckPassword(hash, password string) (bool, error) {
	p, s, hb, err := parseHash(hash)
	if err != nil {
		return false, err
	}
	expected := argon2.IDKey([]byte(password), s, p.time, p.memory, p.threads, p.keyLen)
	return subtle.ConstantTimeCompare(hb, expected) == 1, nil
}

// hashParams holds parsed Argon2id parameters.
type hashParams struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

// parseHash parses an Argon2id hash string into its components.
func parseHash(hash string) (hashParams, []byte, []byte, error) {
	var p hashParams
	_, err := fmt.Sscanf(hash, "$argon2id$v=19$m=%d,t=%d,p=%d$", &p.memory, &p.time, &p.threads)
	if err != nil || p.memory == 0 {
		return p, nil, nil, fmt.Errorf("invalid hash format: %w", err)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" {
		return p, nil, nil, errors.New("invalid hash: wrong format")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	hashBytes, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, fmt.Errorf("decode hash: %w", err)
	}
	p.keyLen = uint32(len(hashBytes))
	return p, salt, hashBytes, nil
}

// GenerateToken creates a signed JWT for a user within a tenant.
func GenerateToken(userID, tenantID string, scopes []string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = DefaultExpiry
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Scopes:   scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "hivestack",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := os.Getenv("HIVESTACK_JWT_SECRET")
	if secret == "" {
		return "", errors.New("HIVESTACK_JWT_SECRET not set")
	}
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT token string.
func ValidateToken(tokenString string) (*Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		secret := os.Getenv("HIVESTACK_JWT_SECRET")
		if secret == "" {
			return nil, errors.New("HIVESTACK_JWT_SECRET not set")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return nil, ErrUnauthorized
	}
	return &claims, nil
}

// GenerateRefreshToken creates a new opaque refresh token for a user.
// The raw token is returned to the caller; only its SHA-256 hash is stored.
// The token expires in 7 days.
func GenerateRefreshToken(userID, tenantID string, scopes []string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	refreshTokenMu.Lock()
	defer refreshTokenMu.Unlock()
	refreshTokenStore[hash] = refreshTokenRecord{
		userID:    userID,
		tenantID:  tenantID,
		scopes:    scopes,
		expiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	return token, nil
}

// ValidateRefreshToken validates a raw refresh token, rotates it (deletes the
// old token and issues a new one), and returns the new token with the
// associated claims. Returns ErrUnauthorized if the token is invalid or expired.
func ValidateRefreshToken(token string) (newToken string, claims *Claims, err error) {
	if token == "" {
		return "", nil, ErrUnauthorized
	}

	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	refreshTokenMu.Lock()
	defer refreshTokenMu.Unlock()

	record, ok := refreshTokenStore[hash]
	if !ok {
		return "", nil, ErrUnauthorized
	}

	// Check expiry
	if time.Now().After(record.expiresAt) {
		delete(refreshTokenStore, hash)
		return "", nil, ErrUnauthorized
	}

	// Delete old token (rotation)
	delete(refreshTokenStore, hash)

	// Issue new token
	newRaw := make([]byte, 32)
	if _, err := rand.Read(newRaw); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	newTokenStr := base64.RawURLEncoding.EncodeToString(newRaw)

	newHash := sha256.Sum256([]byte(newTokenStr))
	newHashStr := hex.EncodeToString(newHash[:])

	refreshTokenStore[newHashStr] = refreshTokenRecord{
		userID:    record.userID,
		tenantID:  record.tenantID,
		scopes:    record.scopes,
		expiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	claims = &Claims{
		UserID:   record.userID,
		TenantID: record.tenantID,
		Scopes:   record.scopes,
	}

	return newTokenStr, claims, nil
}

// GenerateTokenPair creates an access token (15-minute expiry) and a refresh
// token (7-day expiry). Returns (accessToken, refreshToken, error).
func GenerateTokenPair(userID, tenantID string, scopes []string) (accessToken string, refreshToken string, err error) {
	accessToken, err = GenerateToken(userID, tenantID, scopes, 15*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err = GenerateRefreshToken(userID, tenantID, scopes)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}

// RequireAuth is middleware that extracts and validates a JWT from the
// Authorization header. On success it stores the Claims in the request context.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "code": "UNAUTHORIZED"})
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := ValidateToken(tokenStr)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error(), "code": "UNAUTHORIZED"})
			return
		}
		ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
		next(w, r.WithContext(ctx))
	}
}

// claimsContextKey is the context key for stored JWT claims.
type claimsContextKey struct{}

// ClaimsFromContext extracts JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return claims, ok
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
