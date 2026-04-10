# reflex-card-game-backend

Go WebSocket backend for the Reflex Card Game.

## Checkout the Live APP
```
reflex-card-game-frontend.vercel.app
```

## Structure

```
reflex-card-game-backend/
├── cmd/server/
│   └── main.go          # entry point — config, wiring, HTTP mux
├── internal/
│   ├── game/
│   │   ├── card.go      # Card type, NewShuffledDeck
│   │   ├── message.go   # InMessage / OutMessage wire types
│   │   ├── player.go    # Player + Sender interface
│   │   ├── room.go      # game loop, click handling
│   │   └── matchmaker.go# pairs two players into a room
│   └── ws/
│       ├── client.go    # WebSocket read/write pumps, implements game.Sender
│       └── handler.go   # HTTP → WebSocket upgrade, origin check
├── .env.example
├── Dockerfile
└── go.mod
```

## Config (`.env`)

| Variable          | Default                    | Description                        |
|-------------------|----------------------------|------------------------------------|
| `PORT`            | `8080`                     | HTTP listen port                   |
| `ALLOWED_ORIGINS` | `*`                        | Comma-separated WebSocket origins  |
| `CARD_INTERVAL_MS`| `2000`                     | Milliseconds between card reveals  |

## Installing the application
You can run the application by cloning this to your local machine or directly from a Docker image.
The following sections will guide you to install the application:

### By Cloning Repository Locally
```bash
$ git@github.com:shn27/reflex-card-game-backend.git
$ cd reflex-card-game-backend
cp .env.example .env
go mod tidy
go run main.go
```

### Docker

```bash
docker build -t reflex-backend .
docker run -p 8080:8080 --env-file .env reflex-backend
```
-----

## WebSocket endpoint

```
ws://localhost:8080/ws
```

## Message contract

### Server → Client

| `type`        | Fields                                              | When                          |
|---------------|-----------------------------------------------------|-------------------------------|
| `waiting`     | `message`                                           | Player 1 waits for opponent   |
| `game_start`  | `player_id` (1 or 2)                               | Both players connected        |
| `card_reveal` | `card` {rank, suit}, `card_index` (1–52)           | Every `CARD_INTERVAL_MS` ms   |
| `game_over`   | `result` (win/lose/draw), `reason`, `winner_id`, `card` | Game ends for any reason  |

### Client → Server

| `type`  | When              |
|---------|-------------------|
| `click` | Player clicks button |
