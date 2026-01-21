package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	videowithyoupb "videowithyou/v2/proto/gen"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 15 * time.Second
	hostIdleTimeoutDefault  = 600 * time.Second
	hostIdleCheckInterval   = 5 * time.Second
)

var roomAlphabet = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type Config struct {
	Addr string
	Path string
}

type Server struct {
	log       *log.Logger
	mu        sync.RWMutex
	rooms     map[string]*Room
	roomCodes map[string]string
	upgrader  websocket.Upgrader
	hostIdleTimeout time.Duration
}

type Room struct {
	id          string
	code        string
	hostID      string
	members     map[string]*Client
	latestState *videowithyoupb.HostState
	lastHostStateAt time.Time
}

type Client struct {
	id     string
	name   string
	conn   *websocket.Conn
	send   chan []byte
	roomID string
	isHost bool
	active bool
}

func NewServer(logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	srv := &Server{
		log:       logger,
		rooms:     make(map[string]*Room),
		roomCodes: make(map[string]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		hostIdleTimeout: hostIdleTimeoutDefault,
	}
	go srv.hostIdleLoop()
	return srv
}

func (s *Server) SetHostIdleTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	s.hostIdleTimeout = timeout
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Printf("ws upgrade failed: %v", err)
		return
	}

	client := &Client{
		id:   randomID(),
		conn: conn,
		send: make(chan []byte, 64),
		active: true,
	}

	conn.SetReadLimit(2 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	if err := s.handleHello(client); err != nil {
		s.log.Printf("hello failed: %v", err)
		_ = conn.Close()
		return
	}

	done := make(chan struct{})
	go s.writeLoop(client, done)
	s.readLoop(client)

	close(done)
	s.cleanupClient(client)
	_ = conn.Close()
}

func (s *Server) handleHello(client *Client) error {
	msgType, data, err := client.conn.ReadMessage()
	if err != nil {
		return err
	}
	if msgType != websocket.BinaryMessage {
		return errors.New("expected binary hello")
	}

	env := &videowithyoupb.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return err
	}
	hello := env.GetClientHello()
	if hello == nil {
		return errors.New("missing client_hello")
	}

	client.name = hello.GetClientName()
	resp := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_ServerHello{
			ServerHello: &videowithyoupb.ServerHello{
				ClientId:     client.id,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	}
	return s.sendEnvelope(client, resp)
}

func (s *Server) handleClientHelloUpdate(client *Client, hello *videowithyoupb.ClientHello) {
	if hello == nil {
		return
	}
	name := strings.TrimSpace(hello.GetClientName())
	if name == "" {
		return
	}

	s.mu.Lock()
	client.name = name
	room := s.rooms[client.roomID]
	s.mu.Unlock()

	if room != nil {
		s.broadcastRoomSnapshot(room)
	}
}

