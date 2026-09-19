package node

import (
	"context"
	"crypto/tls"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/maddydevel/HiveStack/internal/config"
	hivetls "github.com/maddydevel/HiveStack/internal/tls"
)

// testPKI is a CA with a server certificate for 127.0.0.1 and a client
// certificate, all written under dir.
type testPKI struct {
	config.TLSConfig // the server side: server cert/key and the CA
	clientCert       string
	clientKey        string
}

func newTestPKI(t *testing.T, name string) testPKI {
	t.Helper()
	dir := t.TempDir()
	p := func(f string) string { return filepath.Join(dir, f) }

	if err := hivetls.GenerateCA(p("ca.crt"), p("ca.key"), name+"-ca"); err != nil {
		t.Fatal(err)
	}
	caCert, caKey, err := hivetls.LoadCA(p("ca.crt"), p("ca.key"))
	if err != nil {
		t.Fatal(err)
	}
	err = hivetls.GenerateServerCert(caCert, caKey, p("server.crt"), p("server.key"),
		name+"-node", []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	if err := hivetls.GenerateClientCert(caCert, caKey, p("client.crt"), p("client.key"), name+"-manager"); err != nil {
		t.Fatal(err)
	}
	return testPKI{
		TLSConfig:  config.TLSConfig{CertFile: p("server.crt"), KeyFile: p("server.key"), CAFile: p("ca.crt")},
		clientCert: p("client.crt"),
		clientKey:  p("client.key"),
	}
}

// startTLSServer starts a node gRPC server requiring mutual TLS with pki's
// server credentials and returns its address. The agent is running.
func startTLSServer(t *testing.T, pki testPKI) string {
	t.Helper()

	agent, err := New(&config.NodeConfig{LibvirtURI: "test:///default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.libvirt.Connect(); err != nil {
		t.Fatal(err)
	}
	agent.mu.Lock()
	agent.running = true
	agent.mu.Unlock()

	opt, err := ServerTLSOption(pki.TLSConfig)
	if err != nil || opt == nil {
		t.Fatalf("ServerTLSOption = %v, %v", opt, err)
	}
	gs, err := NewGRPCServer(agent, "127.0.0.1:0", opt)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := gs.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	t.Cleanup(gs.Stop)
	return gs.lis.Addr().String()
}

func TestServerTLSOption(t *testing.T) {
	pki := newTestPKI(t, "opt")

	tests := []struct {
		name    string
		cfg     config.TLSConfig
		wantOpt bool
		wantErr bool
	}{
		{"unconfigured", config.TLSConfig{}, false, false},
		{"complete", pki.TLSConfig, true, false},
		{"missing CA", config.TLSConfig{CertFile: pki.CertFile, KeyFile: pki.KeyFile}, false, true},
		{"missing cert", config.TLSConfig{KeyFile: pki.KeyFile, CAFile: pki.CAFile}, false, true},
		{"unreadable files", config.TLSConfig{CertFile: "/nonexistent/c", KeyFile: "/nonexistent/k", CAFile: "/nonexistent/ca"}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := ServerTLSOption(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if (opt != nil) != tt.wantOpt {
				t.Fatalf("option = %v, want non-nil %v", opt, tt.wantOpt)
			}
		})
	}
}

func TestDialTLS_RequiresCredentials(t *testing.T) {
	if _, err := DialTLS(context.Background(), "127.0.0.1:0", "node-1", nil); err == nil {
		t.Fatal("expected error for nil credentials")
	}
}

func TestMutualTLS(t *testing.T) {
	pki := newTestPKI(t, "good")
	other := newTestPKI(t, "other")
	addr := startTLSServer(t, pki)

	clientCreds := func(t *testing.T, cert, key, ca string) credentials.TransportCredentials {
		t.Helper()
		creds, err := hivetls.ClientCredentials(cert, key, ca, "")
		if err != nil {
			t.Fatal(err)
		}
		return creds
	}

	// A client with the CA-trusted certificate, but not trusting the server:
	// the server CA is unknown, so it must reject the server.
	untrustingServer := clientCreds(t, pki.clientCert, pki.clientKey, other.CAFile)
	// A client that trusts the server but presents a certificate from another CA.
	foreignClientCert := clientCreds(t, other.clientCert, other.clientKey, pki.CAFile)
	// A client that trusts the server but presents no certificate at all.
	pool, err := hivetls.LoadCAPool(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	noClientCert := credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13})

	tests := []struct {
		name    string
		dial    func(ctx context.Context) (*Client, error)
		wantErr bool
	}{
		{"valid mTLS client", func(ctx context.Context) (*Client, error) {
			return DialTLS(ctx, addr, "node-1", clientCreds(t, pki.clientCert, pki.clientKey, pki.CAFile))
		}, false},
		{"client does not trust server CA", func(ctx context.Context) (*Client, error) {
			return DialTLS(ctx, addr, "node-1", untrustingServer)
		}, true},
		{"client cert from untrusted CA", func(ctx context.Context) (*Client, error) {
			return DialTLS(ctx, addr, "node-1", foreignClientCert)
		}, true},
		{"no client cert", func(ctx context.Context) (*Client, error) {
			return DialTLS(ctx, addr, "node-1", noClientCert)
		}, true},
		{"plaintext client", func(ctx context.Context) (*Client, error) {
			return Dial(ctx, addr, "node-1", grpc.WithTransportCredentials(insecure.NewCredentials()))
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			client, err := tt.dial(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()

			err = client.StartVM(ctx, "vm1")
			if tt.wantErr && err == nil {
				t.Fatal("StartVM succeeded, want a connection/handshake error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("StartVM: %v", err)
			}
		})
	}
}
