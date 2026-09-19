package tls

import (
	"crypto/tls"
	"fmt"

	"google.golang.org/grpc/credentials"
)

// ServerCredentials returns gRPC transport credentials for a server that
// requires mutual TLS: it presents the certificate at certPath and only accepts
// clients whose certificate is signed by the CA at caPath.
func ServerCredentials(certPath, keyPath, caPath string) (credentials.TransportCredentials, error) {
	cfg, err := LoadTLSConfig(certPath, keyPath, caPath)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(cfg), nil
}

// LoadClientTLSConfig loads the TLS configuration for a gRPC client that
// authenticates with the certificate at certPath and verifies the server against
// the CA at caPath. serverName is the name expected in the server's certificate;
// when empty, gRPC derives it from the dial target.
func LoadClientTLSConfig(certPath, keyPath, caPath, serverName string) (*tls.Config, error) {
	cert, err := LoadCertificate(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client cert/key: %w", err)
	}

	caPool, err := LoadCAPool(caPath)
	if err != nil {
		return nil, fmt.Errorf("load CA pool: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}, nil
}

// ClientCredentials returns gRPC transport credentials for a client that
// presents its own certificate (mutual TLS) and verifies the server's. See
// LoadClientTLSConfig for the meaning of the arguments.
func ClientCredentials(certPath, keyPath, caPath, serverName string) (credentials.TransportCredentials, error) {
	cfg, err := LoadClientTLSConfig(certPath, keyPath, caPath, serverName)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(cfg), nil
}
