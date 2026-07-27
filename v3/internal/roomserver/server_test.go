package roomserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	videowithyoupb "videowithyou/v3/proto/gen"
)

func TestRoomSurvivesSessionResume(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := New(Config{
		AccessToken:     "test-access",
		ReconnectGrace:  3 * time.Second,
		HostIdleTimeout: time.Minute,
	}, nil)
	server.Start(ctx)
	httpServer := httptest.NewServer(serverHandler(server))
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	host, hello := connectTestClient(t, wsURL, "host-session", "test-access")
	if hello.GetResumed() {
		t.Fatal("new host session unexpectedly resumed")
	}
	sendTestEnvelope(t, host, &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_CreateRoomReq{
			CreateRoomReq: &videowithyoupb.CreateRoomReq{},
		},
	})
	create := waitForEnvelope(t, host, func(env *videowithyoupb.Envelope) bool {
		return env.GetCreateRoomResp() != nil
	}).GetCreateRoomResp()
	if create.GetRoomCode() == "" {
		t.Fatal("server returned an empty room code")
	}

	follower, _ := connectTestClient(t, wsURL, "follower-session", "test-access")
	defer follower.Close()
	sendTestEnvelope(t, follower, &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_JoinRoomReq{
			JoinRoomReq: &videowithyoupb.JoinRoomReq{RoomCode: create.GetRoomCode()},
		},
	})
	waitForEnvelope(t, follower, func(env *videowithyoupb.Envelope) bool {
		return env.GetJoinRoomResp() != nil
	})

	if err := host.Close(); err != nil {
		t.Fatalf("close host: %v", err)
	}
	resumedHost, resumedHello := connectTestClient(t, wsURL, "host-session", "test-access")
	defer resumedHost.Close()
	if !resumedHello.GetResumed() {
		t.Fatal("host session was not resumed")
	}
	snapshot := waitForEnvelope(t, resumedHost, func(env *videowithyoupb.Envelope) bool {
		return env.GetRoomSnapshot() != nil
	}).GetRoomSnapshot()
	if snapshot.GetRoomCode() != create.GetRoomCode() {
		t.Fatalf("room code changed after resume: got %q want %q", snapshot.GetRoomCode(), create.GetRoomCode())
	}
	if len(snapshot.GetMembers()) != 2 {
		t.Fatalf("unexpected member count after resume: %d", len(snapshot.GetMembers()))
	}
}

func TestAccessTokenRejected(t *testing.T) {
	server := New(Config{AccessToken: "correct"}, nil)
	httpServer := httptest.NewServer(serverHandler(server))
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	sendTestEnvelope(t, conn, clientHello("session", "wrong"))
	env := readTestEnvelope(t, conn)
	if env.GetErrorResp().GetMessage() != "access denied" {
		t.Fatalf("unexpected access response: %v", env)
	}
}

func serverHandler(server *Server) *testHandler {
	return &testHandler{server: server}
}

type testHandler struct {
	server *Server
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.server.HandleWS(w, r)
}

func connectTestClient(t *testing.T, wsURL, sessionToken, accessToken string) (*websocket.Conn, *videowithyoupb.ServerHello) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	sendTestEnvelope(t, conn, clientHello(sessionToken, accessToken))
	env := readTestEnvelope(t, conn)
	hello := env.GetServerHello()
	if hello == nil {
		conn.Close()
		t.Fatalf("expected server hello, got %v", env)
	}
	return conn, hello
}

func clientHello(sessionToken, accessToken string) *videowithyoupb.Envelope {
	return &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_ClientHello{
			ClientHello: &videowithyoupb.ClientHello{
				ClientName:       "test",
				ClientVersion:    "v3-test",
				ClientInstanceId: sessionToken + "-instance",
				SessionToken:     sessionToken,
				AccessToken:      accessToken,
				ConnectionRoute:  "test",
			},
		},
	}
}

func sendTestEnvelope(t *testing.T, conn *websocket.Conn, env *videowithyoupb.Envelope) {
	t.Helper()
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readTestEnvelope(t *testing.T, conn *websocket.Conn) *videowithyoupb.Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	messageType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("unexpected message type %d", messageType)
	}
	env := &videowithyoupb.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return env
}

func waitForEnvelope(t *testing.T, conn *websocket.Conn, match func(*videowithyoupb.Envelope) bool) *videowithyoupb.Envelope {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		env := readTestEnvelope(t, conn)
		if match(env) {
			return env
		}
	}
	t.Fatal("timed out waiting for matching envelope")
	return nil
}
