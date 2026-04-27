package game

// Inbound message types (client → server).
const (
	MsgClick = "click"

	MsgRoomCreate    = "room_create"
	MsgRoomJoin      = "room_join"
	MsgRoomStart     = "room_start"
	MsgRoomAnonymous = "room_anonymous"
)

// Outbound message types (server → client).
const (
	MsgWaiting     = "waiting"
	MsgGameStart   = "game_start"
	MsgCardReveal  = "card_reveal"
	MsgGameOver    = "game_over"
	MsgRateLimited = "rate_limited" // sent before closing a rejected connection

	MsgRoomCreated  = "room_created"
	MsgRoomJoined   = "room_joined"
	MsgRoomUpdated  = "room_updated"
	MsgRoomInvalid  = "room_invalid"
	MsgRoomStarted  = "room_started"
	MsgRoomHostLeft = "room_host_left"
)

// InMessage is the envelope for every client → server message.
type InMessage struct {
	Type string `json:"type"`

	// room_join
	Secret string `json:"secret,omitempty"`
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

	// room_created
	// secret: string; player_id: number
	Secret string `json:"secret,omitempty"`

	// room_joined
	// secret: string; player_id: number; players: RoomPlayer[]

	//room_updated
	Players []RoomPlayer `json:"players"`

	// room_started
}

type RoomPlayer struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	IsHost bool   `json:"isHost"`
}

/*
// ── Server → client (OutMessage) ─────────────────────────────────────────────
export type ServerEvent =
  | { type: 'rate_limited';   message: string }
  | { type: 'waiting';        message: string }
  | { type: 'game_start';     player_id: number }
  | { type: 'card_reveal';    card: Card; card_index: number }
  | { type: 'game_over';      result: 'win' | 'lose' | 'draw'; reason: string; winner_id: number }
  // friends room events
  | { type: 'room_created';   secret: string; player_id: number }
  | { type: 'room_joined';    secret: string; player_id: number; players: RoomPlayer[] }
  | { type: 'room_updated';   players: RoomPlayer[] }   // someone joined or left
  | { type: 'room_invalid';   message: string }         // bad secret or room not in waiting state
  | { type: 'room_started';   }                         // host clicked start → transition to game

// Client → server: { type: "click" } | { type: "room_start" }
*/
