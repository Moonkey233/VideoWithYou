package client

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"videowithyou/v2/local-client/internal/adapter"
	"videowithyou/v2/local-client/internal/bridge"
	"videowithyou/v2/local-client/internal/config"
	"videowithyou/v2/local-client/internal/model"
	"videowithyou/v2/local-client/internal/ntp"
	"videowithyou/v2/local-client/internal/syncer"
	"videowithyou/v2/local-client/internal/ws"
	videowithyoupb "videowithyou/v2/proto/gen"
)

type timeSyncSample struct {
	t1 int64
	t2 int64
	t3 int64
	t4 int64
}

const maxRoomEvents = 20

type Client struct {
	log     *log.Logger
	cfg     config.Config
	cfgPath string

	wsClient *ws.Client
	extHost  bridge.Host
	adapter  adapter.Endpoint
	syncer   *syncer.Core

	offsetMs atomic.Int64
	tickMs   atomic.Int64

	mu                sync.Mutex
	role              Role
	desiredRole       Role
	desiredRoom       string
	roomID            string
	roomCode          string
	hostID            string
	clientID          string
	membersCount      int
	lastError         string
	lastHostState     *videowithyoupb.HostState
	lastHostURL       string
	lastNavigateURL   string
	lastNavigateAt    time.Time
	lastExtURL        string
	lastExtSite       string
	lastExtSeen       time.Time
	extIdleTriggered  bool
	hostDisplayName   string
	lastSyncAt        time.Time
	lastSyncUIAt      time.Time
	lastIdleReportAt  time.Time
	roomEvents        []string
	members           map[string]string
	serverConnected   bool
	pendingRoomAction bool

	timeSyncCh chan timeSyncSample
}

func New(cfg config.Config, cfgPath string, host bridge.Host, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.Default()
	}
	browserAdapter := adapter.NewBrowserAdapter(host, logger, cfg.FollowURL)

	syncCfg := syncer.Config{
		HardSeekThresholdMS: cfg.HardSeekThresholdMS,
		DeadzoneMS:          cfg.DeadzoneMS,
		SoftRateEnabled:     cfg.SoftRateEnabled,
		SoftRateThresholdMS: cfg.SoftRateThresholdMS,
		SoftRateAdjust:      cfg.SoftRateAdjust,
		SoftRateMaxMS:       cfg.SoftRateMaxMS,
	}

	client := &Client{
		log:        logger,
		cfg:        cfg,
		cfgPath:    cfgPath,
		wsClient:   ws.NewClient(cfg.ServerURL, logger),
		extHost:    host,
		adapter:    browserAdapter,
		syncer:     syncer.NewCore(syncCfg, browserAdapter, logger),
		timeSyncCh: make(chan timeSyncSample, 8),
	}
	client.tickMs.Store(cfg.TickMS)

	client.wsClient.SetOnConnect(func(conn *websocket.Conn) error {
		hello := client.makeClientHello()
		payload, err := proto.Marshal(hello)
		if err != nil {
			return err
		}
		return conn.WriteMessage(websocket.BinaryMessage, payload)
	})

	client.wsClient.SetOnStatus(func(connected bool) {
		client.mu.Lock()
		client.serverConnected = connected
		client.mu.Unlock()
		client.sendUIState()
	})
	client.wsClient.SetOnActivity(client.markServerActivity)

	return client
}

func (c *Client) makeClientHello() *videowithyoupb.Envelope {
	displayName := strings.TrimSpace(c.cfg.DisplayName)
	if displayName == "" {
		displayName = "local-client"
	}
	return &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_ClientHello{
			ClientHello: &videowithyoupb.ClientHello{
				ClientName:    displayName,
				ClientVersion: "v2",
			},
		},
	}
}

func (c *Client) sendClientHello() {
	c.wsClient.Send(c.makeClientHello())
}

