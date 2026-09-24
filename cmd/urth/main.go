// Command urth is the game server: one world goroutine driven by a fixed
// tick, fed by transport goroutines that own the sockets.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"urth/internal/config"
	"urth/internal/content"
	"urth/internal/copyover"
	"urth/internal/script"
	"urth/internal/session"
	"urth/internal/store"
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
	copyoverPath := flag.String("copyover", "", "internal: copyover state file written by the previous process")
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
		"pid", os.Getpid(),
		"telnet", cfg.Server.TelnetAddr,
		"websocket", cfg.Server.WebSocketAddr,
		"tick_ms", cfg.Timing.TickMs,
		"round_seconds", cfg.Timing.RoundSeconds,
		"data", cfg.Paths.Data,
	)

	world_, err := content.Load(filepath.Join(cfg.Paths.Data, "world"))
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}
	if _, ok := world_.Rooms.Get(cfg.World.StartRoom); !ok {
		return fmt.Errorf("world.start_room %d does not exist", cfg.World.StartRoom)
	}
	logger.Info("world loaded", "areas", len(world_.Rooms.Areas), "rooms", len(world_.Rooms.Rooms),
		"items", len(world_.Items), "mobs", len(world_.Mobs))

	players, err := store.New(filepath.Join(cfg.Paths.Data, "players"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	scripts := script.New(filepath.Join(cfg.Paths.Data, "scripts"), logger.With("component", "script"), script.DefaultBudget)
	if err := scripts.Load(); err != nil {
		return fmt.Errorf("load scripts: %w", err)
	}

	// A copyover state file means we were exec'd by a previous instance and
	// should adopt its sockets instead of binding fresh ones.
	var state copyover.State
	if *copyoverPath != "" {
		state, err = copyover.Read(*copyoverPath)
		if err != nil {
			return fmt.Errorf("copyover: %w", err)
		}
		logger.Info("copyover: resuming", "listeners", len(state.Listeners), "players", len(state.Players))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var (
		tl *telnet.Listener
		ws *web.Server
	)
	deps := world.Deps{
		Store:    players,
		Scripts:  scripts,
		Shutdown: stop,
		LoadContent: func() (*content.World, error) {
			return content.Load(filepath.Join(cfg.Paths.Data, "world"))
		},
	}
	statePath := filepath.Join(cfg.Paths.Data, "copyover.json")
	deps.Copyover = func(st copyover.State) error {
		if err := copyover.Write(statePath, st); err != nil {
			return err
		}
		// Let the transports drain the "please hold" message before the
		// process image is replaced.
		time.Sleep(300 * time.Millisecond)
		logger.Warn("copyover: exec", "players", len(st.Players), "listeners", len(st.Listeners))
		return copyover.Exec(*configPath, statePath)
	}
	deps.Listeners = func() []copyover.Listener {
		var out []copyover.Listener
		add := func(kind string, f session.Filer) {
			if f == nil {
				return
			}
			file, err := f.File()
			if err == nil {
				err = copyover.Inherit(int(file.Fd()))
			}
			if err != nil {
				logger.Error("copyover: cannot hand over listener", "kind", kind, "err", err)
				return
			}
			out = append(out, copyover.Listener{Kind: kind, FD: int(file.Fd())})
		}
		if tl != nil {
			add("telnet", tl)
		}
		if ws != nil {
			add("web", ws)
		}
		return out
	}

	w := world.New(cfg, world_, logger.With("component", "world"), deps)
	w.RegisterTokens(state.Players)

	if cfg.Server.TelnetAddr != "" {
		tl = telnet.NewListener(cfg.Server.TelnetAddr, w.Events(), logger.With("component", "telnet"))
	}
	if cfg.Server.WebSocketAddr != "" {
		ws = web.NewServer(cfg.Server.WebSocketAddr, w.Events(), logger.With("component", "web"))
	}

	// Adopt inherited sockets before serving so restored players are known
	// to the world from its first tick.
	for _, l := range state.Listeners {
		ln, err := net.FileListener(os.NewFile(uintptr(l.FD), "listener"))
		if err != nil {
			logger.Error("copyover: adopt listener failed", "kind", l.Kind, "err", err)
			continue
		}
		switch {
		case l.Kind == "telnet" && tl != nil:
			tl.SetListener(ln)
		case l.Kind == "web" && ws != nil:
			ws.SetListener(ln)
		default:
			ln.Close()
		}
	}
	for _, cp := range state.Players {
		if cp.Kind != "telnet" || tl == nil {
			continue
		}
		nc, err := net.FileConn(os.NewFile(uintptr(cp.FD), cp.Name))
		if err != nil {
			logger.Error("copyover: adopt player failed", "name", cp.Name, "err", err)
			continue
		}
		tl.Adopt(nc, &session.Restore{Name: cp.Name, Room: cp.Room})
	}

	// Transports outlive the world's context: the world says goodbye and
	// closes its players first, then the transports stop and close any
	// straggler that never reached the world.
	tctx, tcancel := context.WithCancel(context.Background())
	defer tcancel()
	var wg sync.WaitGroup
	if tl != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tl.Serve(tctx); err != nil {
				logger.Error("telnet listener failed", "err", err)
				stop()
			}
		}()
	}
	if ws != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ws.Serve(tctx); err != nil {
				logger.Error("web server failed", "err", err)
				stop()
			}
		}()
	}

	started := time.Now()
	w.Run(ctx)
	tcancel()
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
