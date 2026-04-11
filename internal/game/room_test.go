package game

import (
	"testing"
	"time"

	"github.com/shn27/reflex-card-game-backend/internal/logger"
)

// newTestRoom builds a room with two mock players added, ready for Start().
func newTestRoom() (*Room, *mockSender, *mockSender) {
	room := NewRoom("test-room", 10*time.Millisecond)
	p1, p2 := newMock(), newMock()
	room.AddPlayer(p1)
	room.AddPlayer(p2)
	return room, p1, p2
}

// forceCard injects a specific card at deck position 0 so tests are not at
// the mercy of the random shuffle.
func forceCard(room *Room, rank, suit string) {
	room.mu.Lock()
	defer room.mu.Unlock()
	room.deck[0] = Card{Rank: rank, Suit: suit}
}

// setPlaying puts the room into PhasePlaying with the card at index 0 exposed,
// avoiding the need to start the ticker goroutine for click-only tests.
func setPlaying(room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()
	room.phase = PhasePlaying
	room.cardIndex = 0
}

// ─── AddPlayer ───────────────────────────────────────────────────────────────

func TestRoom_AddPlayer_AssignsSequentialIDs(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room := NewRoom("r", time.Second)
	id1, ok1 := room.AddPlayer(newMock())
	id2, ok2 := room.AddPlayer(newMock())

	if !ok1 || !ok2 {
		t.Fatal("expected both AddPlayer calls to succeed")
	}
	if id1 != 1 {
		t.Errorf("expected id1=1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("expected id2=2, got %d", id2)
	}
}

func TestRoom_AddPlayer_RejectsThirdPlayer(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room := NewRoom("r", time.Second)
	room.AddPlayer(newMock())
	room.AddPlayer(newMock())

	_, ok := room.AddPlayer(newMock())
	if ok {
		t.Error("expected third AddPlayer to fail")
	}
}

func TestRoom_IsFull_FalseWhenEmpty(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room := NewRoom("r", time.Second)
	if room.IsFull() {
		t.Error("new room should not be full")
	}
}

func TestRoom_IsFull_FalseWithOnePlayer(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room := NewRoom("r", time.Second)
	room.AddPlayer(newMock())
	if room.IsFull() {
		t.Error("room with one player should not be full")
	}
}

func TestRoom_IsFull_TrueWithTwoPlayers(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room := NewRoom("r", time.Second)
	room.AddPlayer(newMock())
	room.AddPlayer(newMock())
	if !room.IsFull() {
		t.Error("room with two players should be full")
	}
}

// ─── HandleClick: ace ────────────────────────────────────────────────────────

func TestHandleClick_Ace_ClickerWins(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, _ := newTestRoom()
	forceCard(room, "A", "spades")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p1.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected clicker to receive game_over")
	}
	if msg.Result != "win" {
		t.Errorf("clicker expected result=win, got %q", msg.Result)
	}
}

func TestHandleClick_Ace_OpponentLoses(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, _, p2 := newTestRoom()
	forceCard(room, "A", "hearts")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p2.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected opponent to receive game_over")
	}
	if msg.Result != "lose" {
		t.Errorf("opponent expected result=lose, got %q", msg.Result)
	}
}

func TestHandleClick_Ace_WinnerIDMatchesClicker(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, _, p2 := newTestRoom()
	forceCard(room, "A", "clubs")
	setPlaying(room)

	room.HandleClick(2) // player 2 clicks the ace

	msg, _ := p2.firstOfType(MsgGameOver)
	if msg.WinnerID != 2 {
		t.Errorf("expected winner_id=2, got %d", msg.WinnerID)
	}
}

func TestHandleClick_Ace_CardAttachedToMessage(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, _ := newTestRoom()
	forceCard(room, "A", "diamonds")
	setPlaying(room)

	room.HandleClick(1)

	msg, _ := p1.firstOfType(MsgGameOver)
	if msg.Card == nil {
		t.Fatal("expected card to be attached to game_over")
	}
	if msg.Card.Rank != "A" || msg.Card.Suit != "diamonds" {
		t.Errorf("unexpected card in game_over: %+v", msg.Card)
	}
}

// ─── HandleClick: non-ace ────────────────────────────────────────────────────

func TestHandleClick_NonAce_ClickerLoses(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, _ := newTestRoom()
	forceCard(room, "7", "clubs")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p1.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected clicker to receive game_over")
	}
	if msg.Result != "lose" {
		t.Errorf("false-clicker expected result=lose, got %q", msg.Result)
	}
}