func (c *Client) Start(ctx context.Context) {
	c.extHost.Start(ctx)
	go c.wsClient.Start(ctx)
	go c.handleWSIncoming(ctx)
	go c.handleBridgeIncoming(ctx)
	go c.syncLoop(ctx)
	go c.timeSyncLoop(ctx)
	go c.extIdleLoop(ctx)

	c.mu.Lock()
	c.lastExtSeen = time.Now()
	c.mu.Unlock()
	c.sendUIState()
}

func (c *Client) handleWSIncoming(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case env := <-c.wsClient.Incoming():
			c.handleEnvelope(env)
		}
	}
}

func (c *Client) handleEnvelope(env *videowithyoupb.Envelope) {
	switch payload := env.Payload.(type) {
	case *videowithyoupb.Envelope_ServerHello:
		c.handleServerHello(payload.ServerHello)
	case *videowithyoupb.Envelope_CreateRoomResp:
		c.handleCreateRoomResp(payload.CreateRoomResp)
	case *videowithyoupb.Envelope_JoinRoomResp:
		c.handleJoinRoomResp(payload.JoinRoomResp)
	case *videowithyoupb.Envelope_RoomSnapshot:
		c.handleRoomSnapshot(payload.RoomSnapshot)
	case *videowithyoupb.Envelope_BroadcastState:
		c.handleBroadcastState(payload.BroadcastState)
	case *videowithyoupb.Envelope_TimeSyncResp:
		c.handleTimeSyncResp(payload.TimeSyncResp)
	case *videowithyoupb.Envelope_ErrorResp:
		c.handleError(payload.ErrorResp)
	}
}

func (c *Client) handleServerHello(msg *videowithyoupb.ServerHello) {
	if msg == nil {
		return
	}
	c.mu.Lock()
	c.clientID = msg.ClientId
	c.mu.Unlock()

	c.log.Printf("server hello client_id=%s", msg.ClientId)
	go c.runInitialTimeSync()

	c.mu.Lock()
	desiredRole := c.desiredRole
	desiredRoom := c.desiredRoom
	c.mu.Unlock()

	if desiredRole == RoleHost {
		c.sendCreateRoom()
	} else if desiredRole == RoleFollower && desiredRoom != "" {
		c.sendJoinRoom(desiredRoom)
	}
}

func (c *Client) handleCreateRoomResp(resp *videowithyoupb.CreateRoomResp) {
	if resp == nil {
		return
	}
	c.mu.Lock()
	c.roomID = resp.RoomId
	c.roomCode = resp.RoomCode
	c.role = RoleHost
	c.hostID = c.clientID
	c.membersCount = 1
	c.lastHostState = nil
	c.lastHostURL = ""
	c.lastNavigateURL = ""
	c.lastNavigateAt = time.Time{}
	c.lastExtURL = ""
	c.lastExtSite = ""
	c.hostDisplayName = strings.TrimSpace(c.cfg.DisplayName)
	c.lastSyncAt = time.Time{}
	c.lastSyncUIAt = time.Time{}
	c.lastIdleReportAt = time.Time{}
	c.roomEvents = nil
	c.members = nil
	c.lastError = ""
	c.pendingRoomAction = false
	c.mu.Unlock()

	c.log.Printf("room created code=%s", resp.RoomCode)
	joinEvent := formatMemberEvent(c.cfg.DisplayName, true)
	c.appendRoomEvent(joinEvent)
	c.sendRoomEvents([]string{joinEvent})
	c.sendUIState()
}

