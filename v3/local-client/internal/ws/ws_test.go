package ws

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	videowithyoupb "videowithyou/v3/proto/gen"
)

func TestFallsBackFromIPv6ToCloudIPv4(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	accepted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		accepted <- struct{}{}
		_, _, _ = conn.ReadMessage()
		data, _ := proto.Marshal(&videowithyoupb.Envelope{
			Payload: &videowithyoupb.Envelope_ServerHello{
				ServerHello: &videowithyoupb.ServerHello{ClientId: "test"},
			},
		})
		_ = conn.WriteMessage(websocket.BinaryMessage, data)
		<-r.Context().Done()
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(Config{
		URL:              "ws://does-not-exist.invalid:" + port,
		CloudDialAddress: parsed.Host,
		DirectTimeout:    150 * time.Millisecond,
		CloudTimeout:     time.Second,
		RetryDelay:       50 * time.Millisecond,
	}, nil)
	client.SetOnConnect(func(conn *websocket.Conn, _ Route) error {
		return conn.WriteMessage(websocket.BinaryMessage, []byte("hello"))
	})
	statuses := make(chan Status, 16)
	client.SetOnStatus(func(status Status) {
		select {
		case statuses <- status:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Start(ctx)

	deadline := time.After(4 * time.Second)
	for {
		select {
		case status := <-statuses:
			if status.Connected {
				if status.Route != RouteCloudIPv4 {
					t.Fatalf("connected through unexpected route %q", status.Route)
				}
				if status.DirectError == "" {
					t.Fatal("direct failure reason was not retained")
				}
				select {
				case <-accepted:
				case <-time.After(time.Second):
					t.Fatal("cloud endpoint did not accept the connection")
				}
				return
			}
		case <-deadline:
			t.Fatal("client did not reach cloud fallback")
		}
	}
}

func TestClassifiesIPv6DNSFailure(t *testing.T) {
	err := &net.DNSError{Err: "no such host", Name: "example.invalid"}
	if stage := classifyDialFailure(err, nil); stage != "dns" {
		t.Fatalf("unexpected stage %q", stage)
	}
}