func TestHandleClick_NonAce_OpponentWins(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, _, p2 := newTestRoom()
	forceCard(room, "K", "spades")
	setPlaying(room)

	room.HandleClick(1)

	msg, ok := p2.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected opponent to receive game_over")
	}
	if msg.Result != "win" {
		t.Errorf("opponent expected result=win, got %q", msg.Result)
	}
}

func TestHandleClick_NonAce_WinnerIDIsOpponent(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, _, p2 := newTestRoom()
	forceCard(room, "5", "hearts")
	setPlaying(room)

	room.HandleClick(1) // p1 false-clicks — p2 should be winner

	msg, _ := p2.firstOfType(MsgGameOver)
	if msg.WinnerID != 2 {
		t.Errorf("expected winner_id=2 (opponent), got %d", msg.WinnerID)
	}
}

// ─── HandleClick: guard conditions ──────────────────────────────────────────

func TestHandleClick_BeforeCardRevealed_Ignored(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, p2 := newTestRoom()

	room.mu.Lock()
	room.phase = PhasePlaying
	room.cardIndex = -1 // no card yet
	room.mu.Unlock()

	room.HandleClick(1)

	if p1.hasType(MsgGameOver) || p2.hasType(MsgGameOver) {
		t.Error("click before any card is revealed should be ignored")
	}
}

func TestHandleClick_WhenPhaseIsWaiting_Ignored(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, p2 := newTestRoom()
	forceCard(room, "A", "clubs")
	// Leave phase as PhaseWaiting (default) — game hasn't started.

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

	logger.InitLogger()
	room, p1, p2 := newTestRoom()
	forceCard(room, "A", "spades")
	setPlaying(room)

	room.HandleClick(1) // resolves the game
	room.HandleClick(2) // must be ignored — game is finished

	if p1.countOfType(MsgGameOver) != 1 {
		t.Errorf("player 1 received %d game_over messages, want exactly 1", p1.countOfType(MsgGameOver))
	}
	if p2.countOfType(MsgGameOver) != 1 {
		t.Errorf("player 2 received %d game_over messages, want exactly 1", p2.countOfType(MsgGameOver))
	}
}

// ─── OpponentDisconnected ─────────────────────────────────────────────────────

func TestOpponentDisconnected_SurvivorReceivesWin(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
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
		t.Errorf("expected survivor result=win, got %q", msg.Result)
	}
}

func TestOpponentDisconnected_WhenAlreadyFinished_SendsNothing(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, _ := newTestRoom()

	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()

	room.OpponentDisconnected(1)

	if p1.hasType(MsgGameOver) {
		t.Error("OpponentDisconnected on an already-finished room should send nothing")
	}
}

// ─── Start (card reveal loop) ─────────────────────────────────────────────────

func TestStart_SendsCardRevealToBothPlayers(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, p2 := newTestRoom()
	go room.Start()

	time.Sleep(50 * time.Millisecond) // allow at least one tick

	if !p1.hasType(MsgCardReveal) {
		t.Error("expected player 1 to receive at least one card_reveal")
	}
	if !p2.hasType(MsgCardReveal) {
		t.Error("expected player 2 to receive at least one card_reveal")
	}

	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()
}

func TestStart_BothPlayersSeeTheSameCard(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	room, p1, p2 := newTestRoom()
	go room.Start()

	time.Sleep(50 * time.Millisecond)

	room.mu.Lock()
	room.phase = PhaseFinished
	room.mu.Unlock()

	time.Sleep(20 * time.Millisecond) // let ticker goroutine exit

	msg1, ok1 := p1.firstOfType(MsgCardReveal)
	msg2, ok2 := p2.firstOfType(MsgCardReveal)
	if !ok1 || !ok2 {
		t.Skip("ticker did not fire in time — skipping card equality check")
	}

	if msg1.CardIndex != msg2.CardIndex {
		t.Errorf("players saw different card indices: p1=%d p2=%d", msg1.CardIndex, msg2.CardIndex)
	}
	if msg1.Card.Rank != msg2.Card.Rank || msg1.Card.Suit != msg2.Card.Suit {
		t.Errorf("players saw different cards: p1=%+v p2=%+v", msg1.Card, msg2.Card)
	}
}

func TestStart_Draw_WhenAllCardsExhausted(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	// One-card deck: tick 1 reveals it, tick 2 overflows → draw.
	room := &Room{
		id:           "draw-room",
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

	for _, player := range []*mockSender{p1, p2} {
		msg, ok := player.firstOfType(MsgGameOver)
		if !ok {
			t.Fatal("expected game_over after all cards shown")
		}
		if msg.Result != "draw" {
			t.Errorf("expected result=draw, got %q", msg.Result)
		}
	}
}

func TestStart_CardIndexIsOneBased(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
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