func (c *Client) handleJoinRoomResp(resp *videowithyoupb.JoinRoomResp) {
	if resp == nil {
		return
	}
	c.mu.Lock()
	c.roomID = resp.RoomId
	c.hostID = resp.HostId
	c.role = RoleFollower
	c.lastHostState = nil
	c.lastHostURL = ""
	c.lastNavigateURL = ""
	c.lastNavigateAt = time.Time{}
	c.lastExtURL = ""
	c.lastExtSite = ""
	c.lastExtSeen = time.Time{}
	c.extIdleTriggered = false
	c.hostDisplayName = ""
	c.lastSyncAt = time.Time{}
	c.lastSyncUIAt = time.Time{}
	c.lastIdleReportAt = time.Time{}
	c.roomEvents = nil
	c.members = nil
	c.lastError = ""
	c.pendingRoomAction = false
	c.mu.Unlock()

	c.log.Printf("room joined id=%s host=%s", resp.RoomId, resp.HostId)
	joinEvent := formatMemberEvent(c.cfg.DisplayName, true)
	c.appendRoomEvent(joinEvent)
	c.sendRoomEvents([]string{joinEvent})
	c.sendUIState()
}

func (c *Client) handleRoomSnapshot(snapshot *videowithyoupb.RoomSnapshot) {
	if snapshot == nil {
		return
	}
	c.mu.Lock()
	c.roomID = snapshot.RoomId
	c.roomCode = snapshot.RoomCode
	c.hostID = snapshot.HostId
	c.membersCount = len(snapshot.Members)
	c.hostDisplayName = findHostDisplayName(snapshot.HostId, snapshot.Members)
	events := c.updateMembers(snapshot.Members)
	if snapshot.LatestState != nil {
		c.lastHostState = snapshot.LatestState
	}
	c.pendingRoomAction = false
	c.mu.Unlock()

	c.recordRoomEvents(events)
	c.sendRoomEvents(events)
	c.sendUIState()
}

func (c *Client) handleBroadcastState(state *videowithyoupb.BroadcastState) {
	if state == nil {
		return
	}
	c.mu.Lock()
	if state.State != nil && state.State.HostId != "" {
		c.hostID = state.State.HostId
	}
	c.lastHostState = state.State
	var events []string
	if len(state.Members) > 0 {
		c.membersCount = len(state.Members)
		c.hostDisplayName = findHostDisplayName(c.hostID, state.Members)
		events = c.updateMembers(state.Members)
	}
	role := c.role
	followURL := c.cfg.FollowURL
	endpoint := c.cfg.Endpoint
	adapter := c.adapter
	lastExtURL := c.lastExtURL
	c.mu.Unlock()

	if role == RoleFollower && followURL && endpoint == "browser" && adapter != nil {
		hostURL := ""
		if state.State != nil && state.State.Media != nil {
			hostURL = state.State.Media.Url
		}
		if hostURL != "" {
			currentURL := ""
			if adapterState, ok := adapter.GetState(); ok {
				if time.Since(adapterState.UpdatedAt) < 5*time.Second {
					currentURL = adapterState.Media.URL
				}
			}
			if currentURL == "" {
				currentURL = lastExtURL
			}
			if c.shouldNavigate(hostURL, currentURL) {
				_ = adapter.Navigate(hostURL)
			}
		}
	}
	c.recordRoomEvents(events)
	c.sendRoomEvents(events)
	c.sendUIState()
}

func (c *Client) handleTimeSyncResp(resp *videowithyoupb.TimeSyncResp) {
	if resp == nil {
		return
	}
	sample := timeSyncSample{
		t1: resp.T1LocalMs,
		t2: resp.T2ServerMs,
		t3: resp.T3ServerMs,
		t4: time.Now().UnixMilli(),
	}
	select {
	case c.timeSyncCh <- sample:
	default:
	}
}

func (c *Client) handleError(errResp *videowithyoupb.ErrorResp) {
	if errResp == nil {
		return
	}
	message := errResp.Message
	c.mu.Lock()
	c.lastError = message
	c.pendingRoomAction = false
	isRoomClosed := strings.Contains(strings.ToLower(message), "room closed")
	if isRoomClosed {
		c.clearRoomLocked()
		c.lastError = message
	}
	c.mu.Unlock()
	c.log.Printf("server error: %s", message)
	c.sendUIState()
}

func (c *Client) handleBridgeIncoming(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case raw := <-c.extHost.Incoming():
			c.handleBridgeMessage(raw)
		}
	}
}

