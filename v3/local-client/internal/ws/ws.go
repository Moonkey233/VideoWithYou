package ws

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	videowithyoupb "videowithyou/v3/proto/gen"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 15 * time.Second
)

type Route string

const (
	RouteNone       Route = ""
	RouteLocal      Route = "local"
	RouteIPv6Direct Route = "ipv6_direct"
	RouteCloudIPv4  Route = "cloud_ipv4"
)

type Config struct {
	URL              string
	CloudDialAddress string
	PreferLocal      bool
	DirectTimeout    time.Duration
	CloudTimeout     time.Duration
	RetryDelay       time.Duration
}

type Status struct {
	Connected     bool   `json:"connected"`
	Route         Route  `json:"route"`
	Stage         string `json:"stage"`
	DirectError   string `json:"direct_error"`
	CloudError    string `json:"cloud_error"`
	RemoteAddress string `json:"remote_address"`
	LatencyMS     int64  `json:"latency_ms"`
}

type Client struct {
	cfg Config
	log *log.Logger

	incoming chan *videowithyoupb.Envelope
	send     chan []byte

	onConnect  func(*websocket.Conn, Route) error
	onStatus   func(Status)
	onActivity func()

	statusMu sync.RWMutex
	status   Status
}

type routeAttempt struct {
	route       Route
	network     string
	dialAddress string
	timeout     time.Duration
}

func NewClient(cfg Config, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.Default()
	}
	if cfg.DirectTimeout <= 0 {
		cfg.DirectTimeout = 3 * time.Second
	}
	if cfg.CloudTimeout <= 0 {
		cfg.CloudTimeout = 5 * time.Second
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	return &Client{
		cfg:      cfg,
		log:      logger,
		incoming: make(chan *videowithyoupb.Envelope, 128),
		send:     make(chan []byte, 128),
		status: Status{
			Stage: "idle",
		},
	}
}

func (c *Client) SetOnConnect(fn func(*websocket.Conn, Route) error) {
	c.onConnect = fn
}

func (c *Client) SetOnStatus(fn func(Status)) {
	c.onStatus = fn
}

func (c *Client) SetOnActivity(fn func()) {
	c.onActivity = fn
}

func (c *Client) Incoming() <-chan *videowithyoupb.Envelope {
	return c.incoming
}

