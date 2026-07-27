package localcert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	caLifetime        = 10 * 365 * 24 * time.Hour
	serverLifetime    = 397 * 24 * time.Hour
	serverRenewBefore = 45 * 24 * time.Hour
)

var ErrUnsupportedCAAlgorithm = errors.New("unsupported local CA key algorithm")

type Config struct {
	Domain    string
	Directory string
	CAFile    string
	CAKeyFile string
	CertFile  string
	KeyFile   string
	Now       func() time.Time
}

type Result struct {
	CAPEM     string
	CertFile  string
	KeyFile   string
	CAFile    string
	CAKeyFile string
	NotAfter  time.Time
	Renewed   bool
}

func Ensure(cfg Config) (Result, error) {
	if cfg.Domain == "" {
		return Result{}, errors.New("local CA domain is empty")
	}
	if cfg.Directory == "" {
		return Result{}, errors.New("local CA directory is empty")
	}
	applyDefaultPaths(&cfg)
	if err := os.MkdirAll(cfg.Directory, 0o700); err != nil {
		return Result{}, err
	}
	now := time.Now()
	if cfg.Now != nil {
		now = cfg.Now()
	}

	caCert, caKey, caPEM, err := ensureCA(cfg, now)
	if err != nil {
		return Result{}, err
	}
	notAfter, renewed, err := ensureServerCertificate(cfg, caCert, caKey, now)
	if err != nil {
		return Result{}, err
	}
	return Result{
		CAPEM:     string(caPEM),
		CertFile:  cfg.CertFile,
		KeyFile:   cfg.KeyFile,
		CAFile:    cfg.CAFile,
		CAKeyFile: cfg.CAKeyFile,
		NotAfter:  notAfter,
		Renewed:   renewed,
	}, nil
}

func applyDefaultPaths(cfg *Config) {
	if cfg.CAFile == "" {
		cfg.CAFile = filepath.Join(cfg.Directory, "owner-ca.pem")
	}
	if cfg.CAKeyFile == "" {
		cfg.CAKeyFile = filepath.Join(cfg.Directory, "owner-ca-key.pem")
	}
	if cfg.CertFile == "" {
		cfg.CertFile = filepath.Join(cfg.Directory, "server.pem")
	}
	if cfg.KeyFile == "" {
		cfg.KeyFile = filepath.Join(cfg.Directory, "server-key.pem")
	}
}

func ensureCA(cfg Config, now time.Time) (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	certExists := fileExists(cfg.CAFile)
	keyExists := fileExists(cfg.CAKeyFile)
	if certExists != keyExists {
		return nil, nil, nil, errors.New("local CA certificate/key pair is incomplete")
	}
	if certExists {
		certPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, nil, nil, err
		}
		keyPEM, err := os.ReadFile(cfg.CAKeyFile)
		if err != nil {
			return nil, nil, nil, err
		}
		cert, key, err := parseCA(certPEM, keyPEM)
		if err != nil {
			return nil, nil, nil, err
		}
		if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return nil, nil, nil, errors.New("local CA certificate is not currently valid")
		}
		return cert, key, certPEM, nil
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "VideoWithYou Owner CA", Organization: []string{"VideoWithYou"}},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(caLifetime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := writeAtomic(cfg.CAKeyFile, keyPEM, 0o600); err != nil {
		return nil, nil, nil, err
	}
	if err := writeAtomic(cfg.CAFile, certPEM, 0o600); err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}
	return cert, privateKey, certPEM, nil
}

func ensureServerCertificate(cfg Config, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, now time.Time) (time.Time, bool, error) {
	certExists := fileExists(cfg.CertFile)
	keyExists := fileExists(cfg.KeyFile)
	if certExists && keyExists {
		pair, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err == nil && len(pair.Certificate) > 0 {
			leaf, parseErr := x509.ParseCertificate(pair.Certificate[0])
			if parseErr == nil {
				roots := x509.NewCertPool()
				roots.AddCert(caCert)
				_, verifyErr := leaf.Verify(x509.VerifyOptions{
					DNSName:     cfg.Domain,
					Roots:       roots,
					CurrentTime: now,
					KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				})
				if verifyErr == nil && leaf.NotAfter.After(now.Add(serverRenewBefore)) {
					return leaf.NotAfter, false, nil
				}
			}
		}
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return time.Time{}, false, err
	}
	serial, err := randomSerial()
	if err != nil {
		return time.Time{}, false, err
	}
	notAfter := now.Add(serverLifetime)
	if caLimit := caCert.NotAfter.Add(-24 * time.Hour); notAfter.After(caLimit) {
		notAfter = caLimit
	}
	if !notAfter.After(now.Add(serverRenewBefore)) {
		return time.Time{}, false, errors.New("local CA expires too soon to issue a server certificate")
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cfg.Domain, Organization: []string{"VideoWithYou"}},
		DNSNames:     []string{cfg.Domain},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return time.Time{}, false, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return time.Time{}, false, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := writeAtomic(cfg.KeyFile, keyPEM, 0o600); err != nil {
		return time.Time{}, false, err
	}
	if err := writeAtomic(cfg.CertFile, certPEM, 0o600); err != nil {
		return time.Time{}, false, err
	}
	return notAfter, true, nil
}

func parseCA(certPEM, keyPEM []byte) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, errors.New("invalid local CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	if !cert.IsCA {
		return nil, nil, errors.New("local CA certificate is not a CA")
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid local CA private key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	privateKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("%w: local CA private key is not ECDSA", ErrUnsupportedCAAlgorithm)
	}
	publicKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("%w: local CA certificate public key is not ECDSA", ErrUnsupportedCAAlgorithm)
	}
	if publicKey.Curve != elliptic.P256() || privateKey.Curve != elliptic.P256() {
		return nil, nil, fmt.Errorf("%w: local CA must use ECDSA P-256", ErrUnsupportedCAAlgorithm)
	}
	if publicKey.X.Cmp(privateKey.X) != 0 || publicKey.Y.Cmp(privateKey.Y) != 0 {
		return nil, nil, errors.New("local CA certificate and private key do not match")
	}
	return cert, privateKey, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	if serial.Sign() == 0 {
		return big.NewInt(1), nil
	}
	return serial, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(perm); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return os.Rename(tempPath, path)
}
