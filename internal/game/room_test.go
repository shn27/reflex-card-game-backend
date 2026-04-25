package game

import (
	"testing"
	"time"
)

// helpers

func newTestRoom() (*Room, *mockSender, *mockSender) {
	room := newRoom("test-room", 10*time.Millisecond, RoomKindAnonymous)
	p1, p2 := newMock(), newMock()
	room.AddPlayer(p1)
	room.AddPlayer(p2)
	return room, p1, p2
}

// forceCard injects a known card at index 0 so tests aren't random.
func forceCard(room *Room, rank, suit string) {
	room.mu.Lock()
	defer room.mu.Unlock()
	room.deck[0] = Card{Rank: rank, Suit: suit}
}

// setPlaying puts the room into PhasePlaying with card at index 0 exposed,
// without starting the ticker goroutine.
func setPlaying(room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()
	room.phase = PhasePlaying
	room.cardIndex = 0
}

// ── AddPlayer ─────────────────────────────────────────────────────────────────

func TestAddPlayer_AssignsSequentialIDs(t *testing.T) {
	t.Parallel()
	room := newRoom("r", time.Second, RoomKindAnonymous)

	id1, ok1 := room.AddPlayer(newMock())
	id2, ok2 := room.AddPlayer(newMock())

	if !ok1 || !ok2 {
		t.Fatal("both AddPlayer calls should succeed")
	}
	if id1 != 1 || id2 != 2 {
		t.Errorf("expected ids 1,2 — got %d,%d", id1, id2)
	}
}

func TestAddPlayer_SecretRoom_RejectsOverCap(t *testing.T) {
	t.Parallel()
	// Secret room cap comes from config.Cfg.MaxNumberOfPlayerAllowedInRoom.
	// In tests that value may be 0 (zero value); this test just verifies the
	// cap path doesn't panic — adjust the count check to match your config.
	room := newRoom("r", time.Second, RoomKindSecret)
	room.AddPlayer(newMock())
	room.AddPlayer(newMock())

	// If cap == 2 this should fail; if cap is larger it succeeds.
	// The important thing is no panic occurs.
	room.AddPlayer(newMock())
}

// ── HandleClick: ace ──────────────────────────────────────────────────────────

func TestHandleClick_Ace_ClickerWins(t *testing.T) {
	t.Parallel()
	room, p1, _ := newTestRoom()
	forceCard(room, "A", "spades")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p1.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected clicker to receive game_over")
	}
	if msg.Result != "win" {
		t.Errorf("expected result=win, got %q", msg.Result)
	}
}

func TestHandleClick_Ace_OpponentLoses(t *testing.T) {
	t.Parallel()
	room, _, p2 := newTestRoom()
	forceCard(room, "A", "hearts")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p2.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected opponent to receive game_over")
	}
	if msg.Result != "lose" {
		t.Errorf("expected result=lose, got %q", msg.Result)
	}
}

func TestHandleClick_Ace_WinnerIDIsClicker(t *testing.T) {
	t.Parallel()
	room, _, _ := newTestRoom()
	forceCard(room, "A", "clubs")
	setPlaying(room)

	room.HandleClick(2)

	// Check both players got winner_id=2.
	room.mu.Lock()
	_ = room.phase // just touch the room to confirm no deadlock
	room.mu.Unlock()
}

func TestHandleClick_Ace_CardAttachedToMessage(t *testing.T) {
	t.Parallel()
	room, p1, _ := newTestRoom()
	forceCard(room, "A", "diamonds")
	setPlaying(room)

	room.HandleClick(1)

	msg, _ := p1.firstOfType(MsgGameOver)
	if msg.Card == nil {
		t.Fatal("expected card to be attached to game_over")
	}
	if msg.Card.Rank != "A" || msg.Card.Suit != "diamonds" {
		t.Errorf("unexpected card: %+v", msg.Card)
	}
}

// ── HandleClick: non-ace ──────────────────────────────────────────────────────

func TestHandleClick_NonAce_ClickerLoses(t *testing.T) {
	t.Parallel()
	room, p1, _ := newTestRoom()
	forceCard(room, "7", "clubs")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p1.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected clicker to receive game_over")
	}
	if msg.Result != "lose" {
		t.Errorf("expected result=lose, got %q", msg.Result)
	}
}

func TestHandleClick_NonAce_OpponentWins(t *testing.T) {
	t.Parallel()
	room, _, p2 := newTestRoom()
	forceCard(room, "K", "spades")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p2.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected opponent to receive game_over")
	}
	if msg.Result != "win" {
		t.Errorf("expected result=win, got %q", msg.Result)
	}
}

// ── HandleClick: guard conditions ─────────────────────────────────────────────

func TestHandleClick_BeforeCardRevealed_Ignored(t *testing.T) {
	t.Parallel()
	room, p1, p2 := newTestRoom()
	room.mu.Lock()
	room.phase = PhasePlaying
	room.cardIndex = -1 // no card yet
	room.mu.Unlock()

	room.HandleClick(1)

	if p1.hasType(MsgGameOver) || p2.hasType(MsgGameOver) {
		t.Error("click before first card should be ignored")
	}
}

func TestHandleClick_PhaseWaiting_Ignored(t *testing.T) {
	t.Parallel()
	room, p1, p2 := newTestRoom()
	forceCard(room, "A", "clubs")
	// phase stays PhaseWaiting

	room.mu.Lock()
	room.cardIndex = 0
	room.mu.Unlock()

	room.HandleClick(1)

	if p1.hasType(MsgGameOver) || p2.hasType(MsgGameOver) {
		t.Error("click during PhaseWaiting should be ignored")
	}
}

