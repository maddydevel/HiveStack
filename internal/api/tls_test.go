package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/auth"
	hivetls "github.com/maddydevel/HiveStack/internal/tls"
)

// newTLSTestAPI builds an APIServer (bypassing New, which needs *db.DB) and
// runs the transport setup for the given config.
func newTLSTestAPI(t *testing.T, cfg *Config) (*APIServer, error) {
	t.Helper()
	s := &APIServer{
		Config: cfg,
		rbac:   auth.NewRBACEngine(),
		mux:    new(http.ServeMux),
	}
	s.registerRoutes()
	return s, s.setupHTTPServer()
}

// freePort returns a TCP port that was free a moment ago.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestSetupHTTPServer(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		server  ServerConfig
		certs   hivetls.CertConfig
		wantErr bool
		wantTLS bool
	}{
		{
			name:    "TLS disabled by default",
			server:  ServerConfig{Host: "127.0.0.1", Port: 8080},
			wantTLS: false,
		},
		{
			name:    "TLS enabled without cert paths",
			server:  ServerConfig{Host: "127.0.0.1", Port: 8080, TLSEnabled: true},
			wantErr: true,
		},
		{
			name:    "TLS enabled with missing files",
			server:  ServerConfig{Host: "127.0.0.1", Port: 8080, TLSEnabled: true},
			certs:   hivetls.CertConfig{CertFile: filepath.Join(dir, "nope.crt"), KeyFile: filepath.Join(dir, "nope.key")},
			wantErr: true,
		},
		{
			name:    "dev mode ignored when TLS disabled",
			server:  ServerConfig{Host: "127.0.0.1", Port: 8080, TLSDevMode: true},
			wantTLS: false,
		},
		{
			name:    "dev mode generates certificate",
			server:  ServerConfig{Host: "127.0.0.1", Port: 8080, TLSEnabled: true, TLSDevMode: true},
			wantTLS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newTLSTestAPI(t, &Config{Server: tt.server, TLS: tt.certs})
			if (err != nil) != tt.wantErr {
				t.Fatalf("setupHTTPServer error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got := s.tlsConfig != nil; got != tt.wantTLS {
				t.Errorf("tlsConfig set = %v, want %v", got, tt.wantTLS)
			}
			if s.httpServer == nil || s.httpServer.Handler == nil {
				t.Fatal("httpServer not initialised")
			}
			if tt.wantTLS {
				if s.tlsConfig.MinVersion != tls.VersionTLS13 {
					t.Errorf("MinVersion = %#x, want TLS 1.3", s.tlsConfig.MinVersion)
				}
				if s.httpServer.TLSConfig != s.tlsConfig {
					t.Error("httpServer.TLSConfig not wired to tlsConfig")
				}
			}
		})
	}
}

func TestGenerateDevCerts(t *testing.T) {
	t.Run("temp dir when no paths configured", func(t *testing.T) {
		s, err := newTLSTestAPI(t, &Config{Server: ServerConfig{Host: "127.0.0.1", TLSEnabled: true, TLSDevMode: true}})
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		certs, err := s.generateDevCerts()
		if err != nil {
			t.Fatalf("generateDevCerts: %v", err)
		}
		defer os.RemoveAll(filepath.Dir(certs.CertFile))
		for _, p := range []string{certs.CertFile, certs.KeyFile, certs.CAFile} {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("expected %s to exist: %v", p, err)
			}
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(certs.CAFile), "ca.key")); !os.IsNotExist(err) {
			t.Errorf("CA private key should be removed after signing, stat err = %v", err)
		}
	})

	t.Run("configured paths are reused while valid", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &Config{
			Server: ServerConfig{Host: "127.0.0.1", TLSEnabled: true, TLSDevMode: true},
			TLS: hivetls.CertConfig{
				CertFile: filepath.Join(dir, "server.crt"),
				KeyFile:  filepath.Join(dir, "server.key"),
			},
		}
		s, err := newTLSTestAPI(t, cfg)
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		before, err := os.ReadFile(cfg.TLS.CertFile)
		if err != nil {
			t.Fatalf("read generated cert: %v", err)
		}
		if _, err := s.generateDevCerts(); err != nil {
			t.Fatalf("generateDevCerts: %v", err)
		}
		after, err := os.ReadFile(cfg.TLS.CertFile)
		if err != nil {
			t.Fatalf("read cert: %v", err)
		}
		if string(before) != string(after) {
			t.Error("valid dev certificate was regenerated instead of reused")
		}
	})
}

