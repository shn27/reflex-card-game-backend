package game

import (
	"sync"
	"testing"
	"time"
)

func newTestMatchmaker() *Matchmaker {
	return NewMatchmaker(10 * time.Millisecond)
}

// pair joins two anonymous players and waits until both have a roomID.
func pair(mm *Matchmaker, p1, p2 *mockSender) {
	mm.Join(p1, p1.setRoom)
	mm.Join(p2, p2.setRoom)
}

// ── Anonymous Join ────────────────────────────────────────────────────────────

func TestJoin_FirstPlayer_ReceivesWaiting(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if !p1.hasType(MsgWaiting) {
		t.Error("expected player 1 to receive waiting message")
	}
}

func TestJoin_FirstPlayer_AssignedPlayerID1(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if p1.getPlayerID() != 1 {
		t.Errorf("expected playerID=1, got %d", p1.getPlayerID())
	}
}

func TestJoin_FirstPlayer_RoomIDSet(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1 := newMock()

	mm.Join(p1, p1.setRoom)

	if p1.getRoomID() == "" {
		t.Error("expected roomID to be set for waiting player")
	}
}

func TestJoin_SecondPlayer_BothReceiveGameStart(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if !p1.hasType(MsgGameStart) {
		t.Error("expected p1 to receive game_start")
	}
	if !p2.hasType(MsgGameStart) {
		t.Error("expected p2 to receive game_start")
	}
}

func TestJoin_SecondPlayer_DistinctPlayerIDs(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if p1.getPlayerID() == p2.getPlayerID() {
		t.Errorf("both players got same playerID: %d", p1.getPlayerID())
	}
}

func TestJoin_SecondPlayer_SameRoomID(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	if p1.getRoomID() != p2.getRoomID() {
		t.Errorf("players in different rooms: p1=%q p2=%q", p1.getRoomID(), p2.getRoomID())
	}
}

func TestJoin_GameStartCarriesCorrectPlayerID(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	msg1, _ := p1.firstOfType(MsgGameStart)
	if msg1.PlayerID != 1 {
		t.Errorf("expected p1 game_start player_id=1, got %d", msg1.PlayerID)
	}
	msg2, _ := p2.firstOfType(MsgGameStart)
	if msg2.PlayerID != 2 {
		t.Errorf("expected p2 game_start player_id=2, got %d", msg2.PlayerID)
	}
}

func TestJoin_WaitingSlotClearedAfterPairing(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	// Third player should park, not pair with anyone.
	p3 := newMock()
	mm.Join(p3, p3.setRoom)
	if !p3.hasType(MsgWaiting) {
		t.Error("expected third player to be placed in waiting slot after first pair")
	}
}

func TestJoin_RoomStoredAfterPairing(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	_, ok := mm.GetRoom(p1.getRoomID())
	if !ok {
		t.Error("expected room to be stored in matchmaker after pairing")
	}
}

func TestJoin_UniqueRoomIDsAcrossMultiplePairs(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	seen := make(map[string]bool)

	for range 5 {
		p1, p2 := newMock(), newMock()
		pair(mm, p1, p2)
		id := p1.getRoomID()
		if seen[id] {
			t.Errorf("duplicate room ID: %q", id)
		}
		seen[id] = true
	}
}

// ── Secret rooms ──────────────────────────────────────────────────────────────

func TestCreateRoom_HostReceivesRoomCreated(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host := newMock()

	mm.CreateRoom(host, host.setRoom)

	if !host.hasType(MsgRoomCreated) {
		t.Error("expected host to receive room_created")
	}
}

func TestCreateRoom_SecretIsNonEmpty(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host := newMock()

	mm.CreateRoom(host, host.setRoom)

	msg, _ := host.firstOfType(MsgRoomCreated)
	if msg.Secret == "" {
		t.Error("expected non-empty secret in room_created")
	}
}

func TestCreateRoom_HostAssignedPlayerID1(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host := newMock()

	mm.CreateRoom(host, host.setRoom)

	if host.getPlayerID() != 1 {
		t.Errorf("expected host playerID=1, got %d", host.getPlayerID())
	}
}

func TestJoinRoom_ValidSecret_JoinerReceivesRoomJoined(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	secret, _ := host.firstOfType(MsgRoomCreated)

	mm.JoinRoom(guest, secret.Secret, guest.setRoom)

	if !guest.hasType(MsgRoomJoined) {
		t.Error("expected guest to receive room_joined")
	}
}

func TestJoinRoom_ValidSecret_HostReceivesRoomUpdated(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	secret, _ := host.firstOfType(MsgRoomCreated)

	mm.JoinRoom(guest, secret.Secret, guest.setRoom)

	if !host.hasType(MsgRoomUpdated) {
		t.Error("expected host to receive room_updated when guest joins")
	}
}

