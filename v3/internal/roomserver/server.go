package roomserver

import (
	"context"
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

	videowithyoupb "videowithyou/v3/proto/gen"
)

const (
	writeWait              = 10 * time.Second
	pongWait               = 30 * time.Second
	pingPeriod             = 15 * time.Second
	maintenanceInterval    = time.Second
	defaultReconnectGrace  = 30 * time.Second
	defaultHostIdleTimeout = 10 * time.Minute
)

var roomAlphabet = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type Config struct {
	AccessToken     string
	ReconnectGrace  time.Duration
	HostIdleTimeout time.Duration
}

type Server struct {
	log *log.Logger
	cfg Config

	mu        sync.RWMutex
	rooms     map[string]*Room
	roomCodes map[string]string
	sessions  map[string]*Session
	upgrader  websocket.Upgrader
}

type Room struct {
	id              string
	code            string
	hostID          string
	members         map[string]*Session
	latestState     *videowithyoupb.HostState
	lastHostStateAt time.Time
}

type Session struct {
	id             string
	token          string
	instanceID     string
	name           string
	roomID         string
	isHost         bool
	active         bool
	connected      bool
	disconnectedAt time.Time
	peer           *peer
}

type peer struct {
	conn    *websocket.Conn
	send    chan []byte
	done    chan struct{}
	stop    sync.Once
	session *Session
	route   string
}

func New(cfg Config, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	if cfg.ReconnectGrace <= 0 {
		cfg.ReconnectGrace = defaultReconnectGrace
	}
	if cfg.HostIdleTimeout <= 0 {
		cfg.HostIdleTimeout = defaultHostIdleTimeout
	}
	return &Server{
		log:       logger,
		cfg:       cfg,
		rooms:     make(map[string]*Room),
		roomCodes: make(map[string]string),
		sessions:  make(map[string]*Session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (s *Server) Start(ctx context.Context) {
	go s.maintenanceLoop(ctx)
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Printf("[服务端] WebSocket 升级失败 error=%q", err)
		return
	}

	p := &peer{
		conn: conn,
		send: make(chan []byte, 128),
		done: make(chan struct{}),
	}
	conn.SetReadLimit(2 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	resumed, err := s.handleHello(p)
	if err != nil {
		s.log.Printf("[服务端] 客户端握手失败 remote=%s error=%q", conn.RemoteAddr(), err)
		_ = conn.Close()
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	go s.writeLoop(p)
	if resumed {
		s.log.Printf("[服务端] 会话已恢复 client_id=%s route=%s", p.session.id, p.route)
		s.broadcastSessionRoom(p.session)
	}
	s.readLoop(p)
	p.stop.Do(func() { close(p.done) })
	s.detachPeer(p)
	_ = conn.Close()
}

func (s *Server) handleHello(p *peer) (bool, error) {
	msgType, data, err := p.conn.ReadMessage()
	if err != nil {
		return false, err
	}
	if msgType != websocket.BinaryMessage {
		return false, errors.New("expected binary client hello")
	}
	env := &videowithyoupb.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return false, err
	}
	hello := env.GetClientHello()
	if hello == nil {
		return false, errors.New("missing client_hello")
	}
	if s.cfg.AccessToken != "" && hello.GetAccessToken() != s.cfg.AccessToken {
		_ = writeDirect(p.conn, errorEnvelope("access denied"))
		return false, errors.New("access denied")
	}

	token := strings.TrimSpace(hello.GetSessionToken())
	if token == "" {
		token = randomID()
	}
	name := strings.TrimSpace(hello.GetClientName())
	if name == "" {
		name = "VideoWithYou"
	}

	var oldPeer *peer
	s.mu.Lock()
	session, exists := s.sessions[token]
	if !exists {
		session = &Session{
			id:         randomID(),
			token:      token,
			instanceID: hello.GetClientInstanceId(),
		}
		s.sessions[token] = session
	}
	resumed := exists && session.roomID != ""
	oldPeer = session.peer
	session.name = name
	session.instanceID = hello.GetClientInstanceId()
	session.connected = true
	session.disconnectedAt = time.Time{}
	session.peer = p
	p.session = session
	p.route = hello.GetConnectionRoute()
	s.mu.Unlock()

	if oldPeer != nil && oldPeer != p {
		oldPeer.stop.Do(func() { close(oldPeer.done) })
		_ = oldPeer.conn.Close()
	}

	resp := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_ServerHello{
			ServerHello: &videowithyoupb.ServerHello{
				ClientId:     session.id,
				ServerTimeMs: time.Now().UnixMilli(),
				SessionToken: session.token,
				Resumed:      resumed,
			},
		},
	}
	if err := writeDirect(p.conn, resp); err != nil {
		return false, err
	}
	s.log.Printf("[服务端] 客户端已连接 client_id=%s route=%s remote=%s resumed=%t", session.id, p.route, p.conn.RemoteAddr(), resumed)
	return resumed, nil
}

func (s *Server) readLoop(p *peer) {
	for {
		msgType, data, err := p.conn.ReadMessage()
		if err != nil {
			s.log.Printf("[服务端] 客户端断开 client_id=%s route=%s error=%q", p.session.id, p.route, err)
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		env := &videowithyoupb.Envelope{}
		if err := proto.Unmarshal(data, env); err != nil {
			s.log.Printf("[服务端] 协议解析失败 client_id=%s error=%q", p.session.id, err)
			continue
		}
		s.handleEnvelope(p.session, env)
	}
}

func (s *Server) writeLoop(p *peer) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case payload := <-p.send:
			_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				p.stop.Do(func() { close(p.done) })
				_ = p.conn.Close()
				return
			}
		case <-ticker.C:
			_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				p.stop.Do(func() { close(p.done) })
				_ = p.conn.Close()
				return
			}
		case <-p.done:
			return
		}
	}
}

