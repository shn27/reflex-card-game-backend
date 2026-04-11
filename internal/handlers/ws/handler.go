package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/shn27/reflex-card-game-backend/internal/game"
	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"github.com/shn27/reflex-card-game-backend/internal/ratelimit"
	"go.uber.org/zap"
)

// Handler holds the WebSocket upgrader and a reference to the matchmaker.
type Handler struct {
	upgrader   websocket.Upgrader
	matchmaker *game.Matchmaker
	limiter    *ratelimit.Limiter
}

func NewHandler(matchmaker *game.Matchmaker, allowedOrigins []string, limiter *ratelimit.Limiter) *Handler {
	return &Handler{
		matchmaker: matchmaker,
		limiter:    limiter,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
					return true
				}
				origin := r.Header.Get("Origin")
				for _, allowed := range allowedOrigins {
					if strings.EqualFold(origin, allowed) {
						return true
					}
				}
				logger.Logger.Warn("[ws] rejected origin: ", zap.String("origin", origin))

				return false
			},
		},
	}
}

// ServeWS upgrades the HTTP connection first, then checks rate limits. This
// order is intentional: upgrading first gives us a WebSocket channel to send
// a readable rate_limited message before closing, rather than an opaque HTTP
// 429 that the browser's WebSocket API cannot surface to application code.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade failed — roll back the limiter slot we reserved above.
		// defer will still fire, so Done() is called correctly.
		logger.Logger.Error("[ws] upgrade error",
			zap.String("remote_addr", r.RemoteAddr),
			zap.Error(err),
		)
		return
	}

	// Check limits after upgrading so we can deliver a meaningful message.
	if reason := h.limiter.Allow(r); reason != ratelimit.DenyReasonNone {
		logger.Logger.Warn("[ws] connection rate limited",
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("reason", string(reason)),
		)
		// Send the rate_limited event so the frontend can show a proper error,
		// then close with 1008 Policy Violation.
		conn.WriteJSON(game.OutMessage{
			Type:    game.MsgRateLimited,
			Message: string(reason),
		})
		conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limited"),
		)
		conn.Close()
		return
	}
	// Decrement the limiter counters when this connection eventually closes.
	defer h.limiter.Done(r)

	logger.Logger.Info("[ws] new connection",
		zap.String("remote_addr", r.RemoteAddr),
	)

	client := NewClient(conn)
	logger.Logger.Info("[ws] new connection from ", zap.String("remote_addr", r.RemoteAddr))

	go client.WritePump()

	client.ReadPump(h.matchmaker) // blocks until disconnected
}
