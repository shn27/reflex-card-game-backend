package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/shn27/reflex-card-game-backend/internal/game"
	"github.com/shn27/reflex-card-game-backend/internal/ws"
)

func main() {
	// Load .env if present (ignored in production where env vars are injected).
	if err := godotenv.Load(); err != nil {
		log.Println("[config] no .env file found, using environment")
	}

	cfg := loadConfig()

	matchmaker := game.NewMatchmaker(cfg.cardInterval)
	wsHandler := ws.NewHandler(matchmaker, cfg.allowedOrigins)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.ServeWS)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("[server] listening on :%s  card_interval=%s  origins=%v",
		cfg.port, cfg.cardInterval, cfg.allowedOrigins)
	log.Fatal(http.ListenAndServe(":"+cfg.port, mux))
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// ---- config ----------------------------------------------------------------

type config struct {
	port           string
	allowedOrigins []string
	cardInterval   time.Duration
}

func loadConfig() config {
	port := getEnv("PORT", "8080")

	origins := strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	intervalMs, err := strconv.Atoi(getEnv("CARD_INTERVAL_MS", "2000"))
	if err != nil || intervalMs < 500 {
		log.Fatalf("[config] invalid CARD_INTERVAL_MS: %s", os.Getenv("CARD_INTERVAL_MS"))
	}

	return config{
		port:           port,
		allowedOrigins: origins,
		cardInterval:   time.Duration(intervalMs) * time.Millisecond,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
