package game

import (
	"sync"
	"testing"
	"time"

	"github.com/shn27/reflex-card-game-backend/internal/logger"
)

// newTestMatchmaker returns a matchmaker with a short card interval so tests
// that trigger room.Start() don't sit through 2-second ticks.
func newTestMatchmaker() *Matchmaker {
	return NewMatchmaker(10 * time.Millisecond)
}

// pair calls Join for two players in sequence — p1 first (parks), then p2
// (triggers pairing). Because Join is non-blocking, no goroutines are needed.
func pair(mm *Matchmaker, p1, p2 *mockSender) {
	mm.Join(p1, p1.setRoom)
	mm.Join(p2, p2.setRoom)
}

// ─── Join: first player ───────────────────────────────────────────────────────

func TestJoin_FirstPlayer_ReceivesWaitingMessage(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if !p1.hasType(MsgWaiting) {
		t.Error("expected player 1 to receive a 'waiting' message while parked")
	}
}

func TestJoin_FirstPlayer_SetRoomCalledWithPlayerID1(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if p1.getPlayerID() != 1 {
		t.Errorf("expected player 1 to get playerID=1 via setRoom, got %d", p1.getPlayerID())
	}
}

func TestJoin_FirstPlayer_RoomIDAssignedImmediately(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if p1.getRoomID() == "" {
		t.Error("expected setRoom to be called with a non-empty roomID for the waiting player")
	}
}

func TestJoin_FirstPlayer_DoesNotReceiveGameStart(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if p1.hasType(MsgGameStart) {
		t.Error("player 1 should not receive game_start before an opponent joins")
	}
}

// ─── Join: pairing ────────────────────────────────────────────────────────────

func TestJoin_Pairing_BothReceiveGameStart(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if !p1.hasType(MsgGameStart) {
		t.Error("expected player 1 to receive game_start after pairing")
	}
	if !p2.hasType(MsgGameStart) {
		t.Error("expected player 2 to receive game_start after pairing")
	}
}

func TestJoin_Pairing_PlayerIDsAreDistinct(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if p1.getPlayerID() == p2.getPlayerID() {
		t.Errorf("both players got the same playerID: %d", p1.getPlayerID())
	}
}

func TestJoin_Pairing_PlayerIDsAre1And2(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if p1.getPlayerID() != 1 {
		t.Errorf("expected p1 playerID=1, got %d", p1.getPlayerID())
	}
	if p2.getPlayerID() != 2 {
		t.Errorf("expected p2 playerID=2, got %d", p2.getPlayerID())
	}
}

func TestJoin_Pairing_BothPlayersShareSameRoomID(t *testing.T) {
	t.Parallel()
	logger.InitLogger()

	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if p1.getRoomID() == "" {
		t.Fatal("player 1 roomID is empty after pairing")
	}
	if p1.getRoomID() != p2.getRoomID() {
		t.Errorf("players in different rooms: p1=%q p2=%q", p1.getRoomID(), p2.getRoomID())
	}
}

func TestJoin_Pairing_GameStartCarriesCorrectPlayerID(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	msg1, _ := p1.firstOfType(MsgGameStart)
	if msg1.PlayerID != 1 {
		t.Errorf("game_start to p1 should carry player_id=1, got %d", msg1.PlayerID)
	}

	msg2, _ := p2.firstOfType(MsgGameStart)
	if msg2.PlayerID != 2 {
		t.Errorf("game_start to p2 should carry player_id=2, got %d", msg2.PlayerID)
	}
}

func TestJoin_Pairing_RoomStoredInMatchmaker(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	_, ok := mm.GetRoom(p1.getRoomID())
	if !ok {
		t.Errorf("room %q not found in matchmaker after pairing", p1.getRoomID())
	}
}

func TestJoin_Pairing_WaitingSlotClearedAfterPairing(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	// A third player should park in the now-empty waiting slot.
	p3 := newMock()
	mm.Join(p3, p3.setRoom)

	if !p3.hasType(MsgWaiting) {
		t.Error("expected third player to park after first pair cleared the slot")
	}
	if p3.hasType(MsgGameStart) {
		t.Error("third player should not get game_start — no fourth player yet")
	}
}

func TestJoin_SequentialPairs_RoomIDsAreUnique(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	seen := make(map[string]bool)

	for range 5 {
		p1, p2 := newMock(), newMock()
		pair(mm, p1, p2)

		id := p1.getRoomID()
		if seen[id] {
			t.Errorf("duplicate room ID across pairs: %q", id)
		}
		seen[id] = true
	}
}

// ─── Leave ────────────────────────────────────────────────────────────────────

func TestLeave_WaitingPlayer_ClearsSlot(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	p1 := newMock()
	mm.Join(p1, p1.setRoom)

	// p1 disconnects before anyone joins.
	mm.Leave(p1, p1.getRoomID(), p1.getPlayerID())

	// A new player should now park cleanly in the waiting slot.
	p2 := newMock()
	mm.Join(p2, p2.setRoom)

	if !p2.hasType(MsgWaiting) {
		t.Error("expected new player to park after the disconnected waiting player's slot was cleared")
	}
}

func TestLeave_InGame_OpponentReceivesWin(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	mm.Leave(p1, p1.getRoomID(), p1.getPlayerID())

	msg, ok := p2.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected opponent to receive game_over after disconnect")
	}
	if msg.Result != "win" {
		t.Errorf("expected opponent result=win, got %q", msg.Result)
	}
}

func TestLeave_InGame_RoomRemovedFromMatchmaker(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	roomID := p1.getRoomID()
	mm.Leave(p1, roomID, p1.getPlayerID())

	_, ok := mm.GetRoom(roomID)
	if ok {
		t.Error("expected room to be deleted from matchmaker after disconnect")
	}
}

func TestLeave_EmptyRoomID_DoesNotPanic(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	// Calling Leave with no room context (e.g. player disconnects before pairing
	// is handled) must not panic.
	mm.Leave(newMock(), "", 0)
}

func TestLeave_UnknownRoomID_DoesNotPanic(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	mm.Leave(newMock(), "room-does-not-exist", 1)
}

// ─── GetRoom ─────────────────────────────────────────────────────────────────

func TestGetRoom_UnknownID_ReturnsFalse(t *testing.T) {
	t.Parallel()

	logger.InitLogger()
	mm := newTestMatchmaker()
	_, ok := mm.GetRoom("ghost")
	if ok {
		t.Error("GetRoom should return false for an unknown room ID")
	}
}

// ─── Concurrency ─────────────────────────────────────────────────────────────

func TestJoin_ConcurrentPairs_NoRaceCondition(t *testing.T) {
	// Run with: go test -race ./internal/game/...
	t.Parallel()

	logger.InitLogger()
	const pairs = 20
	mm := newTestMatchmaker()

	var wg sync.WaitGroup
	wg.Add(pairs * 2)

	for range pairs {
		go func() {
			defer wg.Done()
			p := newMock()
			mm.Join(p, p.setRoom)
		}()
		go func() {
			defer wg.Done()
			p := newMock()
			mm.Join(p, p.setRoom)
		}()
	}

	wg.Wait()
}
