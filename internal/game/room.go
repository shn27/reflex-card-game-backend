package game

import (
	"strconv"
	"sync"
	"time"

	"github.com/shn27/reflex-card-game-backend/internal/config"
	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"go.uber.org/zap"
)

type Phase int

const (
	PhaseWaiting Phase = iota
	PhasePlaying
	PhaseFinished
)

// Room is a single game session.
type Room struct {
	mu     sync.Mutex
	id     string
	kind   RoomKind
	secret string // only set for RoomKindSecret; empty otherwise

	players []*Player // slice so guests can be removed without index gymnastics
	count   int       // len(players) minus nils; kept in sync manually

	deck      []Card
	cardIndex int
	phase     Phase

	cardInterval time.Duration
}

// newRoom is package-private — callers go through Matchmaker.
func newRoom(id string, cardInterval time.Duration, kind RoomKind) *Room {
	return &Room{
		id:           id,
		kind:         kind,
		deck:         NewShuffledDeck(),
		cardIndex:    -1,
		phase:        PhaseWaiting,
		cardInterval: cardInterval,
	}
}

func (r *Room) ID() string { return r.id }

// AddPlayer appends a player and returns their assigned ID.
// For anonymous rooms the caller must enforce the two-player cap externally.
// For secret rooms there is no hard cap — the host decides when to start.
func (r *Room) AddPlayer(conn Sender) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.kind == RoomKindSecret {
		if r.count == int(config.Cfg.MaxNumberOfPlayerAllowedInRoom) {
			return 0, false
		}
	}

	r.count++
	p := NewPlayer(r.count, conn)
	r.players = append(r.players, p)
	return p.ID, true
}

func (r *Room) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count == 2
}

// ── Game loop ─────────────────────────────────────────────────────────────────

func (r *Room) Start() {
	r.mu.Lock()
	r.phase = PhasePlaying
	r.mu.Unlock()

	logger.Logger.Info("[room] game started", zap.String("room_id", r.id))

	ticker := time.NewTicker(r.cardInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()

		if r.phase == PhaseFinished {
			r.mu.Unlock()
			return
		}

		r.cardIndex++

		if r.cardIndex >= len(r.deck) {
			r.phase = PhaseFinished
			r.mu.Unlock()
			logger.Logger.Info("[room] draw — all cards shown", zap.String("room_id", r.id))
			r.broadcast(OutMessage{
				Type:   MsgGameOver,
				Result: "draw",
				Reason: "All 52 cards were shown with no click",
			})
			return
		}

		card := r.deck[r.cardIndex]
		idx := r.cardIndex
		r.mu.Unlock()

		logger.Logger.Info("[room] card revealed",
			zap.String("room_id", r.id),
			zap.Int("index", idx+1),
			zap.String("rank", card.Rank),
			zap.String("suit", card.Suit),
		)

		r.broadcast(OutMessage{
			Type:      MsgCardReveal,
			Card:      &card,
			CardIndex: idx + 1,
		})
	}
}

// HandleClick is called when playerID sends a click.
func (r *Room) HandleClick(playerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.phase != PhasePlaying || r.cardIndex < 0 {
		return
	}

	r.phase = PhaseFinished
	card := r.deck[r.cardIndex]

	if card.IsAce() {
		logger.Logger.Info("[room] ACE clicked — player wins",
			zap.String("room_id", r.id),
			zap.Int("player_id", playerID),
		)
	} else {
		logger.Logger.Info("[room] false click — player loses",
			zap.String("room_id", r.id),
			zap.Int("player_id", playerID),
		)
	}

	for _, p := range r.players {
		if p == nil {
			continue
		}
		if card.IsAce() {
			if p.ID == playerID {
				p.Send(OutMessage{Type: MsgGameOver, Result: "win",
					Reason: "You clicked the Ace first!", WinnerID: playerID, Card: &card})
			} else {
				p.Send(OutMessage{Type: MsgGameOver, Result: "lose",
					Reason: "Your opponent clicked the Ace first", WinnerID: playerID, Card: &card})
			}
		} else {
			if p.ID == playerID {
				p.Send(OutMessage{Type: MsgGameOver, Result: "lose",
					Reason: "You clicked on a non-Ace card!", Card: &card})
			} else {
				p.Send(OutMessage{Type: MsgGameOver, Result: "win",
					Reason: "Your opponent clicked a non-Ace card", WinnerID: p.ID, Card: &card})
			}
		}
	}
}

// OpponentDisconnected ends a game-in-progress because a player left.
// Safe to call without holding mu.
func (r *Room) OpponentDisconnected(survivorID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.phase == PhaseFinished {
		return
	}
	r.phase = PhaseFinished

	r.sendToLocked(survivorID, OutMessage{
		Type:   MsgGameOver,
		Result: "win",
		Reason: "Your opponent disconnected",
	})
}

// ── Helpers called with mu held ───────────────────────────────────────────────

// removePlayerLocked removes playerID from the slice and decrements count.
// Must be called with mu held.
func (r *Room) removePlayerLocked(playerID int) {
	for i, p := range r.players {
		if p != nil && p.ID == playerID {
			r.players = append(r.players[:i], r.players[i+1:]...)
			r.count--
			return
		}
	}
}

// firstPlayerLocked returns the first non-nil player.
// Must be called with mu held.
func (r *Room) firstPlayerLocked() *Player {
	for _, p := range r.players {
		if p != nil {
			return p
		}
	}
	return nil
}

// playerSnapshot returns a []RoomPlayer safe to use outside the lock.
// Must be called with mu held.
func (r *Room) playerSnapshot() []RoomPlayer {
	out := make([]RoomPlayer, 0, r.count)
	for i, p := range r.players {
		if p == nil {
			continue
		}
		out = append(out, RoomPlayer{
			ID:     p.ID,
			Name:   "Player-" + strconv.Itoa(p.ID),
			IsHost: i == 0,
		})
	}
	return out
}

// broadcastLocked sends to all players. Must be called with mu held.
func (r *Room) broadcastLocked(msg OutMessage) {
	for _, p := range r.players {
		if p != nil {
			p.Send(msg)
		}
	}
}

// broadcast sends to all players without holding mu.
func (r *Room) broadcast(msg OutMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcastLocked(msg)
}

// broadcastExcept sends to all players except excludeID, without holding mu.
func (r *Room) broadcastExcept(excludeID int, msg OutMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		if p != nil && p.ID != excludeID {
			p.Send(msg)
		}
	}
}

// sendToLocked sends to a specific player. Must be called with mu held.
func (r *Room) sendToLocked(playerID int, msg OutMessage) {
	for _, p := range r.players {
		if p != nil && p.ID == playerID {
			p.Send(msg)
			return
		}
	}
}