func (c *Client) handleBridgeMessage(raw []byte) {
	c.markExtSeen()
	var envelope struct {
		Type    string          `json:"type"`
		Action  string          `json:"action"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		c.log.Printf("extension message parse failed: %v", err)
		return
	}

	msgType := envelope.Type
	payload := envelope.Payload
	if msgType == "" {
		var alt map[string]json.RawMessage
		if err := json.Unmarshal(raw, &alt); err == nil {
			if value, ok := alt["ext_hello"]; ok {
				msgType = "ext_hello"
				payload = value
			} else if value, ok := alt["player_state"]; ok {
				msgType = "player_state"
				payload = value
			} else if value, ok := alt["ui_action"]; ok {
				msgType = "ui_action"
				payload = value
			}
		}
	}

	switch msgType {
	case "ext_hello":
		var hello struct {
			URL  string `json:"url"`
			Site string `json:"site"`
		}
		if err := json.Unmarshal(payload, &hello); err == nil {
			c.mu.Lock()
			c.lastExtURL = strings.TrimSpace(hello.URL)
			c.lastExtSite = strings.TrimSpace(hello.Site)
			c.mu.Unlock()
		}
		c.sendUIState()
	case "player_state":
		var state model.PlayerState
		if err := json.Unmarshal(payload, &state); err != nil {
			c.log.Printf("player_state parse failed: %v", err)
			return
		}
		if c.adapter != nil {
			c.adapter.UpdatePlayerState(state)
		}
	case "ui_action":
		c.handleUIAction(payload, raw)
	}
}

func (c *Client) handleUIAction(payload []byte, raw []byte) {
	var action UIAction
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &action); err != nil {
			c.log.Printf("ui_action parse failed: %v", err)
			return
		}
	} else {
		if err := json.Unmarshal(raw, &action); err != nil {
			c.log.Printf("ui_action parse failed: %v", err)
			return
		}
	}

	switch action.Action {
	case "create_room":
		var cfg config.Config
		saveConfig := false
		name := strings.TrimSpace(action.DisplayName)
		c.mu.Lock()
		if c.role != RoleNone || c.roomID != "" {
			c.lastError = "already in a room"
			c.mu.Unlock()
			c.sendUIState()
			return
		}
		if c.pendingRoomAction {
			c.lastError = "room action pending"
			c.mu.Unlock()
			c.sendUIState()
			return
		}
		if name == "" {
			c.lastError = "nickname required"
			c.mu.Unlock()
			c.sendUIState()
			return
		}
		if name != "" {
			c.cfg.DisplayName = name
			cfg = c.cfg
			saveConfig = true
		}
		c.lastError = ""
		c.pendingRoomAction = true
		c.desiredRole = RoleHost
		c.desiredRoom = ""
		c.mu.Unlock()
		if saveConfig {
			_ = config.SaveConfig(c.cfgPath, cfg)
		}
		c.sendClientHello()
		c.sendCreateRoom()
	case "join_room":
		var cfg config.Config
		saveConfig := false
		name := strings.TrimSpace(action.DisplayName)
		c.mu.Lock()
		if c.role != RoleNone || c.roomID != "" {
			c.lastError = "already in a room"
			c.mu.Unlock()
			c.sendUIState()
			return
		}
		if c.pendingRoomAction {
			c.lastError = "room action pending"
			c.mu.Unlock()
			c.sendUIState()
			return
		}
		if name == "" {
			c.lastError = "nickname required"
			c.mu.Unlock()
			c.sendUIState()
			return
		}
		if name != "" {
			c.cfg.DisplayName = name
			cfg = c.cfg
			saveConfig = true
		}
		c.lastError = ""
		c.pendingRoomAction = true
		c.desiredRole = RoleFollower
		c.desiredRoom = action.RoomCode
		c.mu.Unlock()
		if saveConfig {
			_ = config.SaveConfig(c.cfgPath, cfg)
		}
		c.sendClientHello()
		c.sendJoinRoom(action.RoomCode)
	case "leave_room":
		c.sendLeaveRoom()
	case "set_endpoint":
		if action.Endpoint != "" {
			c.updateEndpoint(action.Endpoint)
		}
	case "set_follow_url":
		if action.FollowURL != nil {
			c.updateFollowURL(*action.FollowURL)
		}
	case "set_config":
		if action.Config != nil {
			c.applyConfig(*action.Config)
		}
	case "refresh_state":
		c.sendUIState()
	}
}

func (c *Client) markExtSeen() {
	c.mu.Lock()
	c.lastExtSeen = time.Now()
	c.extIdleTriggered = false
	c.mu.Unlock()
}

func (c *Client) markServerActivity() {
	now := time.Now()
	c.mu.Lock()
	c.lastSyncAt = now
	shouldSend := now.Sub(c.lastSyncUIAt) >= time.Second
	if shouldSend {
		c.lastSyncUIAt = now
	}
	c.mu.Unlock()
	if shouldSend {
		c.sendUIState()
	}
}

func (c *Client) updateEndpoint(endpoint string) {
	c.mu.Lock()
	c.cfg.Endpoint = endpoint
	cfg := c.cfg
	c.mu.Unlock()

	switch endpoint {
	case "potplayer":
		c.adapter = adapter.NewPotPlayerAdapter(cfg.PotPlayer, c.log)
	default:
		c.adapter = adapter.NewBrowserAdapter(c.extHost, c.log, cfg.FollowURL)
	}
	c.syncer.UpdateAdapter(c.adapter)

	_ = config.SaveConfig(c.cfgPath, cfg)
	c.sendUIState()
}

func (c *Client) updateFollowURL(enabled bool) {
	c.mu.Lock()
	c.cfg.FollowURL = enabled
	c.mu.Unlock()

	if c.adapter != nil {
		c.adapter.SetFollowURL(enabled)
	}
	_ = config.SaveConfig(c.cfgPath, c.cfg)
	c.sendUIState()
}

func (c *Client) applyConfig(cfg config.Config) {
	c.mu.Lock()
	previousEndpoint := c.cfg.Endpoint
	c.cfg = cfg
	c.mu.Unlock()

	c.tickMs.Store(cfg.TickMS)
	c.syncer.UpdateConfig(syncer.Config{
		HardSeekThresholdMS: cfg.HardSeekThresholdMS,
		DeadzoneMS:          cfg.DeadzoneMS,
		SoftRateEnabled:     cfg.SoftRateEnabled,
		SoftRateThresholdMS: cfg.SoftRateThresholdMS,
		SoftRateAdjust:      cfg.SoftRateAdjust,
		SoftRateMaxMS:       cfg.SoftRateMaxMS,
	})

	if previousEndpoint != cfg.Endpoint {
		c.updateEndpoint(cfg.Endpoint)
		return
	}

	if c.adapter != nil {
		c.adapter.SetFollowURL(cfg.FollowURL)
	}
	_ = config.SaveConfig(c.cfgPath, cfg)
	c.sendUIState()
}

func (c *Client) sendCreateRoom() {
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_CreateRoomReq{
			CreateRoomReq: &videowithyoupb.CreateRoomReq{ClientId: c.clientID},
		},
	}
	c.wsClient.Send(env)
}

func (c *Client) sendJoinRoom(code string) {
	if code == "" {
		return
	}
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_JoinRoomReq{
			JoinRoomReq: &videowithyoupb.JoinRoomReq{
				ClientId: c.clientID,
				RoomCode: code,
			},
		},
	}
	c.wsClient.Send(env)
}

func (c *Client) sendLeaveRoom() {
	c.mu.Lock()
	roomID := c.roomID
	c.clearRoomLocked()
	c.mu.Unlock()

	if roomID == "" {
		return
	}
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_LeaveRoomReq{
			LeaveRoomReq: &videowithyoupb.LeaveRoomReq{
				ClientId: c.clientID,
				RoomId:   roomID,
			},
		},
	}
	c.wsClient.Send(env)
	c.sendUIState()
}

func (c *Client) clearRoomLocked() {
	c.roomID = ""
	c.roomCode = ""
	c.role = RoleNone
	c.desiredRole = RoleNone
	c.desiredRoom = ""
	c.membersCount = 0
	c.lastHostState = nil
	c.lastHostURL = ""
	c.lastNavigateURL = ""
	c.lastNavigateAt = time.Time{}
	c.lastExtURL = ""
	c.lastExtSite = ""
	c.lastExtSeen = time.Time{}
	c.extIdleTriggered = false
	c.hostDisplayName = ""
	c.lastSyncAt = time.Time{}
	c.lastSyncUIAt = time.Time{}
	c.lastIdleReportAt = time.Time{}
	c.roomEvents = nil
	c.members = nil
	c.pendingRoomAction = false
	c.lastError = ""
}

func (c *Client) syncLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(c.tickMs.Load()) * time.Millisecond):
			c.handleTick()
		}
	}
}

func (c *Client) handleTick() {
	c.mu.Lock()
	role := c.role
	roomID := c.roomID
	hostState := c.lastHostState
	offsetMs := c.offsetMs.Load()
	localOffset := c.cfg.OffsetMS
	c.mu.Unlock()

	if role == RoleHost {
		c.sendHostState(roomID, offsetMs)
		return
	}
	if role == RoleFollower {
		c.syncer.Align(hostState, offsetMs, localOffset)
	}
}

func (c *Client) sendHostState(roomID string, offsetMs int64) {
	if roomID == "" || c.adapter == nil {
		return
	}
	state, ok := c.adapter.GetState()

	c.mu.Lock()
	seq := uint64(time.Now().UnixNano())
	hostID := c.clientID
	localOffset := c.cfg.OffsetMS
	pageURL := c.lastExtURL
	pageSite := c.lastExtSite
	lastExtSeen := c.lastExtSeen
	idleReportSec := c.cfg.HostIdleReportSec
	lastIdleReportAt := c.lastIdleReportAt
	c.mu.Unlock()

	now := time.Now()
	sampleServerTime := now.UnixMilli() + offsetMs
	if !ok || time.Since(state.UpdatedAt) > 5*time.Second {
		if pageURL == "" || (!lastExtSeen.IsZero() && time.Since(lastExtSeen) > 15*time.Second) {
			return
		}
		if idleReportSec <= 0 {
			return
		}
		if !lastIdleReportAt.IsZero() && now.Sub(lastIdleReportAt) < time.Duration(idleReportSec)*time.Second {
			return
		}
		c.mu.Lock()
		if !c.lastIdleReportAt.IsZero() && now.Sub(c.lastIdleReportAt) < time.Duration(idleReportSec)*time.Second {
			c.mu.Unlock()
			return
		}
		c.lastIdleReportAt = now
		c.mu.Unlock()
		hostState := &videowithyoupb.HostState{
			RoomId:             roomID,
			HostId:             hostID,
			Seq:                seq,
			PositionMs:         0,
			Rate:               1,
			Paused:             true,
			SampleServerTimeMs: sampleServerTime,
			OffsetMs:           localOffset,
			Media: &videowithyoupb.MediaInfo{
				Url:   pageURL,
				Title: "",
				Site:  pageSite,
				Attrs: map[string]string{"page_only": "1"},
			},
		}
		env := &videowithyoupb.Envelope{
			Payload: &videowithyoupb.Envelope_HostState{HostState: hostState},
		}
		c.wsClient.Send(env)
		return
	}
	hostState := &videowithyoupb.HostState{
		RoomId:             roomID,
		HostId:             hostID,
		Seq:                seq,
		PositionMs:         state.PositionMs,
		Rate:               state.Rate,
		Paused:             state.Paused,
		SampleServerTimeMs: sampleServerTime,
		OffsetMs:           localOffset,
	}
	if state.Media.URL != "" || state.Media.Title != "" {
		hostState.Media = &videowithyoupb.MediaInfo{
			Url:   state.Media.URL,
			Title: state.Media.Title,
			Site:  state.Media.Site,
			Attrs: state.Media.Attrs,
		}
	}

	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_HostState{HostState: hostState},
	}
	c.wsClient.Send(env)
}

func (c *Client) shouldNavigate(hostURL, currentURL string) bool {
	normalizedHost := normalizeURL(hostURL)
	if normalizedHost == "" {
		return false
	}
	normalizedCurrent := normalizeURL(currentURL)
	if normalizedCurrent != "" && normalizedCurrent == normalizedHost {
		c.mu.Lock()
		c.lastHostURL = normalizedHost
		c.mu.Unlock()
		return false
	}

	now := time.Now()
	c.mu.Lock()
	c.lastHostURL = normalizedHost
	if normalizedHost == c.lastNavigateURL && now.Sub(c.lastNavigateAt) < 3*time.Second {
		c.mu.Unlock()
		return false
	}
	c.lastNavigateURL = normalizedHost
	c.lastNavigateAt = now
	c.mu.Unlock()
	return true
}

func normalizeURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "t" || lower == "start" || lower == "timestamp" ||
			lower == "spm_id_from" || lower == "spm_id" || lower == "from" || lower == "vd_source" ||
			lower == "share_source" || lower == "share_medium" || lower == "share_plat" ||
			lower == "share_session_id" || lower == "unique_k" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	return parsed.String()
}

func findHostDisplayName(hostID string, members []*videowithyoupb.Member) string {
	for _, member := range members {
		if member == nil {
			continue
		}
		if member.IsHost || (hostID != "" && member.MemberId == hostID) {
			return strings.TrimSpace(member.DisplayName)
		}
	}
	return ""
}

func (c *Client) updateMembers(members []*videowithyoupb.Member) []string {
	next := make(map[string]string, len(members))
	for _, member := range members {
		if member == nil || member.MemberId == "" {
			continue
		}
		next[member.MemberId] = strings.TrimSpace(member.DisplayName)
	}

	if c.members == nil {
		c.members = next
		return nil
	}

	events := make([]string, 0)
	for id, name := range next {
		if _, ok := c.members[id]; !ok {
			events = append(events, formatMemberEvent(name, true))
		}
	}
	for id, name := range c.members {
		if _, ok := next[id]; !ok {
			events = append(events, formatMemberEvent(name, false))
		}
	}
	c.members = next
	return events
}

func formatMemberEvent(name string, joined bool) string {
	label := strings.TrimSpace(name)
	if label == "" {
		label = "\u6210\u5458"
	}
	if joined {
		return label + "\u0020\u8fdb\u5165\u4e86\u623f\u95f4"
	}
	return label + "\u0020\u79bb\u5f00\u4e86\u623f\u95f4"
}

func formatSyncTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func (c *Client) timeSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.cfg.TimeSyncIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runSingleTimeSync()
		}
	}
}

func (c *Client) extIdleLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkExtIdle()
		}
	}
}

func (c *Client) checkExtIdle() {
	c.mu.Lock()
	timeoutSec := c.cfg.ExtIdleTimeoutSec
	keepRoomOnIdle := c.cfg.KeepRoomOnIdle
	endpoint := c.cfg.Endpoint
	lastSeen := c.lastExtSeen
	inRoom := c.role != RoleNone && c.roomID != ""
	alreadyTriggered := c.extIdleTriggered
	c.mu.Unlock()

	if timeoutSec <= 0 || !inRoom || alreadyTriggered || keepRoomOnIdle || endpoint == "potplayer" {
		return
	}
	if lastSeen.IsZero() {
		return
	}

	if time.Since(lastSeen) < time.Duration(timeoutSec)*time.Second {
		return
	}

	c.log.Printf("extension idle >%ds, leaving room", timeoutSec)
	c.sendLeaveRoom()

	c.mu.Lock()
	c.extIdleTriggered = true
	c.lastError = "extension idle, left room"
	c.mu.Unlock()
	c.sendUIState()
}

func (c *Client) runInitialTimeSync() {
	bestDelay := int64(math.MaxInt64)
	bestOffset := int64(0)

	for i := 0; i < 5; i++ {
		sample, ok := c.requestTimeSync()
		if !ok {
			continue
		}
		offset, delay := ntp.ComputeOffsetDelay(sample.t1, sample.t2, sample.t3, sample.t4)
		c.log.Printf("ntp sample offset=%dms delay=%dms", offset, delay)
		if delay < bestDelay {
			bestDelay = delay
			bestOffset = offset
		}
		time.Sleep(150 * time.Millisecond)
	}

	if bestDelay != int64(math.MaxInt64) {
		c.offsetMs.Store(bestOffset)
		c.log.Printf("ntp offset selected=%dms", bestOffset)
	}
}

func (c *Client) runSingleTimeSync() {
	sample, ok := c.requestTimeSync()
	if !ok {
		return
	}
	offset, delay := ntp.ComputeOffsetDelay(sample.t1, sample.t2, sample.t3, sample.t4)
	c.offsetMs.Store(offset)
	c.log.Printf("ntp refresh offset=%dms delay=%dms", offset, delay)
}

func (c *Client) requestTimeSync() (timeSyncSample, bool) {
	t1 := time.Now().UnixMilli()
	env := &videowithyoupb.Envelope{
		Payload: &videowithyoupb.Envelope_TimeSyncReq{
			TimeSyncReq: &videowithyoupb.TimeSyncReq{T1LocalMs: t1},
		},
	}
	c.wsClient.Send(env)

	timeout := time.After(2 * time.Second)
	for {
		select {
		case sample := <-c.timeSyncCh:
			if sample.t1 == t1 {
				return sample, true
			}
		case <-timeout:
			return timeSyncSample{}, false
		}
	}
}

func (c *Client) sendUIState() {
	c.mu.Lock()
	events := append([]string(nil), c.roomEvents...)
	state := UIState{
		RoomCode:        c.roomCode,
		Role:            string(c.role),
		MembersCount:    c.membersCount,
		Endpoint:        c.cfg.Endpoint,
		FollowURL:       c.cfg.FollowURL,
		LastError:       c.lastError,
		DisplayName:     c.cfg.DisplayName,
		HostDisplayName: c.hostDisplayName,
		LastSyncTime:    formatSyncTime(c.lastSyncAt),
		RoomEvents:      events,
		ServerConnected: c.serverConnected,
	}
	c.mu.Unlock()

	payload := map[string]any{
		"type":    "ui_state",
		"payload": state,
	}
	_ = c.extHost.Send(payload)
}

func (c *Client) sendRoomEvents(events []string) {
	for _, message := range events {
		payload := map[string]any{
			"type": "room_event",
			"payload": map[string]any{
				"message": message,
			},
		}
		_ = c.extHost.Send(payload)
	}
}

func (c *Client) recordRoomEvents(events []string) {
	if len(events) == 0 {
		return
	}
	c.mu.Lock()
	for _, message := range events {
		c.appendRoomEventLocked(message)
	}
	c.mu.Unlock()
}

func (c *Client) appendRoomEvent(message string) {
	c.mu.Lock()
	c.appendRoomEventLocked(message)
	c.mu.Unlock()
}

func (c *Client) appendRoomEventLocked(message string) {
	label := strings.TrimSpace(message)
	if label == "" {
		return
	}
	c.roomEvents = append(c.roomEvents, label)
	if len(c.roomEvents) > maxRoomEvents {
		c.roomEvents = c.roomEvents[len(c.roomEvents)-maxRoomEvents:]
	}
}