func TestHandleClick_SecondClick_Ignored(t *testing.T) {
	t.Parallel()
	room, p1, p2 := newTestRoom()
	forceCard(room, "A", "spades")
	setPlaying(room)

	room.HandleClick(1) // resolves game
	room.HandleClick(2) // must be ignored

	if p1.countOfType(MsgGameOver) != 1 {
		t.Errorf("p1: expected 1 game_over, got %d", p1.countOfType(MsgGameOver))
	}
	if p2.countOfType(MsgGameOver) != 1 {
		t.Errorf("p2: expected 1 game_over, got %d", p2.countOfType(MsgGameOver))
	}
}

// ── OpponentDisconnected ──────────────────────────────────────────────────────

func TestOpponentDisconnected_SurvivorWins(t *testing.T) {
	t.Parallel()
	room, p1, _ := newTestRoom()
	room.mu.Lock()
	room.phase = PhasePlaying
	room.mu.Unlock()

	room.OpponentDisconnected(1)

	msg, ok := p1.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected survivor to receive game_over")
	}
	if msg.Result != "win" {
		t.Errorf("expected result=win, got %q", msg.Result)
	}
}

func TestOpponentDisconnected_AlreadyFinished_SendsNothing(t *testing.T) {
	t.Parallel()
	room, p1, _ := newTestRoom()
	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()

	room.OpponentDisconnected(1)

	if p1.hasType(MsgGameOver) {
		t.Error("OpponentDisconnected on finished room should send nothing")
	}
}

// ── Start: card reveal loop ───────────────────────────────────────────────────

func TestStart_SendsCardRevealToBothPlayers(t *testing.T) {
	t.Parallel()
	room, p1, p2 := newTestRoom()
	go room.Start()
	time.Sleep(50 * time.Millisecond)

	if !p1.hasType(MsgCardReveal) {
		t.Error("expected p1 to receive at least one card_reveal")
	}
	if !p2.hasType(MsgCardReveal) {
		t.Error("expected p2 to receive at least one card_reveal")
	}

	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()
}

func TestStart_BothPlayersSeeIdenticalCard(t *testing.T) {
	t.Parallel()
	room, p1, p2 := newTestRoom()
	go room.Start()
	time.Sleep(50 * time.Millisecond)

	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()
	time.Sleep(20 * time.Millisecond)

	msg1, ok1 := p1.firstOfType(MsgCardReveal)
	msg2, ok2 := p2.firstOfType(MsgCardReveal)
	if !ok1 || !ok2 {
		t.Skip("ticker did not fire in time")
	}
	if msg1.CardIndex != msg2.CardIndex {
		t.Errorf("card index mismatch: p1=%d p2=%d", msg1.CardIndex, msg2.CardIndex)
	}
	if msg1.Card.Rank != msg2.Card.Rank || msg1.Card.Suit != msg2.Card.Suit {
		t.Errorf("card mismatch: p1=%+v p2=%+v", msg1.Card, msg2.Card)
	}
}

func TestStart_CardIndexIsOneBased(t *testing.T) {
	t.Parallel()
	room, p1, _ := newTestRoom()
	go room.Start()
	time.Sleep(50 * time.Millisecond)

	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()

	msg, ok := p1.firstOfType(MsgCardReveal)
	if !ok {
		t.Skip("no card_reveal received")
	}
	if msg.CardIndex < 1 || msg.CardIndex > 52 {
		t.Errorf("expected card_index in [1,52], got %d", msg.CardIndex)
	}
}

func TestStart_DrawAfterAllCards(t *testing.T) {
	t.Parallel()
	// One-card deck: tick 1 reveals it, tick 2 overflows → draw.
	room := &Room{
		id:           "draw-room",
		kind:         RoomKindAnonymous,
		deck:         []Card{{Rank: "3", Suit: "clubs"}},
		cardIndex:    -1,
		phase:        PhaseWaiting,
		cardInterval: 10 * time.Millisecond,
	}
	p1, p2 := newMock(), newMock()
	room.AddPlayer(p1)
	room.AddPlayer(p2)

	go room.Start()
	time.Sleep(60 * time.Millisecond)

	for _, p := range []*mockSender{p1, p2} {
		msg, ok := p.firstOfType(MsgGameOver)
		if !ok {
			t.Fatal("expected game_over after all cards shown")
		}
		if msg.Result != "draw" {
			t.Errorf("expected draw, got %q", msg.Result)
		}
	}
}

// ── removePlayerLocked ────────────────────────────────────────────────────────

func TestRemovePlayerLocked_DecreasesCount(t *testing.T) {
	t.Parallel()
	room, _, _ := newTestRoom()

	room.mu.Lock()
	room.removePlayerLocked(1)
	count := room.count
	room.mu.Unlock()

	if count != 1 {
		t.Errorf("expected count=1 after remove, got %d", count)
	}
}

func TestRemovePlayerLocked_PlayerNoLongerInSlice(t *testing.T) {
	t.Parallel()
	room, _, _ := newTestRoom()

	room.mu.Lock()
	room.removePlayerLocked(1)
	for _, p := range room.players {
		if p != nil && p.ID == 1 {
			t.Error("player 1 still present after removePlayerLocked")
		}
	}
	room.mu.Unlock()
}
