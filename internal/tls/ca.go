// Package tls provides TLS certificate management for HiveStack.
package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// GenerateCA generates a new CA certificate and key at the specified paths.
// The commonName will be used as the Subject CN of the CA certificate.
// The CA is valid for 10 years.
func GenerateCA(certPath, keyPath, commonName string) error {
	if commonName == "" {
		commonName = "HiveStack Root CA"
	}

	// Generate ECDSA key (P-256 for balance of security and performance)
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate CA private key: %w", err)
	}

	// Create CA template
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
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Self-sign the CA certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("create CA certificate: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("create cert directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}

	// Write CA cert
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open CA cert file: %w", err)
	}

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certFile.Close()
		return fmt.Errorf("encode CA cert: %w", err)
	}
	certFile.Close()

	// Write CA key
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open CA key file: %w", err)
	}

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		keyFile.Close()
		return fmt.Errorf("marshal CA private key: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyFile.Close()
		return fmt.Errorf("encode CA key: %w", err)
	}
	keyFile.Close()

	return nil
}

// LoadCA loads a CA certificate from a PEM file and returns the parsed cert and key.
func LoadCA(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	caCertPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA cert: %w", err)
	}

	caKeyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA key: %w", err)
	}

	certBlock, _ := pem.Decode(caCertPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("decode CA cert PEM")
	}

	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("decode CA key PEM")
	}

	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}

	return caCert, caKey, nil
}