func (s *Server) handleEnvelope(session *Session, env *videowithyoupb.Envelope) {
	switch payload := env.Payload.(type) {
	case *videowithyoupb.Envelope_ClientHello:
		s.handleClientHelloUpdate(session, payload.ClientHello)
	case *videowithyoupb.Envelope_CreateRoomReq:
		s.handleCreateRoom(session)
	case *videowithyoupb.Envelope_JoinRoomReq:
		s.handleJoinRoom(session, payload.JoinRoomReq)
	case *videowithyoupb.Envelope_LeaveRoomReq:
		s.removeSessionFromRoom(session, true)
	case *videowithyoupb.Envelope_MemberStatus:
		s.handleMemberStatus(session, payload.MemberStatus)
	case *videowithyoupb.Envelope_HostState:
		s.handleHostState(session, payload.HostState)
	case *videowithyoupb.Envelope_TimeSyncReq:
		s.handleTimeSync(session, payload.TimeSyncReq)
	default:
		s.log.Printf("[服务端] 未知协议消息 client_id=%s", session.id)
	}
}

func (s *Server) handleClientHelloUpdate(session *Session, hello *videowithyoupb.ClientHello) {
	if hello == nil {
		return
	}
	name := strings.TrimSpace(hello.GetClientName())
	if name == "" {
		return
	}
	s.mu.Lock()
	session.name = name
	room := s.rooms[session.roomID]
	s.mu.Unlock()
	if room != nil {
		s.broadcastRoomSnapshot(room)
	}
}

