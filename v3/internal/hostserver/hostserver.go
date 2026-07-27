package hostserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type TLSConfig struct {
	Mode        string
	Domain      string
	Email       string
	CacheDir    string
	HTTPAddress string
	CertFile    string
	KeyFile     string
}

type Config struct {
	ListenAddress string
	TLS           TLSConfig
}

type Status struct {
	DirectListening bool
	DirectError     string
	ChallengeError  string
}

type Server struct {
	cfg     Config
	handler http.Handler
	log     *log.Logger

	mu              sync.Mutex
	tlsConfig       *tls.Config
	challengeServer *http.Server
	servers         []*http.Server
	status          Status
	onStatus        func(Status)
}

func New(cfg Config, handler http.Handler, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{cfg: cfg, handler: handler, log: logger}
}

func (s *Server) SetOnStatus(callback func(Status)) {
	s.mu.Lock()
	s.onStatus = callback
	status := s.status
	s.mu.Unlock()
	if callback != nil {
		callback(status)
	}
}

func (s *Server) Start(ctx context.Context) error {
	tlsConfig, challengeServer, challengeListener, err := s.prepareTLS()
	if err != nil {
		s.updateStatus(func(status *Status) { status.DirectError = err.Error() })
		return err
	}
	s.mu.Lock()
	s.tlsConfig = tlsConfig
	s.challengeServer = challengeServer
	s.mu.Unlock()

	if challengeServer != nil && challengeListener != nil {
		go func() {
			s.log.Printf("[证书] ACME 验证监听 address=%s domain=%s", challengeListener.Addr(), s.cfg.TLS.Domain)
			err := challengeServer.Serve(challengeListener)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.log.Printf("[证书] ACME 验证服务停止 error=%q", err)
				s.updateStatus(func(status *Status) { status.ChallengeError = err.Error() })
			}
		}()
	}

	listener, err := net.Listen("tcp6", s.cfg.ListenAddress)
	if err != nil {
		s.updateStatus(func(status *Status) { status.DirectError = err.Error() })
		return fmt.Errorf("listen %s: %w", s.cfg.ListenAddress, err)
	}
	s.updateStatus(func(status *Status) {
		status.DirectListening = true
		status.DirectError = ""
	})
	s.log.Printf("[服务端] IPv6 公网监听 address=%s tls=%s", listener.Addr(), s.tlsMode())
	go func() {
		if err := s.ServeListener(ctx, listener, "ipv6_direct"); err != nil && ctx.Err() == nil {
			s.log.Printf("[服务端] IPv6 监听停止 error=%q", err)
			s.updateStatus(func(status *Status) {
				status.DirectListening = false
				status.DirectError = err.Error()
			})
		}
	}()
	return nil
}

func (s *Server) ServeListener(ctx context.Context, listener net.Listener, source string) error {
	if listener == nil {
		return errors.New("listener is nil")
	}
	s.mu.Lock()
	tlsConfig := s.tlsConfig
	s.mu.Unlock()
	if s.tlsMode() != "disabled" {
		if tlsConfig == nil {
			return errors.New("TLS configuration is not ready")
		}
		listener = tls.NewListener(listener, tlsConfig.Clone())
	}

	httpServer := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		ErrorLog:          s.log,
	}
	s.mu.Lock()
	s.servers = append(s.servers, httpServer)
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	err := httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s listener: %w", source, err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	servers := append([]*http.Server(nil), s.servers...)
	challenge := s.challengeServer
	s.mu.Unlock()
	var firstErr error
	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if challenge != nil {
		if err := challenge.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Server) prepareTLS() (*tls.Config, *http.Server, net.Listener, error) {
	switch s.tlsMode() {
	case "disabled":
		s.log.Printf("[证书] TLS 已禁用，仅允许用于本地开发")
		return nil, nil, nil, nil
	case "files", "local_ca":
		certificate, err := tls.LoadX509KeyPair(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("load TLS certificate: %w", err)
		}
		if s.tlsMode() == "local_ca" {
			return &tls.Config{
				MinVersion: tls.VersionTLS12,
				GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
					renewed, err := tls.LoadX509KeyPair(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
					if err != nil {
						return nil, err
					}
					return &renewed, nil
				},
			}, nil, nil, nil
		}
		return &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{certificate},
		}, nil, nil, nil
	case "acme":
		if strings.TrimSpace(s.cfg.TLS.Domain) == "" {
			return nil, nil, nil, errors.New("ACME domain is empty")
		}
		if strings.TrimSpace(s.cfg.TLS.CacheDir) == "" {
			return nil, nil, nil, errors.New("ACME cache directory is empty")
		}
		if err := os.MkdirAll(s.cfg.TLS.CacheDir, 0o700); err != nil {
			return nil, nil, nil, err
		}
		manager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache(s.cfg.TLS.CacheDir),
			HostPolicy: autocert.HostWhitelist(s.cfg.TLS.Domain),
			Email:      s.cfg.TLS.Email,
		}
		tlsConfig := manager.TLSConfig()
		tlsConfig.MinVersion = tls.VersionTLS12

		address := s.cfg.TLS.HTTPAddress
		if strings.TrimSpace(address) == "" {
			address = "[::]:80"
		}
		listener, err := net.Listen("tcp6", address)
		if err != nil {
			// A cached certificate can still serve traffic. Return the TLS
			// configuration and expose the challenge failure through status.
			s.log.Printf("[证书] 无法监听 ACME HTTP 验证端口 address=%s error=%q", address, err)
			s.updateStatus(func(status *Status) { status.ChallengeError = err.Error() })
			return tlsConfig, nil, nil, nil
		}
		challengeServer := &http.Server{
			Handler:           manager.HTTPHandler(nil),
			ReadHeaderTimeout: 10 * time.Second,
			ErrorLog:          s.log,
		}
		return tlsConfig, challengeServer, listener, nil
	default:
		return nil, nil, nil, fmt.Errorf("unknown TLS mode %q", s.cfg.TLS.Mode)
	}
}

func (s *Server) tlsMode() string {
	mode := strings.ToLower(strings.TrimSpace(s.cfg.TLS.Mode))
	if mode == "" {
		return "acme"
	}
	return mode
}

func (s *Server) updateStatus(update func(*Status)) {
	s.mu.Lock()
	update(&s.status)
	status := s.status
	callback := s.onStatus
	s.mu.Unlock()
	if callback != nil {
		callback(status)
	}
}
