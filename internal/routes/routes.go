package routes

import (
	"net/http"

	"github.com/shn27/reflex-card-game-backend/internal/config"
	"github.com/shn27/reflex-card-game-backend/internal/game"
	"github.com/shn27/reflex-card-game-backend/internal/handlers"
	"github.com/shn27/reflex-card-game-backend/internal/handlers/ws"
	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"github.com/shn27/reflex-card-game-backend/internal/ratelimit"
	"go.uber.org/zap"
)

// SetupRoutes initializes all the routes for the application
func SetupRoutes(cfg config.Config) {
	matchmaker := game.NewMatchmaker(cfg.CardInterval)
	limiter := ratelimit.New(cfg.MaxWSTotal, cfg.MaxWSPerIP)
	wsHandler := ws.NewHandler(matchmaker, cfg.AllowedOrigins, limiter)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.ServeWS)
	mux.HandleFunc("/health", handlers.HealthHandler)

	logger.Sugar.Infof("[server] listening on :%s  card_interval=%s  origins=%v maximum_ws_allowed_in_total=%v maximum_ws_allowed_per_IP=%v maximum_number_of_player_allowed_in_a_room=%v",
		cfg.Port, cfg.CardInterval, cfg.AllowedOrigins, cfg.MaxWSTotal, cfg.MaxWSPerIP, cfg.MaxNumberOfPlayerAllowedInRoom)

	logger.Logger.Fatal("[server] failed to start",
		zap.String("port", cfg.Port),
		zap.Error(http.ListenAndServe(":"+cfg.Port, mux)),
	)
}
