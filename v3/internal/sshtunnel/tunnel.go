package sshtunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Address             string
	User                string
	PrivateKeyPath      string
	HostKeyPinPath      string
	RemoteListenAddress string
	ReconnectDelay      time.Duration
}

type Status struct {
	Connected     bool
	Error         string
	RemoteAddress string
	HostKeyPin    string
}

type Tunnel struct {
	cfg Config
	log *log.Logger

	mu       sync.RWMutex
	status   Status
	onStatus func(Status)
}

func New(cfg Config, logger *log.Logger) *Tunnel {
	if logger == nil {
		logger = log.Default()
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = 3 * time.Second
	}
	return &Tunnel{cfg: cfg, log: logger}
}

func (t *Tunnel) SetOnStatus(callback func(Status)) {
	t.mu.Lock()
	t.onStatus = callback
	status := t.status
	t.mu.Unlock()
	if callback != nil {
		callback(status)
	}
}

func (t *Tunnel) Start(ctx context.Context, serve func(context.Context, net.Listener) error) {
	go t.run(ctx, serve)
}

func (t *Tunnel) run(ctx context.Context, serve func(context.Context, net.Listener) error) {
	for {
		if ctx.Err() != nil {
			return
		}
		listener, client, hostPin, err := t.connect(ctx)
		if err != nil {
			t.log.Printf("[隧道] SSH 反向转发连接失败 error=%q", err)
			t.updateStatus(Status{Error: err.Error()})
			if !waitContext(ctx, t.cfg.ReconnectDelay) {
				return
			}
			continue
		}

		t.log.Printf("[隧道] SSH 反向转发已建立 remote=%s", t.cfg.RemoteListenAddress)
		t.updateStatus(Status{
			Connected:     true,
			RemoteAddress: t.cfg.RemoteListenAddress,
			HostKeyPin:    hostPin,
		})

		serveCtx, cancelServe := context.WithCancel(ctx)
		closeDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = listener.Close()
				_ = client.Close()
			case <-closeDone:
			}
		}()
		err = serve(serveCtx, listener)
		close(closeDone)
		cancelServe()
		_ = listener.Close()
		_ = client.Close()
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = errors.New("SSH remote listener stopped")
		}
		t.log.Printf("[隧道] SSH 反向转发中断 error=%q", err)
		t.updateStatus(Status{Error: err.Error(), HostKeyPin: hostPin})
		if !waitContext(ctx, t.cfg.ReconnectDelay) {
			return
		}
	}
}

func (t *Tunnel) connect(ctx context.Context) (net.Listener, *ssh.Client, string, error) {
	privateKey, err := os.ReadFile(t.cfg.PrivateKeyPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read SSH private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, nil, "", fmt.Errorf("parse SSH private key: %w", err)
	}
	hostPin := ""
	hostKeyCallback := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		hostPin = fingerprint
		return verifyOrTrustHostKey(t.cfg.HostKeyPinPath, fingerprint)
	}

	sshConfig := &ssh.ClientConfig{
		User:            t.cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 20 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp4", t.cfg.Address)
	if err != nil {
		return nil, nil, hostPin, fmt.Errorf("dial cloud SSH: %w", err)
	}
	_ = rawConn.SetDeadline(time.Now().Add(15 * time.Second))
	clientConn, channels, requests, err := ssh.NewClientConn(rawConn, t.cfg.Address, sshConfig)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, hostPin, fmt.Errorf("SSH handshake: %w", err)
	}
	_ = rawConn.SetDeadline(time.Time{})
	client := ssh.NewClient(clientConn, channels, requests)
	listener, err := client.Listen("tcp", t.cfg.RemoteListenAddress)
	if err != nil {
		_ = client.Close()
		return nil, nil, hostPin, fmt.Errorf("request remote listener %s: %w", t.cfg.RemoteListenAddress, err)
	}
	return listener, client, hostPin, nil
}

func EnsurePrivateKey(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("SSH private key path is empty")
	}
	if data, err := os.ReadFile(path); err == nil {
		signer, parseErr := ssh.ParsePrivateKey(data)
		if parseErr != nil {
			return "", parseErr
		}
		return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, block, 0o600); err != nil {
		return "", err
	}
	sshPublic, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublic))), nil
}

func verifyOrTrustHostKey(path, fingerprint string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("SSH host key pin path is empty")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		expected := strings.TrimSpace(string(data))
		if expected != fingerprint {
			return fmt.Errorf("SSH host key changed: expected %s, received %s", expected, fingerprint)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fingerprint+"\n"), 0o600)
}

func (t *Tunnel) updateStatus(status Status) {
	t.mu.Lock()
	t.status = status
	callback := t.onStatus
	t.mu.Unlock()
	if callback != nil {
		callback(status)
	}
}

func waitContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
