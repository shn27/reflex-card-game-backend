package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shn27/reflex-card-game-backend/internal/logger"
)

type Config struct {
	Port           string
	AllowedOrigins []string
	CardInterval   time.Duration
}

func LoadConfig() Config {
	port := getEnv("PORT", "8080")

	origins := strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	intervalMs, err := strconv.Atoi(getEnv("CARD_INTERVAL_MS", "2000"))
	if err != nil || intervalMs < 500 {
		logger.Sugar.Fatalf("[config] invalid CARD_INTERVAL_MS: %s", os.Getenv("CARD_INTERVAL_MS"))
	}

	return Config{
		Port:           port,
		AllowedOrigins: origins,
		CardInterval:   time.Duration(intervalMs) * time.Millisecond,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