func (s *Server) readLoop(client *Client) {
	for {
		msgType, data, err := client.conn.ReadMessage()
		if err != nil {
			s.log.Printf("client %s read error: %v", client.id, err)
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		env := &videowithyoupb.Envelope{}
		if err := proto.Unmarshal(data, env); err != nil {
			s.log.Printf("client %s bad envelope: %v", client.id, err)
			continue
		}

		switch payload := env.Payload.(type) {
		case *videowithyoupb.Envelope_ClientHello:
			s.handleClientHelloUpdate(client, payload.ClientHello)
		case *videowithyoupb.Envelope_CreateRoomReq:
			s.handleCreateRoom(client, payload.CreateRoomReq)
		case *videowithyoupb.Envelope_JoinRoomReq:
			s.handleJoinRoom(client, payload.JoinRoomReq)
		case *videowithyoupb.Envelope_LeaveRoomReq:
			s.handleLeaveRoom(client, payload.LeaveRoomReq)
		case *videowithyoupb.Envelope_MemberStatus:
			s.handleMemberStatus(client, payload.MemberStatus)
		case *videowithyoupb.Envelope_HostState:
			s.handleHostState(client, payload.HostState)
		case *videowithyoupb.Envelope_TimeSyncReq:
			s.handleTimeSync(client, payload.TimeSyncReq)
		default:
			s.log.Printf("client %s unknown payload", client.id)
		}
	}
}

func (s *Server) writeLoop(client *Client, done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func (s *Server) handleCreateRoom(client *Client, _ *videowithyoupb.CreateRoomReq) {
	roomID := randomID()
	roomCode := s.uniqueRoomCode(6)

	room := &Room{
		id:      roomID,
		code:    roomCode,
		hostID:  client.id,
		members: map[string]*Client{client.id: client},
		lastHostStateAt: time.Now(),
	}
	client.roomID = roomID
	client.isHost = true
	client.active = true

	s.mu.Lock()
	s.rooms[roomID] = room
	s.roomCodes[roomCode] = roomID
	s.mu.Unlock()

	s.log.Printf("room created %s (%s) host=%s", roomID, roomCode, client.id)

	resp := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_CreateRoomResp{
			CreateRoomResp: &videowithyoupb.CreateRoomResp{
				RoomId:       roomID,
				RoomCode:     roomCode,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	}
	_ = s.sendEnvelope(client, resp)
	s.broadcastRoomSnapshot(room)
}

func (s *Server) handleJoinRoom(client *Client, req *videowithyoupb.JoinRoomReq) {
	if req == nil {
		return
	}

	s.mu.Lock()
	roomID, ok := s.roomCodes[req.RoomCode]
	if !ok {
		s.mu.Unlock()
		s.sendError(client, "room not found")
		return
	}
	room := s.rooms[roomID]
	if room == nil {
		s.mu.Unlock()
		s.sendError(client, "room not found")
		return
	}
	room.members[client.id] = client
	client.roomID = roomID
	client.isHost = false
	client.active = true
	s.mu.Unlock()

	s.log.Printf("room join %s (%s) member=%s", roomID, room.code, client.id)

	resp := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_JoinRoomResp{
			JoinRoomResp: &videowithyoupb.JoinRoomResp{
				RoomId:       roomID,
				HostId:       room.hostID,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	}
	_ = s.sendEnvelope(client, resp)
	s.broadcastRoomSnapshot(room)

	if room.latestState != nil {
		s.broadcastHostState(room, room.latestState)
	}
}

func (s *Server) handleLeaveRoom(client *Client, _ *videowithyoupb.LeaveRoomReq) {
	s.removeClientFromRoom(client)
}

func (s *Server) handleHostState(client *Client, state *videowithyoupb.HostState) {
	if state == nil {
		return
	}

	s.mu.Lock()
	room := s.rooms[state.RoomId]
	if room == nil {
		s.mu.Unlock()
		return
	}
	if room.hostID != client.id {
		s.mu.Unlock()
		return
	}
	room.latestState = state
	room.lastHostStateAt = time.Now()
	s.mu.Unlock()

	s.broadcastHostState(room, state)
}

func (s *Server) handleMemberStatus(client *Client, status *videowithyoupb.MemberStatus) {
	if status == nil {
		return
	}
	if status.MemberId != "" && status.MemberId != client.id {
		return
	}
	s.mu.Lock()
	room := s.rooms[client.roomID]
	if room == nil || room.id != status.RoomId {
		s.mu.Unlock()
		return
	}
	client.active = status.Active
	s.mu.Unlock()
	s.log.Printf("member status room=%s member=%s active=%t", room.id, client.id, status.Active)
}

func (s *Server) broadcastHostState(room *Room, state *videowithyoupb.HostState) {
	targets := make([]*Client, 0, len(room.members))

	s.mu.RLock()
	for _, member := range room.members {
		if member.id == room.hostID {
			continue
		}
		if !member.active {
			continue
		}
		targets = append(targets, member)
	}
	s.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	serverTime := time.Now().UnixMilli()
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_BroadcastState{
			BroadcastState: &videowithyoupb.BroadcastState{
				State:        state,
				ServerTimeMs: serverTime,
			},
		},
	}

	payload, err := proto.Marshal(env)
	if err != nil {
		s.log.Printf("broadcast marshal failed: %v", err)
		return
	}

	count := 0
	for _, member := range targets {
		select {
		case member.send <- payload:
			count++
		default:
		}
	}

	s.log.Printf("broadcast state room=%s followers=%d", room.id, count)
}

func (s *Server) handleTimeSync(client *Client, req *videowithyoupb.TimeSyncReq) {
	if req == nil {
		return
	}
	t2 := time.Now().UnixMilli()
	t3 := time.Now().UnixMilli()
	resp := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_TimeSyncResp{
			TimeSyncResp: &videowithyoupb.TimeSyncResp{
				T1LocalMs:    req.T1LocalMs,
				T2ServerMs:   t2,
				T3ServerMs:   t3,
				ServerTimeMs: t3,
			},
		},
	}
	_ = s.sendEnvelope(client, resp)
}

func (s *Server) hostIdleLoop() {
	ticker := time.NewTicker(hostIdleCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		type roomClose struct {
			id      string
			members []*Client
		}
		toClose := make([]roomClose, 0)

		s.mu.Lock()
		for _, room := range s.rooms {
			if room == nil {
				continue
			}
			last := room.lastHostStateAt
			if last.IsZero() {
				continue
			}
			if now.Sub(last) < s.hostIdleTimeout {
				continue
			}

			members := make([]*Client, 0, len(room.members))
			for _, member := range room.members {
				member.roomID = ""
				member.isHost = false
				members = append(members, member)
			}
			delete(s.rooms, room.id)
			delete(s.roomCodes, room.code)
			toClose = append(toClose, roomClose{id: room.id, members: members})
		}
		s.mu.Unlock()

		for _, item := range toClose {
			for _, member := range item.members {
				s.sendError(member, "room closed (host idle)")
			}
			s.log.Printf("room closed idle %s", item.id)
		}
	}
}

func (s *Server) broadcastRoomSnapshot(room *Room) {
	targets := make([]*Client, 0, len(room.members))
	snapshot := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_RoomSnapshot{
			RoomSnapshot: &videowithyoupb.RoomSnapshot{
				RoomId:       room.id,
				RoomCode:     room.code,
				HostId:       room.hostID,
				Members:      nil,
				LatestState:  room.latestState,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	}
	s.mu.RLock()
	snapshot.GetRoomSnapshot().Members = s.buildMembers(room)
	for _, member := range room.members {
		targets = append(targets, member)
	}
	s.mu.RUnlock()

	payload, err := proto.Marshal(snapshot)
	if err != nil {
		s.log.Printf("snapshot marshal failed: %v", err)
		return
	}

	count := 0
	for _, member := range targets {
		select {
		case member.send <- payload:
			count++
		default:
		}
	}

	s.log.Printf("room snapshot room=%s members=%d", room.id, count)
}

func (s *Server) buildMembers(room *Room) []*videowithyoupb.Member {
	members := make([]*videowithyoupb.Member, 0, len(room.members))
	for _, member := range room.members {
		members = append(members, &videowithyoupb.Member{
			MemberId:    member.id,
			DisplayName: member.name,
			IsHost:      member.id == room.hostID,
		})
	}
	return members
}

func (s *Server) sendEnvelope(client *Client, env *videowithyoupb.Envelope) error {
	payload, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	select {
	case client.send <- payload:
	default:
	}
	return nil
}

func (s *Server) sendError(client *Client, message string) {
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_ErrorResp{
			ErrorResp: &videowithyoupb.ErrorResp{
				Message:      message,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	}
	_ = s.sendEnvelope(client, env)
}

func (s *Server) removeClientFromRoom(client *Client) {
	s.mu.Lock()
	room := s.rooms[client.roomID]
	if room == nil {
		s.mu.Unlock()
		return
	}
	isHost := room.hostID == client.id
	delete(room.members, client.id)
	client.roomID = ""
	client.isHost = false

	if len(room.members) == 0 {
		delete(s.rooms, room.id)
		delete(s.roomCodes, room.code)
		s.mu.Unlock()
		s.log.Printf("room removed %s", room.id)
		return
	}

	if isHost {
		remaining := make([]*Client, 0, len(room.members))
		for _, member := range room.members {
			member.roomID = ""
			member.isHost = false
			remaining = append(remaining, member)
		}
		delete(s.rooms, room.id)
		delete(s.roomCodes, room.code)
		s.mu.Unlock()

		for _, member := range remaining {
			s.sendError(member, "room closed (host left)")
		}
		s.log.Printf("room closed %s host=%s", room.id, client.id)
		return
	}
	s.mu.Unlock()

	s.log.Printf("room leave %s member=%s", room.id, client.id)
	s.broadcastRoomSnapshot(room)
}

func (s *Server) cleanupClient(client *Client) {
	s.removeClientFromRoom(client)
	close(client.send)
}

func (s *Server) uniqueRoomCode(length int) string {
	for {
		code := randomRoomCode(length)
		s.mu.RLock()
		_, exists := s.roomCodes[code]
		s.mu.RUnlock()
		if !exists {
			return code
		}
	}
}

func randomRoomCode(length int) string {
	buf := make([]byte, length)
	for i := range buf {
		buf[i] = roomAlphabet[randomInt(len(roomAlphabet))]
	}
	return string(buf)
}

func randomID() string {
	data := make([]byte, 16)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}

func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	var b [1]byte
	_, _ = rand.Read(b[:])
	return int(b[0]) % max
}
