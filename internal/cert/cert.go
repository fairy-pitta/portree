package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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

// CertPaths holds the file paths for generated certificates and keys.
type CertPaths struct {
	CACert     string
	CAKey      string
	ServerCert string
	ServerKey  string
}

// EnsureCerts ensures that CA and server certificates exist in dir.
// If all four files (ca.crt, ca.key, server.crt, server.key) exist, it returns
// their paths without regenerating. Otherwise, it generates all files.
func EnsureCerts(dir string) (CertPaths, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return CertPaths{}, fmt.Errorf("creating cert directory: %w", err)
	}

	paths := CertPaths{
		CACert:     filepath.Join(dir, "ca.crt"),
		CAKey:      filepath.Join(dir, "ca.key"),
		ServerCert: filepath.Join(dir, "server.crt"),
		ServerKey:  filepath.Join(dir, "server.key"),
	}

	// Check if all files exist.
	if fileExists(paths.CACert) && fileExists(paths.CAKey) &&
		fileExists(paths.ServerCert) && fileExists(paths.ServerKey) {
		return paths, nil
	}

	// Generate CA.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return CertPaths{}, fmt.Errorf("generating CA key: %w", err)
	}

	caSerial, err := randomSerial()
	if err != nil {
		return CertPaths{}, err
	}

	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			CommonName:   "Portree Dev CA",
			Organization: []string{"Portree"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return CertPaths{}, fmt.Errorf("creating CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return CertPaths{}, fmt.Errorf("parsing CA certificate: %w", err)
	}

	// Generate server certificate.
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return CertPaths{}, fmt.Errorf("generating server key: %w", err)
	}

	serverSerial, err := randomSerial()
	if err != nil {
		return CertPaths{}, err
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Portree"},
		},
		DNSNames:    []string{"*.localhost", "localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:   now,
		NotAfter:    now.Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return CertPaths{}, fmt.Errorf("creating server certificate: %w", err)
	}

	// Write files.
	if err := writePEM(paths.CACert, "CERTIFICATE", caCertDER, 0644); err != nil {
		return CertPaths{}, fmt.Errorf("writing CA cert: %w", err)
	}
	if err := writeKeyPEM(paths.CAKey, caKey); err != nil {
		return CertPaths{}, fmt.Errorf("writing CA key: %w", err)
	}
	if err := writePEM(paths.ServerCert, "CERTIFICATE", serverCertDER, 0644); err != nil {
		return CertPaths{}, fmt.Errorf("writing server cert: %w", err)
	}
	if err := writeKeyPEM(paths.ServerKey, serverKey); err != nil {
		return CertPaths{}, fmt.Errorf("writing server key: %w", err)
	}

	return paths, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial number: %w", err)
	}
	return serial, nil
}

func writePEM(path, blockType string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: data})
}

func writeKeyPEM(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(path, "EC PRIVATE KEY", der, 0600)
}
