package game

import (
	"log"
	"sync"
	"time"
)

type Phase int

const (
	PhaseWaiting Phase = iota
	PhasePlaying
	PhaseFinished
)

// Room is a single game session between two players.
type Room struct {
	mu      sync.Mutex
	id      string
	players [2]*Player
	count   int // number of players currently in the room

	deck      []Card
	cardIndex int // current position in the deck (-1 = not started)
	phase     Phase

	cardInterval time.Duration
}

func NewRoom(id string, cardInterval time.Duration) *Room {
	return &Room{
		id:           id,
		deck:         NewShuffledDeck(),
		cardIndex:    -1,
		phase:        PhaseWaiting,
		cardInterval: cardInterval,
	}
}

func (r *Room) ID() string { return r.id }

// AddPlayer adds a player to the room.
// Returns the assigned player ID (1 or 2) and whether the add succeeded.
func (r *Room) AddPlayer(conn Sender) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count >= 2 {
		return 0, false
	}

	r.count++
	p := NewPlayer(r.count, conn)
	r.players[r.count-1] = p
	return p.ID, true
}

func (r *Room) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count == 2
}

// Start begins the card-reveal loop. Must be called after both players have joined.
func (r *Room) Start() {
	r.mu.Lock()
	r.phase = PhasePlaying
	r.mu.Unlock()

	log.Printf("[room %s] game started", r.id)

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
			log.Printf("[room %s] draw — all 52 cards shown", r.id)
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

		log.Printf("[room %s] card %d/52: %s of %s", r.id, idx+1, card.Rank, card.Suit)
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
	opponentID := 3 - playerID

	if card.IsAce() {
		log.Printf("[room %s] player %d clicked ACE (%s of %s) — wins", r.id, playerID, card.Rank, card.Suit)
		r.sendTo(playerID, OutMessage{
			Type:     MsgGameOver,
			Result:   "win",
			Reason:   "You clicked the Ace first!",
			WinnerID: playerID,
			Card:     &card,
		})
		r.sendTo(opponentID, OutMessage{
			Type:     MsgGameOver,
			Result:   "lose",
			Reason:   "Your opponent clicked the Ace first",
			WinnerID: playerID,
			Card:     &card,
		})
		return
	}

	log.Printf("[room %s] player %d false-clicked %s of %s — loses", r.id, playerID, card.Rank, card.Suit)
	r.sendTo(playerID, OutMessage{
		Type:   MsgGameOver,
		Result: "lose",
		Reason: "You clicked on a non-Ace card!",
		Card:   &card,
	})
	r.sendTo(opponentID, OutMessage{
		Type:     MsgGameOver,
		Result:   "win",
		Reason:   "Your opponent clicked a non-Ace card",
		WinnerID: opponentID,
		Card:     &card,
	})
}

// OpponentDisconnected ends the game because one player left.
func (r *Room) OpponentDisconnected(survivorID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.phase == PhaseFinished {
		return
	}
	r.phase = PhaseFinished

	r.sendTo(survivorID, OutMessage{
		Type:   MsgGameOver,
		Result: "win",
		Reason: "Your opponent disconnected",
	})
}

// ---- helpers (must be called with mu held or on safe goroutine) ----

func (r *Room) broadcast(msg OutMessage) {
	for _, p := range r.players {
		if p != nil {
			p.Send(msg)
		}
	}
}

func (r *Room) sendTo(playerID int, msg OutMessage) {
	p := r.players[playerID-1]
	if p != nil {
		p.Send(msg)
	}
}