func TestDevCertSANs(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantDNS string
		wantIP  string
		absent  string
	}{
		{name: "wildcard host", host: "0.0.0.0", wantDNS: "localhost", wantIP: "127.0.0.1", absent: "0.0.0.0"},
		{name: "specific IP", host: "10.1.2.3", wantIP: "10.1.2.3"},
		{name: "DNS name", host: "manager.example.com", wantDNS: "manager.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &APIServer{Config: &Config{Server: ServerConfig{Host: tt.host}}}
			dns, ips := s.devCertSANs()
			if tt.wantDNS != "" && !containsString(dns, tt.wantDNS) {
				t.Errorf("DNS names %v missing %q", dns, tt.wantDNS)
			}
			if tt.wantIP != "" && !containsIP(ips, tt.wantIP) {
				t.Errorf("IPs %v missing %s", ips, tt.wantIP)
			}
			if tt.absent != "" && containsIP(ips, tt.absent) {
				t.Errorf("IPs %v should not contain %s", ips, tt.absent)
			}
		})
	}
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func containsIP(list []net.IP, want string) bool {
	w := net.ParseIP(want)
	for _, ip := range list {
		if ip.Equal(w) {
			return true
		}
	}
	return false
}

// TestRunTLSAndShutdown starts a real HTTPS listener with a generated
// certificate, verifies a client that trusts the dev CA can reach /health,
// and checks Shutdown makes Run return nil.
func TestRunTLSAndShutdown(t *testing.T) {
	port := freePort(t)
	dir := t.TempDir()
	s, err := newTLSTestAPI(t, &Config{
		Server: ServerConfig{Host: "127.0.0.1", Port: port, TLSEnabled: true, TLSDevMode: true},
		TLS: hivetls.CertConfig{
			CertFile: filepath.Join(dir, "server.crt"),
			KeyFile:  filepath.Join(dir, "server.key"),
		},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Dev mode writes the CA certificate next to the server certificate.
	certs := hivetls.CertConfig{CAFile: filepath.Join(dir, "ca.crt")}

	caPEM, err := os.ReadFile(certs.CAFile)
	if err != nil {
		t.Fatalf("read CA: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatal("append CA")
	}
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}},
	}

	runErr := make(chan error, 1)
	go func() { runErr <- s.Run() }()

	url := fmt.Sprintf("https://127.0.0.1:%d/health", port)
	var resp *http.Response
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err = client.Get(url)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", url, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if resp.TLS == nil || resp.TLS.Version != tls.VersionTLS13 {
		t.Errorf("negotiated TLS state = %+v, want TLS 1.3", resp.TLS)
	}

	// Plain HTTP against the TLS port must not succeed.
	if r, err := (&http.Client{Timeout: 2 * time.Second}).Get(fmt.Sprintf("http://127.0.0.1:%d/health", port)); err == nil {
		r.Body.Close()
		if r.StatusCode == http.StatusOK {
			t.Error("plain HTTP request unexpectedly succeeded on TLS port")
		}
	}

	if err := s.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run returned %v after Shutdown, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}
}

func TestRunPlainHTTPAndShutdown(t *testing.T) {
	port := freePort(t)
	s, err := newTLSTestAPI(t, &Config{Server: ServerConfig{Host: "127.0.0.1", Port: port}})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	runErr := make(chan error, 1)
	go func() { runErr <- s.Run() }()

	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", url, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := s.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-runErr; err != nil {
		t.Errorf("Run returned %v after Shutdown, want nil", err)
	}
}

func TestRunWithoutSetup(t *testing.T) {
	s := &APIServer{Config: &Config{}}
	if err := s.Run(); err == nil {
		t.Error("Run on an uninitialised server should fail")
	}
	if err := s.Shutdown(); err != nil {
		t.Errorf("Shutdown on an uninitialised server = %v, want nil", err)
	}
}
