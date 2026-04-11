package game

import "sync"

// mockSender captures every OutMessage sent to it and mirrors what ws.Client
// does in production — storing roomID and playerID via the setRoom callback.
// All methods are goroutine-safe.
type mockSender struct {
	mu       sync.Mutex
	received []OutMessage

	roomID   string
	playerID int
}

func newMock() *mockSender { return &mockSender{} }

// Send implements game.Sender.
func (m *mockSender) Send(msg OutMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received = append(m.received, msg)
}

// setRoom is the callback signature expected by Matchmaker.Join.
// It mirrors exactly what ws.Client.SetRoom does in production.
func (m *mockSender) setRoom(roomID string, playerID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roomID = roomID
	m.playerID = playerID
}

func (m *mockSender) getRoomID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.roomID
}

func (m *mockSender) getPlayerID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.playerID
}

// messages returns a safe copy of all received messages.
func (m *mockSender) messages() []OutMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]OutMessage, len(m.received))
	copy(out, m.received)
	return out
}

// firstOfType returns the first message with the given type.
func (m *mockSender) firstOfType(msgType string) (OutMessage, bool) {
	for _, msg := range m.messages() {
		if msg.Type == msgType {
			return msg, true
		}
	}
	return OutMessage{}, false
}

// hasType returns true if at least one message of that type was received.
func (m *mockSender) hasType(msgType string) bool {
	_, ok := m.firstOfType(msgType)
	return ok
}

// countOfType returns how many messages of that type were received.
func (m *mockSender) countOfType(msgType string) int {
	n := 0
	for _, msg := range m.messages() {
		if msg.Type == msgType {
			n++
		}
	}
	return n
}
