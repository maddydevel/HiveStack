// Package tls provides TLS configuration for the HiveStack Node Agent.
//
// The Node Agent uses mTLS to authenticate with the Manager. This file
// provides:
//   - Client certificate loading for mTLS
//   - TLS 1.3 configuration for gRPC connections to the Manager
//   - Certificate verification against the trusted CA
package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// NodeTLSConfig holds the TLS configuration for the Node Agent.
type NodeTLSConfig struct {
	// CertFile is the path to the client certificate PEM file.
	CertFile string
	// KeyFile is the path to the client private key PEM file.
	KeyFile string
	// CAFile is the path to the CA certificate for verifying the Manager.
	CAFile string
	// ServerName is the expected server name in the Manager's certificate.
	ServerName string
}

// DefaultNodeTLSConfig returns a NodeTLSConfig with sensible defaults.
func DefaultNodeTLSConfig() NodeTLSConfig {
	return NodeTLSConfig{
		CertFile:   "/etc/hivestack/tls/client.crt",
		KeyFile:    "/etc/hivestack/tls/client.key",
		CAFile:     "/etc/hivestack/tls/ca.crt",
		ServerName: "hivestack-manager",
	}
}

// BuildTLSConfig constructs a *tls.Config for the Node Agent.
//
// The configuration:
//   - Enforces TLS 1.3 only
//   - Loads the client certificate for mTLS
//   - Loads the CA certificate for verifying the Manager's server cert
//   - Sets ServerName for SNI and certificate verification
func (c NodeTLSConfig) BuildTLSConfig() (*tls.Config, error) {
	cert, err := LoadCertificate(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	caCertPool, err := loadCAPool(c.CAFile)
	if err != nil {
		return nil, fmt.Errorf("load CA pool: %w", err)
	}

	serverName := c.ServerName
	if serverName == "" {
		serverName = "hivestack-manager"
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		ServerName:   serverName,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		PreferServerCipherSuites: true,
	}, nil
}

// BuildGRPCConfig constructs a *tls.Config specifically for gRPC connections.
//
// gRPC has its own TLS handling, so this config is slightly different from
// the standard HTTP config. It uses the same mTLS settings but is optimized
// for gRPC's connection pooling and multiplexing.
func (c NodeTLSConfig) BuildGRPCConfig() (*tls.Config, error) {
	cfg, err := c.BuildTLSConfig()
	if err != nil {
		return nil, err
	}
	// gRPC-specific: disable session tickets for perfect forward secrecy
	cfg.SessionTicketsDisabled = true
	return cfg, nil
}

// CertInfo returns information about the loaded client certificate.
func (c NodeTLSConfig) CertInfo() (subject string, expiry string, err error) {
	certPEM, err := os.ReadFile(c.CertFile)
	if err != nil {
		return "", "", fmt.Errorf("read cert: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", "", fmt.Errorf("decode cert PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", "", fmt.Errorf("parse cert: %w", err)
	}

	return cert.Subject.String(), cert.NotAfter.Format("2006-01-02"), nil
}
