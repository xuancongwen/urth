// Command urth is the game server: one world goroutine driven by a fixed
// tick, fed by transport goroutines that own the sockets.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"urth/internal/config"
	"urth/internal/room"
	"urth/internal/telnet"
	"urth/internal/version"
	"urth/internal/web"
	"urth/internal/world"
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

	worldDir := filepath.Join(cfg.Paths.Data, "world")
	rooms, err := room.Load(worldDir)
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}
	if _, ok := rooms.Get(cfg.World.StartRoom); !ok {
		return fmt.Errorf("world.start_room %d does not exist", cfg.World.StartRoom)
	}
	logger.Info("world loaded", "areas", len(rooms.Areas), "rooms", len(rooms.Rooms))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	w := world.New(cfg, rooms, logger.With("component", "world"))

	var wg sync.WaitGroup
	if cfg.Server.TelnetAddr != "" {
		ln := telnet.NewListener(cfg.Server.TelnetAddr, w.Events(), logger.With("component", "telnet"))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ln.Serve(ctx); err != nil {
				logger.Error("telnet listener failed", "err", err)
				stop()
			}
		}()
	}

	if cfg.Server.WebSocketAddr != "" {
		ws := web.NewServer(cfg.Server.WebSocketAddr, w.Events(), logger.With("component", "web"))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ws.Serve(ctx); err != nil {
				logger.Error("web server failed", "err", err)
				stop()
			}
		}()
	}

	started := time.Now()
	w.Run(ctx)
	wg.Wait()

	logger.Info("shut down", "uptime", time.Since(started).Round(time.Millisecond))
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
