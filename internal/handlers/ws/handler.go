package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/shn27/reflex-card-game-backend/internal/game"
	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"go.uber.org/zap"
)

// Handler holds the WebSocket upgrader and a reference to the matchmaker.
type Handler struct {
	upgrader   websocket.Upgrader
	matchmaker *game.Matchmaker
}

func NewHandler(matchmaker *game.Matchmaker, allowedOrigins []string) *Handler {
	return &Handler{
		matchmaker: matchmaker,
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

// ServeWS upgrades the HTTP connection and starts the client pumps.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Logger.Error("[ws] upgrade error: ", zap.Error(err))

		return
	}

	client := NewClient(conn)
	logger.Logger.Info("[ws] new connection from ", zap.String("remote_addr", r.RemoteAddr))

	go client.WritePump()

	client.ReadPump(h.matchmaker) // blocks until disconnected
}
