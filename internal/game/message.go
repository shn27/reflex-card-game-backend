package game

// Inbound message types (client → server).
const (
	MsgClick = "click"
)

// Outbound message types (server → client).
const (
	MsgWaiting     = "waiting"
	MsgGameStart   = "game_start"
	MsgCardReveal  = "card_reveal"
	MsgGameOver    = "game_over"
	MsgRateLimited = "rate_limited" // sent before closing a rejected connection
)

// InMessage is the envelope for every client → server message.
type InMessage struct {
	Type string `json:"type"`
}

// OutMessage is the envelope for every server → client message.
type OutMessage struct {
	Type string `json:"type"`

	// game_start
	PlayerID int `json:"player_id,omitempty"`

	// card_reveal
	Card      *Card `json:"card,omitempty"`
	CardIndex int   `json:"card_index,omitempty"` // 1-based, out of 52

	// game_over
	Result   string `json:"result,omitempty"` // "win" | "lose" | "draw"
	Reason   string `json:"reason,omitempty"`
	WinnerID int    `json:"winner_id,omitempty"` // 0 = draw

	// waiting / error
	Message string `json:"message,omitempty"`
}
