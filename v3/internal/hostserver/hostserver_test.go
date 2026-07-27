package hostserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"videowithyou/v3/internal/localcert"
)

func TestServeAdditionalListenerWithoutTLS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	server := New(Config{TLS: TLSConfig{Mode: "disabled"}}, handler, nil)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.ServeListener(ctx, listener, "test")
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "ok" {
		t.Fatalf("unexpected response %q", data)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop after cancellation")
	}
}

func TestServeAdditionalListenerWithLocalCA(t *testing.T) {
	generated, err := localcert.Ensure(localcert.Config{
		Domain:    "owner.invalid",
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "secure")
	})
	server := New(Config{TLS: TLSConfig{
		Mode:     "local_ca",
		CertFile: generated.CertFile,
		KeyFile:  generated.KeyFile,
	}}, handler, nil)
	tlsConfig, _, _, err := server.prepareTLS()
	if err != nil {
		t.Fatal(err)
	}
	server.tlsConfig = tlsConfig

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.ServeListener(ctx, listener, "local-ca-test")
	}()

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(generated.CAPEM)) {
		t.Fatal("could not load generated CA")
	}
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: "owner.invalid",
			RootCAs:    roots,
		}},
	}
	response, err := client.Get("https://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "secure" {
		t.Fatalf("unexpected response %q", data)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop after cancellation")
	}
}
