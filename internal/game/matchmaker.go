package game

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"go.uber.org/zap"
)

// RoomKind distinguishes how a room was created.
// It drives Leave behaviour — anonymous and secret rooms have different
// semantics for host departure, mid-game dropout, etc.
type RoomKind int

const (
	RoomKindAnonymous RoomKind = iota // paired via the global matchmaking queue
	RoomKindSecret                    // created explicitly with a short code
)

// Matchmaker pairs players into game rooms.
type Matchmaker struct {
	mu      sync.Mutex
	waiting *waitingPlayer

	rooms        sync.Map // roomID  → *Room
	secrets      sync.Map // secret  → roomID  (deleted once game starts)
	roomSeq      atomic.Uint64
	cardInterval time.Duration
}

type waitingPlayer struct {
	conn    Sender
	roomID  string
	setRoom func(roomID string, playerID int)
}

func NewMatchmaker(cardInterval time.Duration) *Matchmaker {
	return &Matchmaker{cardInterval: cardInterval}
}

// ── Anonymous matchmaking ─────────────────────────────────────────────────────

// Join enqueues a player into the global queue. If a partner is already
// waiting a room is created, both players are notified, and the game starts.
func (m *Matchmaker) Join(conn Sender, setRoom func(roomID string, playerID int)) {
	m.mu.Lock()

	if m.waiting == nil {
		roomID := fmt.Sprintf("room-%d", m.roomSeq.Add(1))
		m.waiting = &waitingPlayer{conn: conn, roomID: roomID, setRoom: setRoom}
		m.mu.Unlock()

		setRoom(roomID, 1)
		logger.Logger.Info("[matchmaker] player waiting for opponent")
		conn.Send(OutMessage{Type: MsgWaiting, Message: "Waiting for an opponent…"})
		return
	}

	w := m.waiting
	m.waiting = nil
	m.mu.Unlock()

	room := newRoom(w.roomID, m.cardInterval, RoomKindAnonymous)
	m.rooms.Store(room.ID(), room)

	p1ID, _ := room.AddPlayer(w.conn)
	p2ID, _ := room.AddPlayer(conn)

	w.setRoom(room.ID(), p1ID)
	setRoom(room.ID(), p2ID)

	logger.Logger.Info("[matchmaker] anonymous pair created", zap.String("room_id", room.ID()))

	w.conn.Send(OutMessage{Type: MsgGameStart, PlayerID: p1ID})
	conn.Send(OutMessage{Type: MsgGameStart, PlayerID: p2ID})

	go room.Start()
}

// ── Secret room management ────────────────────────────────────────────────────

// CreateRoom creates a new secret room, adds the host as player 1, and returns
// the roomID and playerID. The short code (secret) is stored separately.
func (m *Matchmaker) CreateRoom(conn Sender, setRoom func(roomID string, playerID int)) {
	secret := GetSecretString(6)
	roomID := fmt.Sprintf("room-%d", m.roomSeq.Add(1))

	room := newRoom(roomID, m.cardInterval, RoomKindSecret)
	room.secret = secret

	m.rooms.Store(roomID, room)
	m.secrets.Store(secret, roomID)

	playerID, _ := room.AddPlayer(conn)
	setRoom(roomID, playerID)

	logger.Logger.Info("[matchmaker] secret room created",
		zap.String("room_id", roomID),
		zap.String("secret", secret),
	)

	conn.Send(OutMessage{Type: MsgRoomCreated, PlayerID: playerID, Secret: secret})
}

// JoinRoom adds a player to an existing secret room identified by its short code.
func (m *Matchmaker) JoinRoom(conn Sender, secret string, setRoom func(roomID string, playerID int)) {
	roomID, ok := m.getRoomIDBySecret(secret)
	if !ok {
		conn.Send(OutMessage{Type: MsgRoomInvalid, Message: "Room not found"})
		return
	}

	room, ok := m.getRoom(roomID)
	if !ok {
		conn.Send(OutMessage{Type: MsgRoomInvalid, Message: "Room not found"})
		return
	}

	playerID, ok := room.AddPlayer(conn)
	if !ok {
		conn.Send(OutMessage{Type: MsgRoomInvalid, Message: "Room is full"})
		return
	}

	setRoom(roomID, playerID)

	room.mu.Lock()
	snapshot := room.playerSnapshot()
	room.mu.Unlock()

	logger.Logger.Info("[matchmaker] player joined secret room",
		zap.String("room_id", roomID),
		zap.Int("player_id", playerID),
	)

	// Tell the joiner their full lobby state.
	conn.Send(OutMessage{
		Type:     MsgRoomJoined,
		PlayerID: playerID,
		Secret:   secret,
		Players:  snapshot,
	})

	// Tell everyone else the lobby updated.
	room.broadcastExcept(playerID, OutMessage{
		Type:    MsgRoomUpdated,
		Secret:  secret,
		Players: snapshot,
	})
}

