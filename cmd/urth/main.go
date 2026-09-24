// Command urth is the game server. Milestone 1: it loads configuration,
// sets up logging, and runs until asked to stop. The world loop and
// listeners arrive in milestone 2.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"urth/internal/config"
	"urth/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "urth:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "path to the server configuration file")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("urth %s (%s)\n", version.Version, version.Commit)
		return nil
	}

	cfg, found, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	logger := newLogger(cfg.Log)
	slog.SetDefault(logger)

	if found {
		logger.Info("config loaded", "path", *configPath)
	} else {
		logger.Warn("config file not found, using defaults", "path", *configPath)
	}
	if err := os.MkdirAll(cfg.Paths.Data, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	logger.Info("starting",
		"name", cfg.Server.Name,
		"version", version.Version,
		"commit", version.Commit,
		"telnet", cfg.Server.TelnetAddr,
		"websocket", cfg.Server.WebSocketAddr,
		"tick_ms", cfg.Timing.TickMs,
		"round_seconds", cfg.Timing.RoundSeconds,
		"data", cfg.Paths.Data,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	started := time.Now()
	<-ctx.Done()
	stop()

	logger.Info("shutting down", "uptime", time.Since(started).Round(time.Millisecond))
	return nil
}

func newLogger(cfg config.Log) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}
