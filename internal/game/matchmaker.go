package game

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Matchmaker pairs players who click Play into game rooms.
type Matchmaker struct {
	mu      sync.Mutex
	waiting *waitingPlayer

	rooms        sync.Map
	roomSeq      atomic.Uint64
	cardInterval time.Duration
}

type waitingPlayer struct {
	conn     Sender
	playerID int // will be 1
	roomID   string
	setRoom  func(roomID string, playerID int)
}

func NewMatchmaker(cardInterval time.Duration) *Matchmaker {
	return &Matchmaker{cardInterval: cardInterval}
}

// Join enqueues a player. If another player is already waiting, a room is
// created, both players are notified, and the game loop starts.
//
// setRoom is a callback the matchmaker uses to attach the room/playerID back
// onto the caller's transport layer (ws.Client), keeping packages decoupled.
func (m *Matchmaker) Join(conn Sender, setRoom func(roomID string, playerID int)) {
	m.mu.Lock()

	if m.waiting == nil {
		roomID := fmt.Sprintf("room-%d", m.roomSeq.Add(1))
		m.waiting = &waitingPlayer{conn: conn, roomID: roomID, setRoom: setRoom}
		m.mu.Unlock()

		setRoom(roomID, 1)
		log.Printf("[matchmaker] player 1 waiting in %s", roomID)
		conn.Send(OutMessage{Type: MsgWaiting, Message: "Waiting for an opponent…"})
		return
	}

	w := m.waiting
	m.waiting = nil
	m.mu.Unlock()

	// Both players are ready — create the room
	room := NewRoom(w.roomID, m.cardInterval)
	m.rooms.Store(room.ID(), room)

	p1ID, _ := room.AddPlayer(w.conn)
	p2ID, _ := room.AddPlayer(conn)

	w.setRoom(room.ID(), p1ID)
	setRoom(room.ID(), p2ID)

	log.Printf("[matchmaker] paired into %s", room.ID())

	w.conn.Send(OutMessage{Type: MsgGameStart, PlayerID: p1ID})
	conn.Send(OutMessage{Type: MsgGameStart, PlayerID: p2ID})

	go room.Start()
}

// Leave is called when a player's connection closes.
func (m *Matchmaker) Leave(conn Sender, roomID string, playerID int) {
	// If this player was still in the waiting slot, clear it.
	m.mu.Lock()
	if m.waiting != nil && m.waiting.conn == conn {
		m.waiting = nil
		m.mu.Unlock()
		log.Printf("[matchmaker] waiting player disconnected")
		return
	}
	m.mu.Unlock()

	if roomID == "" {
		return
	}

	val, loaded := m.rooms.LoadAndDelete(roomID)
	if !loaded {
		return
	}

	room := val.(*Room)
	survivorID := 3 - playerID
	room.OpponentDisconnected(survivorID)
	log.Printf("[matchmaker] player %d left %s", playerID, roomID)
}

// GetRoom returns the room for a given ID.
func (m *Matchmaker) GetRoom(roomID string) (*Room, bool) {
	val, ok := m.rooms.Load(roomID)
	if !ok {
		return nil, false
	}
	return val.(*Room), true
}
