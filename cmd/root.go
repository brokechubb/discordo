// Package cmd defines the command-line interface commands
package cmd

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ayn2op/discordo/internal/config"
	"github.com/ayn2op/discordo/internal/keyring"
	"github.com/ayn2op/discordo/internal/logger"
	"github.com/ayn2op/discordo/internal/ui"
	"github.com/ayn2op/tview"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/ningen/v3"
)

var (
	discordState             *ningen.State
	app                      *application
	globalInteractionHandler *interactionHandler
	lastDMNotification       map[discord.UserID]time.Time // Track last DM notification per user
	dmMessageCount           map[discord.UserID]int       // Track message count per user
)

func Run() error {
	tokenEnvVar := os.Getenv("DISCORDO_TOKEN")
	tokenFlag := flag.String("token", tokenEnvVar, "authentication token")

	configPath := flag.String("config-path", config.DefaultPath(), "path of the configuration file")
	logPath := flag.String("log-path", logger.DefaultPath(), "path of the log file")
	logLevel := flag.String("log-level", "info", "log level")
	flag.Parse()

	var level slog.Level
	switch *logLevel {
	case "debug":
		ws.EnableRawEvents = true
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	if err := logger.Load(*logPath, level); err != nil {
		return fmt.Errorf("failed to load logger: %w", err)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply terminal-specific optimizations before initializing the application
	applyTerminalOptimizations(cfg)

	token := *tokenFlag
	if token == "" {
		token, err = keyring.GetToken()
		if err != nil {
			slog.Info("failed to retrieve token from keyring", "err", err)
		}
	}

	tview.Styles = tview.Theme{}

	// Initialize DM notification tracker
	lastDMNotification = make(map[discord.UserID]time.Time)
	dmMessageCount = make(map[discord.UserID]int)

	app = newApplication(cfg)
	return app.run(token)
}

// applyTerminalOptimizations applies terminal-specific optimizations to reduce visual artifacts
func applyTerminalOptimizations(cfg *config.Config) {
	// Apply general terminal optimizations first
	ui.ApplyGeneralTerminalOptimizations()

	// Apply Kitty-specific optimizations with config options
	ui.ApplyKittyOptimizationsWithConfig(
		cfg.Terminal.TrueColor,
		cfg.Terminal.ForceTerm,
		cfg.Terminal.OptimizeForKitty,
	)
}