// StartRoom begins the game for a secret room. Only the host (playerID == 1)
// should be able to call this — enforce that in the WS handler.
func (m *Matchmaker) StartRoom(secret string, callerID int, conn Sender) {
	roomID, ok := m.getRoomIDBySecret(secret)
	if !ok {
		conn.Send(OutMessage{Type: MsgRoomInvalid, Message: "Room not found"})
		return
	}

	room, ok := m.getRoom(roomID)
	if !ok {
		conn.Send(OutMessage{Type: MsgRoomInvalid, Message: "Room not found"})
		return
	}

	if callerID != 1 {
		conn.Send(OutMessage{Type: MsgRoomInvalid, Message: "Only the host can start the game"})
		return
	}

	// Secret no longer needed once the game is running.
	m.secrets.Delete(secret)

	room.broadcast(OutMessage{Type: MsgRoomStarted})

	logger.Logger.Info("[matchmaker] secret room started", zap.String("room_id", roomID))

	go room.Start()
}

// Leave is the single entry point called by the WS handler whenever a
// connection closes, regardless of room type or game phase.

// Phase    | Kind      | Who left  | Action
// ---------|-----------|-----------|------------------------------------------
// Waiting  | Anonymous | only p1   | clear waiting slot (Join handles this)
// Waiting  | Secret    | host (1)  | notify all → MsgRoomHostLeft, delete room
// Waiting  | Secret    | guest     | remove from lobby, broadcast MsgRoomUpdated
// Playing  | Anonymous | anyone    | survivor wins (MsgGameOver)
// Playing  | Secret    | anyone    | if last player → survivor wins
//
//	if others remain → silent remove, no message
func (m *Matchmaker) Leave(conn Sender, roomID string, playerID int) {
	// Player disconnected before being matched (anonymous waiting slot).
	if roomID == "" {
		m.clearWaitingSlot(conn)
		return
	}

	room, ok := m.getRoom(roomID)
	if !ok {
		return
	}

	room.mu.Lock()
	//defer room.mu.Unlock()

	switch room.phase {
	case PhaseWaiting:
		m.leaveWaiting(room, conn, playerID)
	case PhasePlaying, PhaseFinished:
		m.leavePlaying(room, playerID)
	}
	// leaveWaiting / leavePlaying both release mu before returning.
}

// leaveWaiting handles a disconnect while the room is in the lobby/waiting phase.
// Called with room.mu held; releases it before returning.
func (m *Matchmaker) leaveWaiting(room *Room, conn Sender, playerID int) {
	defer room.mu.Unlock()

	switch room.kind {

	case RoomKindAnonymous:
		// Anonymous waiting rooms only ever have one player (they start the
		// moment a second player joins). Leaving just clears the global slot.
		m.clearWaitingSlot(conn)

	case RoomKindSecret:
		if playerID == 1 {
			// Host left — shut the whole lobby down.
			room.broadcastLocked(OutMessage{Type: MsgRoomHostLeft})
			m.secrets.Delete(room.secret)
			m.rooms.Delete(room.id)

			logger.Logger.Info("[matchmaker] secret room host left, room closed",
				zap.String("room_id", room.id),
			)
		} else {
			// Guest left — remove them, update everyone remaining.
			room.removePlayerLocked(playerID)
			snapshot := room.playerSnapshot()

			room.broadcastLocked(OutMessage{
				Type:    MsgRoomUpdated,
				Secret:  room.secret,
				Players: snapshot,
			})

			logger.Logger.Info("[matchmaker] guest left secret room",
				zap.Int("player_id", playerID),
				zap.String("room_id", room.id),
			)
		}
	}
}

// leavePlaying handles a disconnect while the game is running.
// Called with room.mu held; releases it before returning.
func (m *Matchmaker) leavePlaying(room *Room, playerID int) {
	if room.phase == PhaseFinished {
		room.mu.Unlock()
		return
	}

	room.removePlayerLocked(playerID)

	logger.Logger.Info("[matchmaker] player left mid-game",
		zap.Int("player_id", playerID),
		zap.String("room_id", room.id),
		zap.Int("remaining", room.count),
	)

	switch room.kind {

	case RoomKindAnonymous:
		// Always exactly two players — the remaining one wins.
		survivor := room.firstPlayerLocked()
		room.mu.Unlock()
		if survivor != nil {
			room.OpponentDisconnected(survivor.ID)
		}

	case RoomKindSecret:
		if room.count == 1 {
			// One player left — they win.
			survivor := room.firstPlayerLocked()
			room.mu.Unlock()
			if survivor != nil {
				room.OpponentDisconnected(survivor.ID)
			}
		} else {
			// More than one player still in — just silently drop the leaver.
			// No frontend message; the game continues.
			room.mu.Unlock()
		}
	}
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (m *Matchmaker) clearWaitingSlot(conn Sender) {
	//m.mu.Lock()
	if m.waiting != nil && m.waiting.conn == conn {
		m.waiting = nil
		logger.Logger.Info("[matchmaker] waiting player disconnected before match")
	}
	//m.mu.Unlock()
}

func (m *Matchmaker) getRoom(roomID string) (*Room, bool) {
	val, ok := m.rooms.Load(roomID)
	if !ok {
		return nil, false
	}
	return val.(*Room), true
}

// GetRoom is the public accessor used by the WS handler for click routing.
func (m *Matchmaker) GetRoom(roomID string) (*Room, bool) {
	return m.getRoom(roomID)
}

func (m *Matchmaker) getRoomIDBySecret(secret string) (string, bool) {
	val, ok := m.secrets.Load(secret)
	if !ok {
		return "", false
	}
	return val.(string), true
}
