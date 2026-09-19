package node

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/maddydevel/HiveStack/internal/config"
	hivetls "github.com/maddydevel/HiveStack/internal/tls"
)

// ServerTLSOption returns the grpc.ServerOption that makes the node agent's gRPC
// server require mutual TLS, using the certificate, key and CA in cfg.
//
// It returns a nil option and no error when cfg is empty, meaning TLS is not
// configured and the caller decides whether to serve plaintext. A partially
// filled cfg is an error rather than a silent fallback to plaintext.
func ServerTLSOption(cfg config.TLSConfig) (grpc.ServerOption, error) {
	if cfg.CertFile == "" && cfg.KeyFile == "" && cfg.CAFile == "" {
		return nil, nil
	}
	if cfg.CertFile == "" || cfg.KeyFile == "" || cfg.CAFile == "" {
		return nil, fmt.Errorf("incomplete TLS config: cert_file, key_file and ca_file must all be set")
	}
	creds, err := hivetls.ServerCredentials(cfg.CertFile, cfg.KeyFile, cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("load gRPC server TLS credentials: %w", err)
	}
	return grpc.Creds(creds), nil
}

// DialTLS connects to the node agent at addr like Dial, but over the given
// transport credentials, typically from tls.ClientCredentials for mutual TLS.
func DialTLS(ctx context.Context, addr, nodeID string, creds credentials.TransportCredentials, opts ...grpc.DialOption) (*Client, error) {
	if creds == nil {
		return nil, fmt.Errorf("transport credentials are required")
	}
	return Dial(ctx, addr, nodeID, append([]grpc.DialOption{grpc.WithTransportCredentials(creds)}, opts...)...)
}
