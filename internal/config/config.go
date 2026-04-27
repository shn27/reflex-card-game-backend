package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"go.uber.org/zap"
)

type Config struct {
	Port           string
	AllowedOrigins []string
	CardInterval   time.Duration

	// Rate limiting
	MaxWSPerIP int64
	MaxWSTotal int64

	MaxNumberOfPlayerAllowedInRoom int64
}

var Cfg Config

func LoadConfig() {
	port := getEnv("PORT", "8080")

	origins := strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	intervalMs, err := strconv.Atoi(getEnv("CARD_INTERVAL_MS", "2000"))
	if err != nil || intervalMs < 500 {
		logger.Sugar.Fatalf("[config] invalid CARD_INTERVAL_MS: %s", os.Getenv("CARD_INTERVAL_MS"))
	}

	maxWSPerIP := parseInt("MAX_NUM_WS_PER_IP", 20)
	maxWSTotal := parseInt("MAX_NUM_OF_WS_ALLOWED_INTOTAL", 1000)

	maxNumberOfPlayerAllowedInRoom := parseInt("MAX_NUM_OF_PlAYER_ALLOWED_IN_ROOM", 10)

	Cfg = Config{
		Port:                           port,
		AllowedOrigins:                 origins,
		CardInterval:                   time.Duration(intervalMs) * time.Millisecond,
		MaxWSPerIP:                     maxWSPerIP,
		MaxWSTotal:                     maxWSTotal,
		MaxNumberOfPlayerAllowedInRoom: maxNumberOfPlayerAllowedInRoom,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseInt(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		logger.Logger.Fatal("[config] invalid integer value",
			zap.String("key", key),
			zap.String("value", v),
		)
	}
	return n
}
