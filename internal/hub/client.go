package hub

import (
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait   = 10 * time.Second // Time to write a message
	pongWait    = 60 * time.Second // Time to wait for pong
	pingPeriod  = 30 * time.Second // Ping interval (D-08, must be < pongWait)
	maxMsgSize  = 4096             // Maximum inbound message size (4KB, T-02-06)
	sendBufSize = 256              // Client send channel buffer (T-02-05)
)

// Client represents a connected WebSocket client.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	section string      // currently subscribed section (one at a time per D-18)
	closed  atomic.Bool // set by removeClient/shutdown; checked before sending to avoid panic on closed channel (CR-01)
	logger  *slog.Logger
}

// NewClient creates a new Client for the given WebSocket connection.
func NewClient(hub *Hub, conn *websocket.Conn, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, sendBufSize),
		logger: logger.With("component", "client"),
	}
}

// ReadPump reads messages from the WebSocket and forwards to hub commands channel.
// Must be called in a dedicated goroutine. Returns when connection closes.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Debug("websocket read error", "error", err)
			}
			break
		}
		var msg InboundMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.logger.Debug("invalid message JSON", "error", err)
			continue
		}
		c.hub.Command(c, msg)
	}
}

// WritePump writes messages from the send channel to the WebSocket.
// Also sends periodic ping frames (D-08). Must be called in a dedicated goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
