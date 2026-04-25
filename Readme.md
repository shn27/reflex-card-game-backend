# reflex-card-game-backend

Real-time WebSocket backend for **Reflex** — a multiplayer card reaction game built in Go.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![WebSocket](https://img.shields.io/badge/Transport-WebSocket-informational)](https://datatracker.ietf.org/doc/html/rfc6455)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](./Dockerfile)

---

## Live App

**[reflex-card-game-frontend.vercel.app](https://reflex-card-game-frontend.vercel.app)**

Frontend on [Vercel](https://vercel.com) · Backend on [Render](https://render.com) · First matchmaking may take a few seconds on the free tier.

---

## Table of Contents

- [Game Rules](#game-rules)
- [How to Run](#how-to-run)
- [Architecture](#architecture)
- [API Documentation](#api-documentation)
- [Tech Stack & Design Decisions](#tech-stack--design-decisions)
- [Configuration](#configuration)
- [Running Tests](#running-tests)

---

## Game Rules

1. Two or more players connect — they are paired automatically or via a secret room code.
2. A shuffled 52-card deck is dealt entirely on the server.
3. Cards are revealed to all players simultaneously, one every `CARD_INTERVAL_MS` milliseconds.
4. **Wait for an Ace.** Clicking any non-Ace card results in an immediate loss.
5. When an Ace appears, the **first player to click wins**.
6. If all 52 cards are revealed with no click, the game ends in a **draw**.
7. If a player disconnects mid-game, the remaining player continues the game. If only 1 player remains then he wins.

---

## How to Run

### Prerequisites

- Go 1.25+
- Docker (optional)

### Clone & Run Locally

```bash
git clone git@github.com:shn27/reflex-card-game-backend.git
cd reflex-card-game-backend

cp .env.example .env
go mod tidy
go run .
```

Server starts on `http://localhost:8080`.

### Run with Docker Compose (backend + frontend together)

```bash
docker compose up -d
```

Visit `http://localhost:3000`.

### Run Backend Only with Docker

```bash
docker build -t reflex-backend .
docker run -p 8080:8080 --env-file .env reflex-backend
```

Multi-stage build — final image is based on `scratch` (~7 MB binary, no OS layer).

### Frontend

See [reflex-card-game-frontend](https://github.com/shn27/reflex-card-game-frontend) for setup, or use the live app above.

---

## Architecture

### Directory Structure

```
reflex-card-game-backend/
├── main.go                      # Binary entry point — calls server.Execute()
├── cmd/
│   └── server/
│       └── root.go              # Cobra root command — init logger, load config, start server
├── internal/
│   ├── config/
│   │   └── config.go            # Env var loading and validation
│   ├── game/
│   │   ├── card.go              # Card type, deck creation, shuffle
│   │   ├── message.go           # All inbound/outbound message types and structs
│   │   ├── player.go            # Player struct + Sender interface
│   │   ├── room.go              # Game loop, click handling, player management
│   │   └── matchmaker.go        # Anonymous queue + secret room lifecycle
│   ├── handlers/
│   │   ├── health.go            # GET /health
│   │   └── ws/
│   │       └── handler.go       # HTTP → WebSocket upgrade, rate limit check
│   ├── logger/
│   │   └── logger.go            # Zap logger initialisation
│   ├── ratelimit/
│   │   └── limiter.go           # Per-IP and global WS connection caps
│   └── routes/
│       └── routes.go            # Mux registration, server start
├── .env.example
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

### Package dependency direction

```
main.go → cmd/server (Cobra) → routes → handlers/ws → game
                                                     → ratelimit
                              → config
                              → logger
          game (no ws import — decoupled via Sender interface)
```

`game` never imports `ws`. Communication goes through the `Sender` interface, keeping game logic independently testable.

### Connection Lifecycle

```
Browser opens WebSocket
        │
        ▼
ws.Handler: upgrade HTTP → WS, rate limit check
        │
        ├── go client.WritePump()        drains outgoing channel → wire
        │
        ▼
client.ReadPump() begins reading frames
        │
        ▼  first message from client determines mode:
        │
        ├── room_anonymous → mm.Join()   global queue, blocks until paired
        ├── room_create    → mm.CreateRoom()   new secret room
        ├── room_join      → mm.JoinRoom()     join by 6-char code
        │
        ▼  once paired / game started:
        │
        ├── room_start     → mm.StartRoom()    host starts secret room
        └── click          → room.HandleClick()
        │
        ▼
client disconnects → mm.Leave() → phase/kind matrix → clean up
```

### Leave Decision Matrix

```
Phase    │ Kind      │ Who left │ Action
─────────┼───────────┼──────────┼──────────────────────────────────────────
Waiting  │ Anonymous │ p1       │ clear global waiting slot
Waiting  │ Secret    │ host     │ MsgRoomHostLeft to all guests, delete room
Waiting  │ Secret    │ guest    │ remove from lobby, MsgRoomUpdated to others
Playing  │ Anonymous │ anyone   │ remaining player wins (MsgGameOver)
Playing  │ Secret    │ last one │ remaining player wins (MsgGameOver)
Playing  │ Secret    │ not last │ silent remove, game continues
```

### Locking Convention

Methods named with the `Locked` suffix (`removePlayerLocked`, `broadcastLocked`, `sendToLocked`, etc.) must be called with `room.mu` already held. Methods without the suffix acquire their own lock. This naming contract prevents deadlocks at call sites.

---

## API Documentation

### Endpoints

```
WS   ws://localhost:8080/ws
GET  http://localhost:8080/health   → 200 OK
```

### Client → Server Messages

| `type` | Fields | When |
|---|---|---|
| `room_anonymous` | — | Sent immediately after opening WS to join global queue |
| `room_create` | — | Create a new secret room (sender becomes host) |
| `room_join` | `secret` | Join an existing secret room by 6-char code |
| `room_start` | `secret` | Host starts the game (only player ID 1 may call this) |
| `click` | — | Player clicks the card button during a game |

All setup messages (`room_*`) must be the **first message** sent after the connection opens. Sending them after a game is in progress is silently ignored.

### Server → Client Messages

#### `waiting`
Player 1 is in the anonymous queue, waiting for an opponent.
```json
{ "type": "waiting", "message": "Waiting for an opponent…" }
```

#### `rate_limited`
Connection rejected due to per-IP or global cap. Server closes with code `1008` immediately after.
```json
{ "type": "rate_limited", "message": "too many connections from your IP, please try again later" }
```

#### `room_created`
Sent to the host after `room_create`.
```json
{ "type": "room_created", "player_id": 1, "secret": "XK92F3" }
```

#### `room_joined`
Sent to a player after successfully joining a secret room.
```json
{
  "type": "room_joined",
  "player_id": 2,
  "secret": "XK92F3",
  "players": [
    { "id": 1, "name": "Player-1", "is_host": true },
    { "id": 2, "name": "Player-2", "is_host": false }
  ]
}
```

#### `room_updated`
Broadcast to all existing players when the lobby changes (someone joins or leaves).
```json
{
  "type": "room_updated",
  "secret": "XK92F3",
  "players": [ { "id": 1, "name": "Player-1", "is_host": true } ]
}
```

#### `room_started`
Broadcast to all players when the host starts the game.
```json
{ "type": "room_started" }
```

#### `room_host_left`
Sent to all guests when the host disconnects during the lobby phase.
```json
{ "type": "room_host_left" }
```

#### `room_invalid`
Sent when a secret or permission check fails.
```json
{ "type": "room_invalid", "message": "Room not found" }
```

#### `game_start`
Sent to both players once a match begins (anonymous or secret).
```json
{ "type": "game_start", "player_id": 1 }
```

#### `card_reveal`
Sent every `CARD_INTERVAL_MS` ms. Both players always receive the same card.
```json
{
  "type": "card_reveal",
  "card": { "rank": "A", "suit": "hearts" },
  "card_index": 14
}
```

| Field | Type | Values |
|---|---|---|
| `card.rank` | string | `A`, `2`–`10`, `J`, `Q`, `K` |
| `card.suit` | string | `hearts`, `diamonds`, `clubs`, `spades` |
| `card_index` | int | 1-based, `1`–`52` |

#### `game_over`
Sent to all players when the game ends. `result` is from the **receiving player's perspective**.
```json
{
  "type": "game_over",
  "result": "win",
  "reason": "You clicked the Ace first!",
  "winner_id": 1,
  "card": { "rank": "A", "suit": "hearts" }
}
```

| Scenario | Clicker | Others |
|---|---|---|
| Clicked an Ace first | `win` | `lose` |
| Clicked a non-Ace | `lose` | `win` |
| Opponent disconnected | `win` | — |
| All 52 cards, no clicks | `draw` | `draw` |

---

## Tech Stack & Design Decisions

### CLI — Cobra

`main.go` at the repo root is the binary entry point — a single call to `server.Execute()`. The actual startup logic lives in `cmd/server/root.go` as a Cobra `rootCmd`. This structure means:

- Adding a second subcommand (e.g. `migrate`, `seed`, `worker`) requires only a new file under `cmd/server/` — `main.go` never changes.
- The `Run` func is the single place where initialisation order is explicit: logger first, `.env` second, config third, server last. If logger reads env vars during init, load `.env` before `InitLogger()`.

### Go

Goroutines map directly onto the problem — one read goroutine and one write goroutine per connection, coordinated through channels and mutexes. No framework needed; `net/http`, `sync`, `time`, and `encoding/json` cover everything.

### WebSocket — gorilla/websocket

The established standard for WebSocket in Go. Handles frame parsing, ping/pong keepalives, and enforces the single-writer constraint. The read/write pump pattern comes from its own documentation.

### HTTP Router — `net/http` stdlib

Two routes (`/ws`, `/health`). Gin or Chi would add a dependency with no functional gain.

### Logger — Zap

Zero-allocation structured logging. Every game event (card reveal, click, disconnect) emits structured fields (`room_id`, `player_id`, `rank`, `suit`) making logs machine-parseable for aggregators without post-processing.

### Rate Limiter — concurrency cap, not token bucket

Two independent counters: a global cap (`MAX_NUM_OF_WS_ALLOWED_INTOTAL`) using `atomic.Int64`, and a per-IP cap (`MAX_NUM_WS_PER_IP`) behind a `sync.Mutex`. Limits are checked **after** the WebSocket upgrade so the server can send a readable `rate_limited` message instead of an opaque HTTP 429 that browsers cannot surface to application code.

### `Sender` Interface

```go
type Sender interface {
    Send(msg OutMessage)
}
```

`ws.Client` implements it; `game` never imports `ws`. Game logic is testable with a mock `Sender` — no real WebSocket connection required.

### Two separate maps for rooms and secrets

`rooms sync.Map` stores `roomID → *Room`. `secrets sync.Map` stores `secret → roomID`. Keeping them separate prevents the type assertion `val.(*Room)` from panicking if a secret string was accidentally passed as a room ID lookup.

### Single source of truth

The server controls everything: deck order, shuffle, card timing, click validation, result. Clients send only intent (`click`, `room_join`, etc.) — no game state originates from the client.

### No persistence

All state is in memory. No database, no reconnection logic. A disconnected player ends the game — appropriate for a reflex game where even a 5-second drop makes the session unplayable.

---

## Configuration

```bash
cp .env.example .env
```

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `ALLOWED_ORIGINS` | `*` | Comma-separated WebSocket origins. Set to your frontend URL in production |
| `CARD_INTERVAL_MS` | `2000` | Milliseconds between card reveals. Minimum: `500` |
| `MAX_NUM_WS_PER_IP` | `20` | Max simultaneous WebSocket connections from one IP |
| `MAX_NUM_OF_WS_ALLOWED_INTOTAL` | `10000` | Max simultaneous WebSocket connections across all IPs |
| `MAX_NUMBER_OF_PLAYER_ALLOWED_IN_ROOM` | `10` | Max players per secret room |

---

## Running Tests

```bash
# All tests
go test ./...

# With race detector (recommended — catches concurrency bugs)
go test -race ./...

# Game logic only, verbose
go test -v -race ./internal/game/...

# Single test group
go test -v -run TestHandleClick ./internal/game/...
go test -v -run TestJoin ./internal/game/...
go test -v -run TestLeave ./internal/game/...
```

Tests cover:

- Anonymous matchmaking: waiting slot, pairing, player ID assignment, room storage
- Secret rooms: create, join, start, non-host rejection
- Leave matrix: all six phase/kind combinations
- `HandleClick`: ace win, false click loss, guard conditions (pre-card, wrong phase, double click)
- `OpponentDisconnected`: survivor notification, idempotency on finished room
- `Start` loop: card reveal, both players see same card, draw after 52 cards
- Concurrency: 20 simultaneous anonymous pairs under `-race`