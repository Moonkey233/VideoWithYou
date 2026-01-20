package ws

import (
	"context"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	videowithyoupb "videowithyou/v2/proto/gen"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 30 * time.Second
	pingPeriod     = 15 * time.Second
	reconnectDelay = 2 * time.Second
)

type Client struct {
	url        string
	log        *log.Logger
	incoming   chan *videowithyoupb.Envelope
	send       chan []byte
	onConnect  func(*websocket.Conn) error
	onStatus   func(bool)
	onActivity func()
}

func NewClient(url string, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.Default()
	}
	return &Client{
		url:      url,
		log:      logger,
		incoming: make(chan *videowithyoupb.Envelope, 128),
		send:     make(chan []byte, 128),
	}
}

func (c *Client) SetOnConnect(fn func(*websocket.Conn) error) {
	c.onConnect = fn
}

func (c *Client) SetOnStatus(fn func(bool)) {
	c.onStatus = fn
}

func (c *Client) SetOnActivity(fn func()) {
	c.onActivity = fn
}

func (c *Client) Incoming() <-chan *videowithyoupb.Envelope {
	return c.incoming
}

func (c *Client) Send(env *videowithyoupb.Envelope) {
	payload, err := proto.Marshal(env)
	if err != nil {
		c.log.Printf("ws marshal failed: %v", err)
		return
	}
	select {
	case c.send <- payload:
	default:
		c.log.Printf("ws send queue full")
	}
}

func (c *Client) Start(ctx context.Context) {
	connected := false
	for {
		select {
		case <-ctx.Done():
			if connected && c.onStatus != nil {
				c.onStatus(false)
			}
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
		if err != nil {
			if connected && c.onStatus != nil {
				connected = false
				c.onStatus(false)
			}
			c.log.Printf("ws connect failed: %v", err)
			time.Sleep(reconnectDelay)
			continue
		}

		conn.SetReadLimit(2 << 20)
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		if c.onConnect != nil {
			if err := c.onConnect(conn); err != nil {
				_ = conn.Close()
				time.Sleep(reconnectDelay)
				continue
			}
		}
		if c.onActivity != nil {
			c.onActivity()
		}
		c.log.Printf("ws connected %s", c.url)
		if !connected && c.onStatus != nil {
			connected = true
			c.onStatus(true)
		}

		errCh := make(chan error, 2)
		go c.readLoop(conn, errCh)
		go c.writeLoop(conn, errCh)

		select {
		case <-ctx.Done():
			_ = conn.Close()
			if connected && c.onStatus != nil {
				connected = false
				c.onStatus(false)
			}
			return
		case <-errCh:
			_ = conn.Close()
			c.log.Printf("ws disconnected, retrying")
			if connected && c.onStatus != nil {
				connected = false
				c.onStatus(false)
			}
			time.Sleep(reconnectDelay)
		}
	}
}

func (c *Client) readLoop(conn *websocket.Conn, errCh chan<- error) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			errCh <- err
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
			c.log.Printf("ws bad envelope: %v", err)
			continue
		}
		select {
		case c.incoming <- env:
		default:
			c.log.Printf("ws incoming queue full")
		}
	}
}

func (c *Client) writeLoop(conn *websocket.Conn, errCh chan<- error) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case payload := <-c.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				errCh <- err
				return
			}
			if c.onActivity != nil {
				c.onActivity()
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				errCh <- err
				return
			}
			if c.onActivity != nil {
				c.onActivity()
			}
		}
	}
}
