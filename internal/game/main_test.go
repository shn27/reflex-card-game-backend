package game

import (
	"os"
	"testing"

	"github.com/shn27/reflex-card-game-backend/internal/config"
	"github.com/shn27/reflex-card-game-backend/internal/logger"
)

// TestMain is the entry point for all tests in the game package.
// It mirrors the initialisation order in cmd/server/root.go:
// logger first, config second.
// Without this, any test that triggers a logger.Logger.Info() or reads
// config.Cfg will panic with a nil pointer dereference.
func TestMain(m *testing.M) {
	logger.InitLogger()
	defer logger.CloseLogger()

	config.LoadConfig()

	os.Exit(m.Run())
}