func TestJoinRoom_InvalidSecret_GetsRoomInvalid(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	guest := newMock()

	mm.JoinRoom(guest, "BADCODE", guest.setRoom)

	if !guest.hasType(MsgRoomInvalid) {
		t.Error("expected room_invalid for unknown secret")
	}
}

func TestStartRoom_BothPlayersReceiveRoomStarted(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	created, _ := host.firstOfType(MsgRoomCreated)
	mm.JoinRoom(guest, created.Secret, guest.setRoom)

	mm.StartRoom(created.Secret, host.getPlayerID(), host)

	if !host.hasType(MsgRoomStarted) {
		t.Error("expected host to receive room_started")
	}
	if !guest.hasType(MsgRoomStarted) {
		t.Error("expected guest to receive room_started")
	}
}

func TestStartRoom_NonHostCannotStart(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	created, _ := host.firstOfType(MsgRoomCreated)
	mm.JoinRoom(guest, created.Secret, guest.setRoom)

	mm.StartRoom(created.Secret, guest.getPlayerID(), guest)

	if guest.hasType(MsgRoomStarted) {
		t.Error("non-host should not be able to start the room")
	}
	if !guest.hasType(MsgRoomInvalid) {
		t.Error("expected room_invalid when non-host tries to start")
	}
}

// ── Leave: anonymous ──────────────────────────────────────────────────────────

func TestLeave_WaitingAnonymous_ClearsSlot(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1 := newMock()
	mm.Join(p1, p1.setRoom)

	mm.Leave(p1, p1.getRoomID(), p1.getPlayerID())

	// A new player should now park in the empty slot.
	p2 := newMock()
	mm.Join(p2, p2.setRoom)
	if !p2.hasType(MsgWaiting) {
		t.Error("expected new player to park after waiting player disconnected")
	}
}

func TestLeave_AnonymousPlaying_SurvivorWins(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	p1, p2 := newMock(), newMock()
	pair(mm, p1, p2)

	// Small sleep so room.Start() goroutine changes phase to PhasePlaying.
	time.Sleep(30 * time.Millisecond)

	mm.Leave(p1, p1.getRoomID(), p1.getPlayerID())
	time.Sleep(20 * time.Millisecond)

	msg, ok := p2.firstOfType(MsgGameOver)
	if !ok {
		t.Fatal("expected survivor to receive game_over")
	}
	if msg.Result != "win" {
		t.Errorf("expected result=win, got %q", msg.Result)
	}
}

func TestLeave_UnknownRoomID_DoesNotPanic(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	mm.Leave(newMock(), "ghost-room", 1)
}

func TestLeave_EmptyRoomID_DoesNotPanic(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	mm.Leave(newMock(), "", 0)
}

// ── Leave: secret room waiting ────────────────────────────────────────────────

func TestLeave_SecretWaiting_HostLeaves_GuestNotified(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	created, _ := host.firstOfType(MsgRoomCreated)
	mm.JoinRoom(guest, created.Secret, guest.setRoom)

	mm.Leave(host, host.getRoomID(), host.getPlayerID())

	if !guest.hasType(MsgRoomHostLeft) {
		t.Error("expected guest to receive room_host_left when host disconnects")
	}
}

func TestLeave_SecretWaiting_GuestLeaves_HostGetsUpdate(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	created, _ := host.firstOfType(MsgRoomCreated)
	mm.JoinRoom(guest, created.Secret, guest.setRoom)

	mm.Leave(guest, guest.getRoomID(), guest.getPlayerID())

	if !host.hasType(MsgRoomUpdated) {
		t.Error("expected host to receive room_updated when guest leaves lobby")
	}
}

func TestLeave_SecretWaiting_GuestLeaves_CanRejoin(t *testing.T) {
	t.Parallel()
	mm := newTestMatchmaker()
	host, guest := newMock(), newMock()

	mm.CreateRoom(host, host.setRoom)
	created, _ := host.firstOfType(MsgRoomCreated)
	mm.JoinRoom(guest, created.Secret, guest.setRoom)
	mm.Leave(guest, guest.getRoomID(), guest.getPlayerID())

	// Guest tries to rejoin — must succeed, not panic.
	guest2 := newMock()
	mm.JoinRoom(guest2, created.Secret, guest2.setRoom)

	if !guest2.hasType(MsgRoomJoined) {
		t.Error("expected new guest to join successfully after previous guest left")
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestJoin_ConcurrentPairs_NoRace(t *testing.T) {
	// go test -race ./internal/game/...
	t.Parallel()
	mm := newTestMatchmaker()
	var wg sync.WaitGroup

	for range 20 {
		wg.Add(2)
		go func() { defer wg.Done(); p := newMock(); mm.Join(p, p.setRoom) }()
		go func() { defer wg.Done(); p := newMock(); mm.Join(p, p.setRoom) }()
	}
	wg.Wait()
}
