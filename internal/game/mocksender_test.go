package game

import (
	"sync"
)

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

func (m *mockSender) Send(msg OutMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received = append(m.received, msg)
}

// setRoom matches the callback signature expected by Matchmaker.Join /
// CreateRoom / JoinRoom — mirrors ws.Client.SetRoom exactly.
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

func (m *mockSender) messages() []OutMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]OutMessage, len(m.received))
	copy(out, m.received)
	return out
}

func (m *mockSender) firstOfType(msgType string) (OutMessage, bool) {
	for _, msg := range m.messages() {
		if msg.Type == msgType {
			return msg, true
		}
	}
	return OutMessage{}, false
}

func (m *mockSender) hasType(msgType string) bool {
	_, ok := m.firstOfType(msgType)
	return ok
}

func (m *mockSender) countOfType(msgType string) int {
	n := 0
	for _, msg := range m.messages() {
		if msg.Type == msgType {
			n++
		}
	}
	return n
}
