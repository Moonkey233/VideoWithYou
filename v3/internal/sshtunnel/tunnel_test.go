package sshtunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	"videowithyou/v3/internal/localcert"
)

func TestEnsurePrivateKeyIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh", "id_ed25519")
	first, err := EnsurePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsurePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("SSH public key changed after reload")
	}
	if !strings.HasPrefix(first, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key format: %s", first)
	}
}

func TestHostKeyTrustOnFirstUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host.pin")
	if err := verifyOrTrustHostKey(path, "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := verifyOrTrustHostKey(path, "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := verifyOrTrustHostKey(path, "SHA256:changed"); err == nil {
		t.Fatal("changed host key was accepted")
	}
}

func TestSSHRemoteForwardEndToEnd(t *testing.T) {
	testDir := t.TempDir()
	privateKeyPath := filepath.Join(testDir, "id_ed25519")
	if _, err := EnsurePrivateKey(privateKeyPath); err != nil {
		t.Fatal(err)
	}
	sshAddress, stopSSH := startForwardingSSHServer(t)
	defer stopSSH()

	portProbe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	remoteAddress := portProbe.Addr().String()
	_ = portProbe.Close()

	tunnel := New(Config{
		Address:             sshAddress,
		User:                "test",
		PrivateKeyPath:      privateKeyPath,
		HostKeyPinPath:      filepath.Join(testDir, "host.pin"),
		RemoteListenAddress: remoteAddress,
		ReconnectDelay:      50 * time.Millisecond,
	}, nil)
	statuses := make(chan Status, 8)
	tunnel.SetOnStatus(func(status Status) {
		select {
		case statuses <- status:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tunnel.Start(ctx, func(_ context.Context, listener net.Listener) error {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		defer conn.Close()
		buffer := make([]byte, 4)
		if _, err := io.ReadFull(conn, buffer); err != nil {
			return err
		}
		if string(buffer) != "ping" {
			return io.ErrUnexpectedEOF
		}
		_, err = conn.Write([]byte("pong"))
		return err
	})

	waitForTunnelStatus(t, statuses, true)
	external, err := net.DialTimeout("tcp4", remoteAddress, 2*time.Second)
	if err != nil {
		t.Fatalf("dial forwarded port: %v", err)
	}
	defer external.Close()
	if _, err := external.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(external, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "pong" {
		t.Fatalf("unexpected forwarded response %q", response)
	}
}

func TestSSHRemoteForwardProxiesWSSWithLocalCA(t *testing.T) {
	generated, err := localcert.Ensure(localcert.Config{
		Domain:    "owner.invalid",
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(generated.CertFile, generated.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	localServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			t.Errorf("upgrade: %v", upgradeErr)
			return
		}
		defer conn.Close()
		messageType, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			return
		}
		_ = conn.WriteMessage(messageType, payload)
	}))
	localServer.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{pair},
	}
	localServer.StartTLS()
	defer localServer.Close()
	localTarget := strings.TrimPrefix(localServer.URL, "https://")

	testDir := t.TempDir()
	privateKeyPath := filepath.Join(testDir, "id_ed25519")
	if _, err := EnsurePrivateKey(privateKeyPath); err != nil {
		t.Fatal(err)
	}
	sshAddress, stopSSH := startForwardingSSHServer(t)
	defer stopSSH()
	portProbe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	remoteAddress := portProbe.Addr().String()
	_ = portProbe.Close()

	tunnel := New(Config{
		Address:             sshAddress,
		User:                "test",
		PrivateKeyPath:      privateKeyPath,
		HostKeyPinPath:      filepath.Join(testDir, "host.pin"),
		RemoteListenAddress: remoteAddress,
		ReconnectDelay:      50 * time.Millisecond,
	}, nil)
	statuses := make(chan Status, 8)
	tunnel.SetOnStatus(func(status Status) {
		select {
		case statuses <- status:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tunnel.Start(ctx, func(proxyCtx context.Context, listener net.Listener) error {
		return ProxyListener(proxyCtx, listener, "tcp4", localTarget, nil)
	})
	waitForTunnelStatus(t, statuses, true)

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(generated.CAPEM)) {
		t.Fatal("could not load generated CA")
	}
	netDialer := &net.Dialer{Timeout: 2 * time.Second}
	dialer := websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: "owner.invalid",
			RootCAs:    roots,
		},
		NetDialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			return netDialer.DialContext(dialCtx, "tcp4", remoteAddress)
		},
	}
	conn, _, err := dialer.Dial("wss://owner.invalid/ws", nil)
	if err != nil {
		t.Fatalf("dial WSS through SSH proxy: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("cloud")); err != nil {
		t.Fatal(err)
	}
	_, response, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != "cloud" {
		t.Fatalf("unexpected WebSocket response %q", response)
	}
}

type forwardRequest struct {
	Address string
	Port    uint32
}

type forwardedPayload struct {
	Address       string
	Port          uint32
	OriginAddress string
	OriginPort    uint32
}

func startForwardingSSHServer(t *testing.T) (string, func()) {
	t.Helper()
	_, hostPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPrivate)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	serverConfig.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		connection, channels, requests, err := ssh.NewServerConn(raw, serverConfig)
		if err != nil {
			return
		}
		defer connection.Close()
		go func() {
			for channel := range channels {
				_ = channel.Reject(ssh.UnknownChannelType, "no client channels supported")
			}
		}()
		var remoteListener net.Listener
		defer func() {
			if remoteListener != nil {
				_ = remoteListener.Close()
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case request, ok := <-requests:
				if !ok {
					return
				}
				if request.Type != "tcpip-forward" {
					_ = request.Reply(false, nil)
					continue
				}
				var payload forwardRequest
				if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
					_ = request.Reply(false, nil)
					continue
				}
				address := net.JoinHostPort(payload.Address, strconv.Itoa(int(payload.Port)))
				remoteListener, err = net.Listen("tcp4", address)
				if err != nil {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				go forwardAcceptedConnections(connection, remoteListener)
			}
		}
	}()
	return listener.Addr().String(), func() {
		cancel()
		_ = listener.Close()
		wait.Wait()
	}
}

func forwardAcceptedConnections(connection *ssh.ServerConn, listener net.Listener) {
	for {
		external, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer external.Close()
			remoteHost, remotePortText, _ := net.SplitHostPort(listener.Addr().String())
			originHost, originPortText, _ := net.SplitHostPort(external.RemoteAddr().String())
			remotePort, _ := strconv.Atoi(remotePortText)
			originPort, _ := strconv.Atoi(originPortText)
			channel, requests, err := connection.OpenChannel("forwarded-tcpip", ssh.Marshal(&forwardedPayload{
				Address:       remoteHost,
				Port:          uint32(remotePort),
				OriginAddress: originHost,
				OriginPort:    uint32(originPort),
			}))
			if err != nil {
				return
			}
			defer channel.Close()
			go ssh.DiscardRequests(requests)
			copyDone := make(chan struct{}, 2)
			go func() {
				_, _ = io.Copy(channel, external)
				copyDone <- struct{}{}
			}()
			go func() {
				_, _ = io.Copy(external, channel)
				copyDone <- struct{}{}
			}()
			<-copyDone
		}()
	}
}

func waitForTunnelStatus(t *testing.T, statuses <-chan Status, connected bool) Status {
	t.Helper()
	timeout := time.After(4 * time.Second)
	for {
		select {
		case status := <-statuses:
			if status.Connected == connected {
				return status
			}
		case <-timeout:
			t.Fatalf("timed out waiting for tunnel connected=%t", connected)
		}
	}
}
