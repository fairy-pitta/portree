package cert

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCerts_GeneratesValidCA(t *testing.T) {
	dir := t.TempDir()
	paths, err := EnsureCerts(dir)
	if err != nil {
		t.Fatalf("EnsureCerts() error: %v", err)
	}

	// Parse CA cert.
	caPEM, err := os.ReadFile(paths.CACert)
	if err != nil {
		t.Fatalf("reading CA cert: %v", err)
	}
	block, _ := pem.Decode(caPEM)
	if block == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing CA cert: %v", err)
	}

	if !ca.IsCA {
		t.Error("CA cert should have IsCA=true")
	}
	if ca.Subject.CommonName != "Portree Dev CA" {
		t.Errorf("CA CN = %q, want %q", ca.Subject.CommonName, "Portree Dev CA")
	}
	if ca.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("CA should have KeyUsageCertSign")
	}
	if ca.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Error("CA should have KeyUsageCRLSign")
	}
}

func TestEnsureCerts_GeneratesValidServerCert(t *testing.T) {
	dir := t.TempDir()
	paths, err := EnsureCerts(dir)
	if err != nil {
		t.Fatalf("EnsureCerts() error: %v", err)
	}

	// Parse server cert.
	serverPEM, err := os.ReadFile(paths.ServerCert)
	if err != nil {
		t.Fatalf("reading server cert: %v", err)
	}
	block, _ := pem.Decode(serverPEM)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	server, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing server cert: %v", err)
	}

	// Check SANs.
	wantDNS := []string{"*.localhost", "localhost"}
	if len(server.DNSNames) != len(wantDNS) {
		t.Fatalf("DNSNames = %v, want %v", server.DNSNames, wantDNS)
	}
	for i, name := range wantDNS {
		if server.DNSNames[i] != name {
			t.Errorf("DNSNames[%d] = %q, want %q", i, server.DNSNames[i], name)
		}
	}

	// Check IP SANs.
	foundV4, foundV6 := false, false
	for _, ip := range server.IPAddresses {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			foundV4 = true
		}
		if ip.Equal(net.IPv6loopback) {
			foundV6 = true
		}
	}
	if !foundV4 {
		t.Error("server cert missing 127.0.0.1 IP SAN")
	}
	if !foundV6 {
		t.Error("server cert missing ::1 IP SAN")
	}

	// Check ExtKeyUsage.
	foundServerAuth := false
	for _, usage := range server.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			foundServerAuth = true
		}
	}
	if !foundServerAuth {
		t.Error("server cert missing ExtKeyUsageServerAuth")
	}

	// Verify chain: server cert signed by CA.
	caPEM, err := os.ReadFile(paths.CACert)
	if err != nil {
		t.Fatalf("reading CA cert: %v", err)
	}
	caBlock, _ := pem.Decode(caPEM)
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parsing CA cert: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, err := server.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Errorf("server cert chain verification failed: %v", err)
	}
}

func TestEnsureCerts_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// First call generates certs.
	paths1, err := EnsureCerts(dir)
	if err != nil {
		t.Fatalf("first EnsureCerts() error: %v", err)
	}

	// Record modification times.
	info1, _ := os.Stat(paths1.CACert)
	modTime1 := info1.ModTime()

	// Second call should skip regeneration.
	paths2, err := EnsureCerts(dir)
	if err != nil {
		t.Fatalf("second EnsureCerts() error: %v", err)
	}

	info2, _ := os.Stat(paths2.CACert)
	modTime2 := info2.ModTime()

	if !modTime1.Equal(modTime2) {
		t.Error("EnsureCerts should not regenerate when all files exist")
	}
}

func TestEnsureCerts_RegeneratesOnMissing(t *testing.T) {
	dir := t.TempDir()

	// First call generates certs.
	paths, err := EnsureCerts(dir)
	if err != nil {
		t.Fatalf("first EnsureCerts() error: %v", err)
	}

	// Remove one file.
	if err := os.Remove(paths.ServerKey); err != nil {
		t.Fatalf("removing server key: %v", err)
	}

	// Second call should regenerate all files.
	_, err = EnsureCerts(dir)
	if err != nil {
		t.Fatalf("second EnsureCerts() error: %v", err)
	}

	// Verify file exists again.
	if !fileExists(filepath.Join(dir, "server.key")) {
		t.Error("server.key should be regenerated")
	}
}

func TestEnsureCerts_KeyFilePermissions(t *testing.T) {
	dir := t.TempDir()
	paths, err := EnsureCerts(dir)
	if err != nil {
		t.Fatalf("EnsureCerts() error: %v", err)
	}

	for _, keyPath := range []string{paths.CAKey, paths.ServerKey} {
		info, err := os.Stat(keyPath)
		if err != nil {
			t.Fatalf("stat %s: %v", keyPath, err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("%s permissions = %o, want 0600", keyPath, perm)
		}
	}
}
