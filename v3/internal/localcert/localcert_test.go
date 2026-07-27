package localcert

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestEnsureCreatesStableCAAndValidServerCertificate(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	cfg := Config{
		Domain:    "ipv6.moonkey.top",
		Directory: t.TempDir(),
		Now:       func() time.Time { return now },
	}
	first, err := Ensure(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Renewed {
		t.Fatal("first ensure did not create a server certificate")
	}
	second, err := Ensure(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Renewed {
		t.Fatal("second ensure unexpectedly renewed the server certificate")
	}
	if first.CAPEM != second.CAPEM {
		t.Fatal("local CA changed between calls")
	}

	pair, err := tls.LoadX509KeyPair(first.CertFile, first.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.PublicKeyAlgorithm != x509.ECDSA {
		t.Fatalf("server certificate algorithm = %s, want ECDSA", leaf.PublicKeyAlgorithm)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(first.CAPEM)) {
		t.Fatal("could not load generated CA")
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		DNSName:     cfg.Domain,
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureReportsLegacyEd25519CA(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Legacy VideoWithYou CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/owner-ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/owner-ca-key.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Ensure(Config{Domain: "ipv6.moonkey.top", Directory: dir})
	if !errors.Is(err, ErrUnsupportedCAAlgorithm) {
		t.Fatalf("Ensure() error = %v, want ErrUnsupportedCAAlgorithm", err)
	}
}

func TestEnsureRejectsIncompleteCA(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/owner-ca.pem", []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Ensure(Config{Domain: "ipv6.moonkey.top", Directory: dir})
	if err == nil {
		t.Fatal("expected incomplete CA error")
	}
}
