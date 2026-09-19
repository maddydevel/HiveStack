package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	hivetls "github.com/maddydevel/HiveStack/internal/tls"
)

const (
	// shutdownTimeout bounds how long Shutdown waits for in-flight requests.
	shutdownTimeout = 30 * time.Second

	// readHeaderTimeout guards against slow-header (Slowloris) clients.
	readHeaderTimeout = 10 * time.Second

	// idleTimeout closes keep-alive connections that have gone quiet.
	idleTimeout = 120 * time.Second

	// devCertMinValidity is the remaining lifetime below which an existing
	// development certificate is regenerated instead of reused.
	devCertMinValidity = 30 * 24 * time.Hour
)

// setupHTTPServer builds the http.Server and, when TLS is enabled, loads the
// TLS configuration. It runs from New so a bad certificate fails startup
// rather than surfacing later from Run.
func (s *APIServer) setupHTTPServer() error {
	addr := fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	if s.Config.Server.TLSEnabled {
		tlsCfg, err := s.loadTLSConfig()
		if err != nil {
			return fmt.Errorf("configure TLS: %w", err)
		}
		s.tlsConfig = tlsCfg
		srv.TLSConfig = tlsCfg
	}

	s.httpServer = srv
	return nil
}

// loadTLSConfig resolves the certificate paths (generating a self-signed
// certificate in development mode) and loads the REST TLS configuration.
func (s *APIServer) loadTLSConfig() (*tls.Config, error) {
	certs := s.Config.TLS
	if s.Config.Server.TLSDevMode {
		generated, err := s.generateDevCerts()
		if err != nil {
			return nil, fmt.Errorf("generate development certificate: %w", err)
		}
		certs = generated
	}
	if certs.CertFile == "" || certs.KeyFile == "" {
		return nil, errors.New("TLS enabled but cert_file and key_file are not set")
	}
	return hivetls.LoadRESTTLSConfig(certs.CertFile, certs.KeyFile, certs.CAFile)
}

// generateDevCerts creates a throwaway CA and a server certificate signed by
// it, for use with Server.TLSDevMode. It must not be used in production.
//
// Files are written to the directory of Config.TLS.CertFile when set, and to a
// fresh temporary directory otherwise. When Config.TLS already points at a
// certificate that is not close to expiry, it is reused so restarts keep the
// same identity. The CA private key is deleted after signing; the CA
// certificate (CAFile in the result) is kept so clients can trust the server.
func (s *APIServer) generateDevCerts() (hivetls.CertConfig, error) {
	cfg := s.Config.TLS
	if cfg.CertFile != "" && cfg.KeyFile != "" && devCertUsable(cfg) {
		return cfg, nil
	}

	var dir string
	if cfg.CertFile != "" {
		dir = filepath.Dir(cfg.CertFile)
	} else {
		tmp, err := os.MkdirTemp("", "hivestack-dev-tls-")
		if err != nil {
			return hivetls.CertConfig{}, fmt.Errorf("create temp dir: %w", err)
		}
		dir = tmp
	}

	out := hivetls.CertConfig{
		CertFile: cfg.CertFile,
		KeyFile:  cfg.KeyFile,
		CAFile:   filepath.Join(dir, "ca.crt"),
	}
	if out.CertFile == "" {
		out.CertFile = filepath.Join(dir, "server.crt")
	}
	if out.KeyFile == "" {
		out.KeyFile = filepath.Join(dir, "server.key")
	}
	caKeyFile := filepath.Join(dir, "ca.key")

	if err := hivetls.GenerateCA(out.CAFile, caKeyFile, "HiveStack Development CA"); err != nil {
		return hivetls.CertConfig{}, err
	}
	// The CA key is only needed to sign the server certificate below.
	defer os.Remove(caKeyFile)

	caCert, caKey, err := hivetls.LoadCA(out.CAFile, caKeyFile)
	if err != nil {
		return hivetls.CertConfig{}, err
	}

	dnsNames, ips := s.devCertSANs()
	if err := hivetls.GenerateServerCert(caCert, caKey, out.CertFile, out.KeyFile, "localhost", dnsNames, ips); err != nil {
		return hivetls.CertConfig{}, err
	}

	fmt.Printf("WARNING: TLS dev mode enabled: using self-signed certificate %s (trust CA %s); do not use in production\n",
		out.CertFile, out.CAFile)
	return out, nil
}

// devCertSANs returns the DNS names and IP addresses a development
// certificate should be valid for: loopback, the machine hostname, and the
// configured listen host when it is a specific name or address.
func (s *APIServer) devCertSANs() ([]string, []net.IP) {
	dnsNames := []string{"localhost"}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		dnsNames = append(dnsNames, hostname)
	}

	host := s.Config.Server.Host
	if host == "" || host == "localhost" {
		return dnsNames, ips
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsUnspecified() {
			ips = append(ips, ip)
		}
	} else {
		dnsNames = append(dnsNames, host)
	}
	return dnsNames, ips
}

// devCertUsable reports whether the certificate and key named by cfg exist and
// the certificate has more than devCertMinValidity left.
func devCertUsable(cfg hivetls.CertConfig) bool {
	if _, err := os.Stat(cfg.KeyFile); err != nil {
		return false
	}
	expiry, err := hivetls.CertExpiry(cfg.CertFile)
	if err != nil {
		return false
	}
	return time.Until(expiry) > devCertMinValidity
}

// Run starts the API server and blocks until it stops. It returns nil after a
// clean Shutdown.
func (s *APIServer) Run() error {
	if s.httpServer == nil {
		return errors.New("API server not initialised: create it with New")
	}

	var err error
	if s.tlsConfig != nil {
		fmt.Printf("HiveStack API server starting on https://%s (v%s)\n", s.httpServer.Addr, Version)
		// Certificates come from TLSConfig, so the file arguments stay empty.
		err = s.httpServer.ListenAndServeTLS("", "")
	} else {
		fmt.Printf("HiveStack API server starting on http://%s (v%s)\n", s.httpServer.Addr, Version)
		err = s.httpServer.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server, waiting up to shutdownTimeout for
// in-flight requests before closing remaining connections.
func (s *APIServer) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		_ = s.httpServer.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
