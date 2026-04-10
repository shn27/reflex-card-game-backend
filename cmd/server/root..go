package server

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/shn27/reflex-card-game-backend/internal/config"
	"github.com/shn27/reflex-card-game-backend/internal/logger"
	"github.com/shn27/reflex-card-game-backend/internal/routes"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "Reflex-card-game-backend",
	Long:  `Reflex-card-game-backend`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.InitLogger()
		defer logger.CloseLogger()
		// Load .env if present (ignored in production where env vars are injected).
		if err := godotenv.Load(); err != nil {
			logger.Sugar.Info("[config] no .env file found, using environment")
		}

		cfg := config.LoadConfig()

		routes.SetupRoutes(cfg)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