func (c *Client) CurrentStatus() Status {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

func (c *Client) Send(env *videowithyoupb.Envelope) {
	payload, err := proto.Marshal(env)
	if err != nil {
		c.log.Printf("[网络] 协议编码失败 error=%q", err)
		return
	}
	select {
	case c.send <- payload:
	default:
		c.log.Printf("[网络] 发送队列已满")
	}
}

func (c *Client) Start(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			c.updateStatus(func(status *Status) {
				status.Connected = false
				status.Stage = "stopped"
				status.RemoteAddress = ""
				status.LatencyMS = 0
			})
			return
		}

		attempts, err := c.routeAttempts()
		if err != nil {
			c.updateStatus(func(status *Status) {
				status.Stage = "configuration_error"
				status.DirectError = err.Error()
			})
			if !waitContext(ctx, c.cfg.RetryDelay) {
				return
			}
			continue
		}

		var conn *websocket.Conn
		var connectedRoute Route
		for _, attempt := range attempts {
			if ctx.Err() != nil {
				return
			}
			c.logAttempt(attempt)
			c.updateStatus(func(status *Status) {
				status.Connected = false
				status.Route = attempt.route
				status.Stage = stageForAttempt(attempt.route)
				status.RemoteAddress = ""
				status.LatencyMS = 0
			})
			started := time.Now()
			next, failure := c.dial(ctx, attempt)
			if failure != nil {
				c.log.Printf("[网络] %s失败 stage=%s error=%q", routeDisplay(attempt.route), failure.stage, failure.err)
				c.updateStatus(func(status *Status) {
					status.Stage = failure.stage
					if attempt.route == RouteCloudIPv4 {
						status.CloudError = failure.err.Error()
					} else {
						status.DirectError = failure.err.Error()
					}
				})
				continue
			}
			conn = next
			connectedRoute = attempt.route
			latency := time.Since(started).Milliseconds()
			if c.onConnect != nil {
				if err := c.onConnect(conn, connectedRoute); err != nil {
					_ = conn.Close()
					conn = nil
					c.log.Printf("[网络] %s协议握手发送失败 error=%q", routeDisplay(attempt.route), err)
					c.updateStatus(func(status *Status) {
						status.Stage = "protocol_hello"
						if attempt.route == RouteCloudIPv4 {
							status.CloudError = err.Error()
						} else {
							status.DirectError = err.Error()
						}
					})
					continue
				}
			}
			if failure := c.readProtocolHello(conn, attempt.timeout); failure != nil {
				_ = conn.Close()
				conn = nil
				c.log.Printf("[网络] %s协议握手失败 stage=%s error=%q", routeDisplay(attempt.route), failure.stage, failure.err)
				c.updateStatus(func(status *Status) {
					status.Stage = failure.stage
					if attempt.route == RouteCloudIPv4 {
						status.CloudError = failure.err.Error()
					} else {
						status.DirectError = failure.err.Error()
					}
				})
				continue
			}
			if c.onActivity != nil {
				c.onActivity()
			}
			remote := conn.UnderlyingConn().RemoteAddr().String()
			c.updateStatus(func(status *Status) {
				status.Connected = true
				status.Route = connectedRoute
				status.Stage = "connected"
				status.RemoteAddress = remote
				status.LatencyMS = latency
				if connectedRoute == RouteIPv6Direct || connectedRoute == RouteLocal {
					status.DirectError = ""
				} else {
					status.CloudError = ""
				}
			})
			c.log.Printf("[网络] 已连接 route=%s remote=%s latency_ms=%d", connectedRoute, remote, latency)
			break
		}

		if conn == nil {
			c.updateStatus(func(status *Status) {
				status.Connected = false
				status.Route = RouteNone
				status.Stage = "retry_wait"
				status.RemoteAddress = ""
				status.LatencyMS = 0
			})
			c.log.Printf("[网络] IPv6 直连和 IPv4 云转发均失败，%s 后重试", c.cfg.RetryDelay)
			if !waitContext(ctx, c.cfg.RetryDelay) {
				return
			}
			continue
		}

		errCh := make(chan error, 2)
		go c.readLoop(conn, errCh)
		go c.writeLoop(conn, errCh)

		var disconnectErr error
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return
		case disconnectErr = <-errCh:
			_ = conn.Close()
		}
		c.log.Printf("[网络] 连接中断 route=%s error=%q，将重新检测直连", connectedRoute, disconnectErr)
		c.updateStatus(func(status *Status) {
			status.Connected = false
			status.Stage = "disconnected"
			status.RemoteAddress = ""
			status.LatencyMS = 0
			if connectedRoute == RouteCloudIPv4 {
				status.CloudError = disconnectErr.Error()
			} else {
				status.DirectError = disconnectErr.Error()
			}
		})
		if !waitContext(ctx, c.cfg.RetryDelay) {
			return
		}
	}
}

func (c *Client) routeAttempts() ([]routeAttempt, error) {
	parsed, err := url.Parse(c.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid direct URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported WebSocket scheme %q", parsed.Scheme)
	}
	host := parsed.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		defaultPort := "80"
		if parsed.Scheme == "wss" {
			defaultPort = "443"
		}
		host = net.JoinHostPort(parsed.Hostname(), defaultPort)
	}

	attempts := make([]routeAttempt, 0, 3)
	if c.cfg.PreferLocal {
		_, port, err := net.SplitHostPort(host)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, routeAttempt{
			route:       RouteLocal,
			network:     "tcp6",
			dialAddress: net.JoinHostPort("::1", port),
			timeout:     c.cfg.DirectTimeout,
		})
	}
	attempts = append(attempts, routeAttempt{
		route:       RouteIPv6Direct,
		network:     "tcp6",
		dialAddress: host,
		timeout:     c.cfg.DirectTimeout,
	})
	if strings.TrimSpace(c.cfg.CloudDialAddress) != "" {
		attempts = append(attempts, routeAttempt{
			route:       RouteCloudIPv4,
			network:     "tcp4",
			dialAddress: c.cfg.CloudDialAddress,
			timeout:     c.cfg.CloudTimeout,
		})
	}
	return attempts, nil
}

type dialFailure struct {
	stage string
	err   error
}

func (c *Client) dial(parent context.Context, attempt routeAttempt) (*websocket.Conn, *dialFailure) {
	ctx, cancel := context.WithTimeout(parent, attempt.timeout)
	defer cancel()

	netDialer := &net.Dialer{Timeout: attempt.timeout, KeepAlive: 30 * time.Second}
	dialer := websocket.Dialer{
		HandshakeTimeout: attempt.timeout,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return netDialer.DialContext(ctx, attempt.network, attempt.dialAddress)
		},
	}
	conn, resp, err := dialer.DialContext(ctx, c.cfg.URL, nil)
	if err == nil {
		return conn, nil
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	return nil, &dialFailure{stage: classifyDialFailure(err, resp), err: err}
}

