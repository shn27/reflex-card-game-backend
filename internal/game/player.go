package game

// Sender is satisfied by anything that can receive an outbound message.
// The ws.Client implements this; using an interface keeps the game package
// free of any WebSocket dependency.
type Sender interface {
	Send(msg OutMessage)
}

// Player holds the identity and transport of one participant in a room.
type Player struct {
	ID   int // 1 , 2 ..
	conn Sender
}

func NewPlayer(id int, conn Sender) *Player {
	return &Player{ID: id, conn: conn}
}

//Player → forwards message → Client
func (p *Player) Send(msg OutMessage) {
	p.conn.Send(msg)
}
