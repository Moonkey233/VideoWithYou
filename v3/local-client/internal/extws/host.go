package extws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const writeWait = 5 * time.Second

type Host struct {
	log      *log.Logger
	addr     string
	path     string
	incoming chan []byte
	send     chan []byte
	upgrader websocket.Upgrader

	mu   sync.RWMutex
	conn *websocket.Conn
}

func NewHost(addr, path string, logger *log.Logger) *Host {
	if logger == nil {
		logger = log.Default()
	}
	if path == "" {
		path = "/ext"
	}
	return &Host{
		log:      logger,
		addr:     addr,
		path:     path,
		incoming: make(chan []byte, 64),
		send:     make(chan []byte, 64),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

func (h *Host) Start(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc(h.path, h.handleWS)

	server := &http.Server{Addr: h.addr, Handler: mux}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.log.Printf("ext ws listen failed: %v", err)
		}
	}()

	go h.writeLoop(ctx)
}

func (h *Host) Incoming() <-chan []byte {
	return h.incoming
}

func (h *Host) Send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case h.send <- data:
	default:
		h.log.Printf("ext ws send queue full")
	}
	return nil
}

func (h *Host) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Printf("ext ws upgrade failed: %v", err)
		return
	}
	h.setConn(conn)
	h.readLoop(conn)
}

func (h *Host) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		select {
		case h.incoming <- data:
		default:
			h.log.Printf("ext ws incoming queue full")
		}
	}
	h.clearConn(conn)
}

func (h *Host) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-h.send:
			conn := h.getConn()
			if conn == nil {
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				h.log.Printf("ext ws write failed: %v", err)
			}
		}
	}
}

func (h *Host) setConn(conn *websocket.Conn) {
	h.mu.Lock()
	if h.conn != nil {
		_ = h.conn.Close()
	}
	h.conn = conn
	h.mu.Unlock()
	h.log.Printf("ext ws connected")
}

func (h *Host) clearConn(conn *websocket.Conn) {
	h.mu.Lock()
	if h.conn == conn {
		h.conn = nil
	}
	h.mu.Unlock()
	h.log.Printf("ext ws disconnected")
}

func (h *Host) getConn() *websocket.Conn {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()
	return conn
}
