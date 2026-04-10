package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shn27/reflex-card-game-backend/internal/game"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	sendBuf    = 16
)

// Client is one connected browser tab. It implements game.Sender.
type Client struct {
	conn     *websocket.Conn
	outgoing chan game.OutMessage

	// Set by the matchmaker via the setRoom callback.
	roomID   string
	playerID int
}

func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		conn:     conn,
		outgoing: make(chan game.OutMessage, sendBuf),
	}
}

// Send implements game.Sender. Non-blocking: drops the message if the buffer
// is full (the client is too slow).
func (c *Client) Send(msg game.OutMessage) {
	select {
	case c.outgoing <- msg:
	default:
		log.Printf("[client] send buffer full — dropping %s", msg.Type)
	}
}

// SetRoom is the matchmaker callback; attaches room context to the client.
func (c *Client) SetRoom(roomID string, playerID int) {
	c.roomID = roomID
	c.playerID = playerID
}

// WritePump drains the outgoing channel to the WebSocket.
// Must run in its own goroutine.
// send msg from client to websocket
// if any new msg in the client's chan it immediately sends it to the frontend
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.outgoing:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("[client] write error: %v", err)
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

// ReadPump blocks reading messages from the WebSocket.
// It drives the client lifecycle: join → play → leave.
// Must run in its own goroutine (or the handler goroutine).
func (c *Client) ReadPump(mm *game.Matchmaker) {
	defer func() {
		mm.Leave(c, c.roomID, c.playerID)
		close(c.outgoing)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// why joining into a room is not separated from ReadPump

	mm.Join(c, c.SetRoom)

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[client] read error: %v", err)
			}
			return
		}

		var msg game.InMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[client] malformed message: %v", err)
			continue
		}

		c.route(msg, mm)
	}
}

func (c *Client) route(msg game.InMessage, mm *game.Matchmaker) {
	switch msg.Type {
	case game.MsgClick:
		if c.roomID == "" {
			return
		}
		room, ok := mm.GetRoom(c.roomID)
		if !ok {
			return
		}
		room.HandleClick(c.playerID)

	default:
		log.Printf("[client] unknown message type: %q", msg.Type)
	}
}
