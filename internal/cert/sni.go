package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// leafValidity is how long a per-SNI leaf certificate remains valid.
const leafValidity = 365 * 24 * time.Hour

// NewSNIGetCertificate returns a callback for tls.Config.GetCertificate that
// mints (and caches) a leaf certificate per TLS handshake, keyed on the SNI
// server name and signed by the dev CA at the given paths.
//
// This makes any "<slug>.localhost" host verify without regeneration and
// without a "*.localhost" wildcard. Strict TLS clients reject that wildcard
// because its only remaining label is the single-label suffix "localhost"
// (they require at least two labels after the "*"), so the generated server
// cert validates for bare "localhost" but not for "feature-auth.localhost" —
// exactly the hosts portree routes on.
//
// The CA cert and key are loaded once; each distinct server name is minted on
// first use and cached, so repeat handshakes are cheap. Minting is restricted
// to the hosts portree routes ("localhost" and "<slug>.localhost"); any other
// SNI collapses to the loopback leaf, so an untrusted client cannot drive
// unbounded key generation or cache growth with arbitrary server names.
func NewSNIGetCertificate(paths CertPaths) (func(*tls.ClientHelloInfo) (*tls.Certificate, error), error) {
	caCert, caKey, err := loadCA(paths)
	if err != nil {
		return nil, err
	}

	var (
		mu    sync.RWMutex
		cache = map[string]*tls.Certificate{}
	)

	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// Only mint for hosts portree actually serves. Empty SNI (a client
		// connecting by IP), or any non-localhost name, falls back to the
		// loopback leaf so the handshake still completes without minting a
		// certificate per arbitrary name.
		name := hello.ServerName
		if name != "localhost" && !strings.HasSuffix(name, ".localhost") {
			name = "localhost"
		}

		mu.RLock()
		leaf, ok := cache[name]
		mu.RUnlock()
		if ok {
			return leaf, nil
		}

		leaf, err := mintLeaf(name, caCert, caKey)
		if err != nil {
			return nil, err
		}

		// Double-check under the write lock so concurrent handshakes for the
		// same new host settle on a single cached certificate rather than
		// minting twice.
		mu.Lock()
		if existing, ok := cache[name]; ok {
			leaf = existing
		} else {
			cache[name] = leaf
		}
		mu.Unlock()

		return leaf, nil
	}, nil
}

// mintLeaf creates a leaf certificate for serverName signed by the CA.
func mintLeaf(serverName string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) (*tls.Certificate, error) {
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating leaf key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	// Always include bare "localhost" so loopback handshakes keep working,
	// avoiding a duplicate SAN when the requested name already is "localhost".
	dnsNames := []string{serverName}
	if serverName != "localhost" {
		dnsNames = append(dnsNames, "localhost")
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   serverName,
			Organization: []string{"Portree"},
		},
		DNSNames:    dnsNames,
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:   now,
		NotAfter:    now.Add(leafValidity),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating leaf certificate for %q: %w", serverName, err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parsing leaf certificate: %w", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  leafKey,
		Leaf:        leaf,
	}, nil
}

// loadCA loads and parses the CA certificate and private key from disk.
func loadCA(paths CertPaths) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(paths.CACert)
	if err != nil {
		return nil, nil, fmt.Errorf("reading CA cert: %w", err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("decoding CA cert PEM from %s", paths.CACert)
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA cert: %w", err)
	}

	keyPEM, err := os.ReadFile(paths.CAKey)
	if err != nil {
		return nil, nil, fmt.Errorf("reading CA key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("decoding CA key PEM from %s", paths.CAKey)
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA key: %w", err)
	}

	return caCert, caKey, nil
}
