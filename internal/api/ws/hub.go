// Package ws provides WebSocket connection management.
package ws

import (
	"context"
	"log/slog"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
)

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	MsgLog   MessageType = "log"
	MsgEvent MessageType = "event"
)

// Message is a WebSocket message.
type Message struct {
	Type    MessageType `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client represents a WebSocket client connection.
type Client struct {
	ID     int
	Conn   *websocket.Conn
	Filter MessageType
}

// Hub manages WebSocket connections.
type Hub struct {
	mu        sync.RWMutex
	clients   map[int]*Client
	nextID    int
	broadcast chan Message
	stopCh    chan struct{}
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[int]*Client),
		broadcast: make(chan Message, 256),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the hub's broadcast loop.
func (h *Hub) Start() {
	go h.run()
	slog.Info("websocket hub started")
}

// Stop stops the hub.
func (h *Hub) Stop() {
	close(h.stopCh)
	h.mu.Lock()
	for _, client := range h.clients {
		_ = client.Conn.CloseNow()
	}
	h.clients = make(map[int]*Client)
	h.mu.Unlock()
}

func (h *Hub) run() {
	for {
		select {
		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				if client.Filter != "" && client.Filter != msg.Type {
					continue
				}
				if err := wsjson.Write(context.Background(), client.Conn, msg); err != nil {
					slog.Debug("websocket write error", "client", client.ID, "error", err)
				}
			}
			h.mu.RUnlock()
		case <-h.stopCh:
			return
		}
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	select {
	case h.broadcast <- msg:
	default:
		// Drop if buffer is full.
	}
}

// HandleWebSocket handles WebSocket upgrade requests.
func (h *Hub) HandleWebSocket(filter MessageType) gin.HandlerFunc {
	return func(c *gin.Context) {
		// nil options = library default origin verification: browser
		// connections must be same-origin (Origin matches Host); non-browser
		// clients send no Origin and are allowed.
		conn, err := websocket.Accept(c.Writer, c.Request, nil)
		if err != nil {
			slog.Error("websocket accept error", "error", err)
			return
		}

		h.mu.Lock()
		h.nextID++
		client := &Client{
			ID:     h.nextID,
			Conn:   conn,
			Filter: filter,
		}
		h.clients[client.ID] = client
		h.mu.Unlock()

		slog.Info("websocket client connected", "id", client.ID, "filter", filter)
		defer func() {
			h.mu.Lock()
			delete(h.clients, client.ID)
			h.mu.Unlock()
			_ = conn.CloseNow()
			slog.Info("websocket client disconnected", "id", client.ID)
		}()

		// Keep connection alive, read messages (for ping/pong).
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				break
			}
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