func (s *Server) handleCreateRoom(session *Session) {
	s.mu.Lock()
	if session.roomID != "" {
		s.mu.Unlock()
		s.sendError(session, "already in a room")
		return
	}
	roomID := randomID()
	roomCode := s.uniqueRoomCodeLocked(6)
	room := &Room{
		id:              roomID,
		code:            roomCode,
		hostID:          session.id,
		members:         map[string]*Session{session.id: session},
		lastHostStateAt: time.Now(),
	}
	session.roomID = roomID
	session.isHost = true
	session.active = true
	s.rooms[roomID] = room
	s.roomCodes[roomCode] = roomID
	s.mu.Unlock()

	s.log.Printf("[服务端] 房间已创建 room=%s code=%s host=%s", roomID, roomCode, session.id)
	s.sendEnvelope(session, &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_CreateRoomResp{
			CreateRoomResp: &videowithyoupb.CreateRoomResp{
				RoomId:       roomID,
				RoomCode:     roomCode,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	})
	s.broadcastRoomSnapshot(room)
}

func (s *Server) handleJoinRoom(session *Session, req *videowithyoupb.JoinRoomReq) {
	if req == nil {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.GetRoomCode()))
	s.mu.Lock()
	if session.roomID != "" {
		s.mu.Unlock()
		s.sendError(session, "already in a room")
		return
	}
	roomID, ok := s.roomCodes[code]
	room := s.rooms[roomID]
	if !ok || room == nil {
		s.mu.Unlock()
		s.sendError(session, "room not found")
		return
	}
	room.members[session.id] = session
	session.roomID = roomID
	session.isHost = false
	session.active = true
	hostID := room.hostID
	s.mu.Unlock()

	s.log.Printf("[服务端] 成员加入 room=%s code=%s member=%s", roomID, code, session.id)
	s.sendEnvelope(session, &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_JoinRoomResp{
			JoinRoomResp: &videowithyoupb.JoinRoomResp{
				RoomId:       roomID,
				HostId:       hostID,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	})
	s.broadcastRoomSnapshot(room)
	if room.latestState != nil {
		s.broadcastHostState(room, room.latestState)
	}
}

func (s *Server) handleHostState(session *Session, state *videowithyoupb.HostState) {
	if state == nil {
		return
	}
	s.mu.Lock()
	room := s.rooms[session.roomID]
	if room == nil || room.id != state.GetRoomId() || room.hostID != session.id {
		s.mu.Unlock()
		return
	}
	state.HostId = session.id
	room.latestState = state
	room.lastHostStateAt = time.Now()
	s.mu.Unlock()
	s.broadcastHostState(room, state)
}

func (s *Server) handleMemberStatus(session *Session, status *videowithyoupb.MemberStatus) {
	if status == nil {
		return
	}
	s.mu.Lock()
	room := s.rooms[session.roomID]
	if room == nil || room.id != status.GetRoomId() {
		s.mu.Unlock()
		return
	}
	session.active = status.GetActive()
	s.mu.Unlock()
}

func (s *Server) handleTimeSync(session *Session, req *videowithyoupb.TimeSyncReq) {
	if req == nil {
		return
	}
	t2 := time.Now().UnixMilli()
	t3 := time.Now().UnixMilli()
	s.sendEnvelope(session, &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_TimeSyncResp{
			TimeSyncResp: &videowithyoupb.TimeSyncResp{
				T1LocalMs:    req.GetT1LocalMs(),
				T2ServerMs:   t2,
				T3ServerMs:   t3,
				ServerTimeMs: t3,
			},
		},
	})
}

func (s *Server) broadcastHostState(room *Room, state *videowithyoupb.HostState) {
	s.mu.RLock()
	targets := make([]*Session, 0, len(room.members))
	members := s.buildMembersLocked(room)
	for _, member := range room.members {
		if member.id != room.hostID && member.active && member.connected {
			targets = append(targets, member)
		}
	}
	s.mu.RUnlock()
	if len(targets) == 0 {
		return
	}
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_BroadcastState{
			BroadcastState: &videowithyoupb.BroadcastState{
				State:        state,
				ServerTimeMs: time.Now().UnixMilli(),
				Members:      members,
			},
		},
	}
	for _, target := range targets {
		s.sendEnvelope(target, env)
	}
}

func (s *Server) broadcastRoomSnapshot(room *Room) {
	s.mu.RLock()
	if s.rooms[room.id] != room {
		s.mu.RUnlock()
		return
	}
	targets := make([]*Session, 0, len(room.members))
	for _, member := range room.members {
		if member.connected {
			targets = append(targets, member)
		}
	}
	snapshot := &videowithyoupb.RoomSnapshot{
		RoomId:       room.id,
		RoomCode:     room.code,
		HostId:       room.hostID,
		Members:      s.buildMembersLocked(room),
		LatestState:  room.latestState,
		ServerTimeMs: time.Now().UnixMilli(),
	}
	s.mu.RUnlock()
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_RoomSnapshot{RoomSnapshot: snapshot},
	}
	for _, target := range targets {
		s.sendEnvelope(target, env)
	}
}

func (s *Server) broadcastSessionRoom(session *Session) {
	s.mu.RLock()
	room := s.rooms[session.roomID]
	s.mu.RUnlock()
	if room != nil {
		s.broadcastRoomSnapshot(room)
	}
}

func (s *Server) buildMembersLocked(room *Room) []*videowithyoupb.Member {
	members := make([]*videowithyoupb.Member, 0, len(room.members))
	for _, member := range room.members {
		members = append(members, &videowithyoupb.Member{
			MemberId:    member.id,
			DisplayName: member.name,
			IsHost:      member.id == room.hostID,
			Connected:   member.connected,
		})
	}
	return members
}

func (s *Server) sendEnvelope(session *Session, env *videowithyoupb.Envelope) {
	payload, err := proto.Marshal(env)
	if err != nil {
		s.log.Printf("[服务端] 消息编码失败 error=%q", err)
		return
	}
	s.mu.RLock()
	p := session.peer
	connected := session.connected
	s.mu.RUnlock()
	if !connected || p == nil {
		return
	}
	select {
	case p.send <- payload:
	default:
		s.log.Printf("[服务端] 发送队列已满 client_id=%s", session.id)
	}
}

