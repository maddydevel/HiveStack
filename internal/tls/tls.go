// Package tls provides TLS certificate management for HiveStack components.
//
// It handles:
//   - CA certificate generation and loading
//   - Server certificate loading with TLS 1.3 configuration for the Manager
//   - Client certificate loading with mTLS for Node Agents
//   - gRPC mTLS configuration for both Manager and Node communication
package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// CertConfig holds paths to certificate files.
type CertConfig struct {
	// CertFile is the path to the certificate PEM file.
	CertFile string
	// KeyFile is the path to the private key PEM file.
	KeyFile string
	// CAFile is the path to the CA certificate PEM file.
	CAFile string
}

// LoadCertificate loads a certificate and key from PEM files.
func LoadCertificate(certFile, keyFile string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load certificate pair: %w", err)
	}
	return cert, nil
}

// loadCAPool reads a CA certificate file and adds it to a x509.CertPool.
func loadCAPool(caFile string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", caFile)
	}
	return pool, nil
}