func classifyDialFailure(err error, resp *http.Response) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	var certErr x509.UnknownAuthorityError
	if errors.As(err, &certErr) {
		return "tls_certificate"
	}
	var hostErr x509.HostnameError
	if errors.As(err, &hostErr) {
		return "tls_hostname"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "tcp_timeout"
	}
	if errors.Is(err, websocket.ErrBadHandshake) || resp != nil {
		return "websocket_handshake"
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "no suitable address"), strings.Contains(lower, "no such host"):
		return "dns"
	case strings.Contains(lower, "certificate"), strings.Contains(lower, "tls"):
		return "tls"
	case strings.Contains(lower, "refused"):
		return "tcp_refused"
	case strings.Contains(lower, "network is unreachable"), strings.Contains(lower, "no route"):
		return "ipv6_unavailable"
	default:
		return "tcp_connect"
	}
}

func (c *Client) readLoop(conn *websocket.Conn, errCh chan<- error) {
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case errCh <- err:
			default:
			}
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		if c.onActivity != nil {
			c.onActivity()
		}
		env := &videowithyoupb.Envelope{}
		if err := proto.Unmarshal(data, env); err != nil {
			c.log.Printf("[网络] 收到无效协议消息 error=%q", err)
			continue
		}
		select {
		case c.incoming <- env:
		default:
			c.log.Printf("[网络] 接收队列已满")
		}
	}
}

func (c *Client) readProtocolHello(conn *websocket.Conn, timeout time.Duration) *dialFailure {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	messageType, data, err := conn.ReadMessage()
	if err != nil {
		return &dialFailure{stage: "protocol_hello_timeout", err: err}
	}
	if messageType != websocket.BinaryMessage {
		return &dialFailure{stage: "protocol_hello", err: fmt.Errorf("expected binary server hello")}
	}
	env := &videowithyoupb.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return &dialFailure{stage: "protocol_hello", err: err}
	}
	if rejected := env.GetErrorResp(); rejected != nil {
		select {
		case c.incoming <- env:
		default:
		}
		return &dialFailure{stage: "protocol_rejected", err: errors.New(rejected.GetMessage())}
	}
	if env.GetServerHello() == nil {
		return &dialFailure{stage: "protocol_hello", err: errors.New("missing server_hello")}
	}
	select {
	case c.incoming <- env:
	default:
		return &dialFailure{stage: "protocol_hello", err: errors.New("incoming queue full")}
	}
	_ = conn.SetReadDeadline(time.Time{})
	return nil
}

func (c *Client) writeLoop(conn *websocket.Conn, errCh chan<- error) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case payload := <-c.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if c.onActivity != nil {
				c.onActivity()
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}
}

func (c *Client) updateStatus(update func(*Status)) {
	c.statusMu.Lock()
	update(&c.status)
	status := c.status
	c.statusMu.Unlock()
	if c.onStatus != nil {
		c.onStatus(status)
	}
}

func (c *Client) logAttempt(attempt routeAttempt) {
	if attempt.route == RouteCloudIPv4 {
		c.log.Printf("[网络] 正在尝试 IPv4 云转发 dial=%s timeout=%s", attempt.dialAddress, attempt.timeout)
		return
	}
	if attempt.route == RouteLocal {
		c.log.Printf("[网络] 正在连接本机内嵌服务端 dial=%s timeout=%s", attempt.dialAddress, attempt.timeout)
		return
	}
	c.log.Printf("[网络] 正在尝试 IPv6 直连 endpoint=%s timeout=%s", attempt.dialAddress, attempt.timeout)
}

func stageForAttempt(route Route) string {
	switch route {
	case RouteLocal:
		return "trying_local"
	case RouteIPv6Direct:
		return "trying_ipv6"
	case RouteCloudIPv4:
		return "trying_cloud_ipv4"
	default:
		return "connecting"
	}
}

func routeDisplay(route Route) string {
	switch route {
	case RouteLocal:
		return "本机服务端"
	case RouteIPv6Direct:
		return "IPv6 直连"
	case RouteCloudIPv4:
		return "IPv4 云转发"
	default:
		return "服务器连接"
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