func (s *Server) sendError(session *Session, message string) {
	s.sendEnvelope(session, errorEnvelope(message))
}

func (s *Server) detachPeer(p *peer) {
	s.mu.Lock()
	session := p.session
	if session == nil || session.peer != p {
		s.mu.Unlock()
		return
	}
	session.peer = nil
	session.connected = false
	session.disconnectedAt = time.Now()
	room := s.rooms[session.roomID]
	s.mu.Unlock()
	if room != nil {
		s.log.Printf("[服务端] 会话进入恢复宽限 client_id=%s grace=%s", session.id, s.cfg.ReconnectGrace)
		s.broadcastRoomSnapshot(room)
	}
}

func (s *Server) removeSessionFromRoom(session *Session, explicit bool) {
	s.mu.Lock()
	room := s.rooms[session.roomID]
	if room == nil {
		session.roomID = ""
		session.isHost = false
		s.mu.Unlock()
		return
	}
	isHost := room.hostID == session.id
	delete(room.members, session.id)
	session.roomID = ""
	session.isHost = false
	session.active = false

	if isHost || len(room.members) == 0 {
		remaining := make([]*Session, 0, len(room.members))
		for _, member := range room.members {
			member.roomID = ""
			member.isHost = false
			member.active = false
			remaining = append(remaining, member)
		}
		delete(s.rooms, room.id)
		delete(s.roomCodes, room.code)
		s.mu.Unlock()
		if isHost {
			for _, member := range remaining {
				s.sendError(member, "room closed (host left)")
			}
			s.log.Printf("[服务端] 房间已解散 room=%s host=%s explicit=%t", room.id, session.id, explicit)
		}
		return
	}
	s.mu.Unlock()
	s.log.Printf("[服务端] 成员离开 room=%s member=%s explicit=%t", room.id, session.id, explicit)
	s.broadcastRoomSnapshot(room)
}

func (s *Server) maintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(maintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.expireDisconnected(now)
			s.expireIdleRooms(now)
		}
	}
}

func (s *Server) expireDisconnected(now time.Time) {
	s.mu.RLock()
	expired := make([]*Session, 0)
	for _, session := range s.sessions {
		if session.connected || session.roomID == "" || session.disconnectedAt.IsZero() {
			continue
		}
		if now.Sub(session.disconnectedAt) >= s.cfg.ReconnectGrace {
			expired = append(expired, session)
		}
	}
	s.mu.RUnlock()
	for _, session := range expired {
		s.log.Printf("[服务端] 会话恢复超时 client_id=%s", session.id)
		s.removeSessionFromRoom(session, false)
	}
}

func (s *Server) expireIdleRooms(now time.Time) {
	s.mu.RLock()
	hosts := make([]*Session, 0)
	for _, room := range s.rooms {
		if room.lastHostStateAt.IsZero() || now.Sub(room.lastHostStateAt) < s.cfg.HostIdleTimeout {
			continue
		}
		if host := room.members[room.hostID]; host != nil {
			hosts = append(hosts, host)
		}
	}
	s.mu.RUnlock()
	for _, host := range hosts {
		s.sendError(host, "room closed (host idle)")
		s.removeSessionFromRoom(host, false)
	}
}

func (s *Server) uniqueRoomCodeLocked(length int) string {
	for {
		code := randomRoomCode(length)
		if _, exists := s.roomCodes[code]; !exists {
			return code
		}
	}
}

func writeDirect(conn *websocket.Conn, env *videowithyoupb.Envelope) error {
	payload, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteMessage(websocket.BinaryMessage, payload)
}

func errorEnvelope(message string) *videowithyoupb.Envelope {
	return &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_ErrorResp{
			ErrorResp: &videowithyoupb.ErrorResp{
				Message:      message,
				ServerTimeMs: time.Now().UnixMilli(),
			},
		},
	}
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(buf)
}

func randomRoomCode(length int) string {
	buf := make([]byte, length)
	for i := range buf {
		buf[i] = roomAlphabet[randomInt(len(roomAlphabet))]
	}
	return string(buf)
}

func randomInt(max int) int {
	if max <= 1 {
		return 0
	}
	var buf [1]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			return int(time.Now().UnixNano() % int64(max))
		}
		limit := 256 - (256 % max)
		if int(buf[0]) < limit {
			return int(buf[0]) % max
		}
	}
}
