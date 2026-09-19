// Package tls provides TLS certificate management for HiveStack.
//
// This package handles CA certificate generation, server certificate generation,
// client certificate generation for mTLS, and TLS configuration for both
// the Manager and Node Agent.
package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// TLSManager provides TLS configuration management for the HiveStack Manager.
type TLSManager struct {
	certPath string
	keyPath  string
	caPath   string
}

// NewTLSManager creates a new TLSManager with the specified certificate paths.
func NewTLSManager(certPath, keyPath, caPath string) *TLSManager {
	return &TLSManager{
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,
	}
}

// LoadTLSConfig loads a TLS configuration for the Manager gRPC server.
// Uses TLS 1.3 minimum, strong cipher suites, and mutual TLS for gRPC.
func LoadTLSConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load server cert/key: %w", err)
	}

	caCertPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCertPEM); !ok {
		return nil, fmt.Errorf("failed to append CA cert to pool")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
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
	}

	return config, nil
}

// LoadRESTTLSConfig loads TLS configuration for the REST API server.
// Uses TLS 1.3 with standard web cipher suites but no mTLS (clients use tokens).
func LoadRESTTLSConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load server cert/key: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
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
	}

	// Optionally verify client certs if CA path is provided (for admin API)
	if caPath != "" {
		caCertPEM, err := os.ReadFile(caPath)
		if err == nil && len(caCertPEM) > 0 {
			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(caCertPEM); ok {
				config.ClientCAs = caCertPool
				config.ClientAuth = tls.VerifyClientCertIfGiven
			}
		}
	}

	return config, nil
}

// GenerateServerCert generates a server certificate signed by the given CA.
// Used for Manager's gRPC and REST endpoints.
func GenerateServerCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, serverCertPath, serverKeyPath, commonName string, dnsNames []string, ips []net.IP) error {
	return generateCert(caCert, caKey, serverCertPath, serverKeyPath, commonName, dnsNames, ips, false)
}

// GenerateClientCert generates a client certificate signed by the given CA.
// Used for Node Agent mTLS authentication to the Manager.
func GenerateClientCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, clientCertPath, clientKeyPath, commonName string) error {
	return generateCert(caCert, caKey, clientCertPath, clientKeyPath, commonName, nil, nil, true)
}

// generateCert is the internal helper for certificate generation.
func generateCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, certPath, keyPath, commonName string, dnsNames []string, ips []net.IP, isClient bool) error {
	// Generate ECDSA key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ECDSA key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"HiveStack"},
			Country:      []string{"US"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{},
		DNSNames:    dnsNames,
		IPAddresses: ips,
	}

	if isClient {
		template.KeyUsage |= x509.KeyUsageKeyEncipherment
		template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
	} else {
		template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	// Write cert
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("create cert directory: %w", err)
	}

	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("encode cert: %w", err)
	}

	// Write key
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}

	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open key file: %w", err)
	}
	defer keyFile.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("encode key: %w", err)
	}

	return nil
}

// CertExpiry returns the expiry time of a certificate at the given path.
func CertExpiry(certPath string) (time.Time, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("read cert: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("decode cert PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cert: %w", err)
	}

	return cert.NotAfter, nil
}
