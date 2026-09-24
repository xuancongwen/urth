// Package config loads the server configuration from a YAML file, applying
// defaults for anything the file leaves out.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level server configuration.
type Config struct {
	Server Server `yaml:"server"`
	Timing Timing `yaml:"timing"`
	Paths  Paths  `yaml:"paths"`
	World  World  `yaml:"world"`
	Log    Log    `yaml:"log"`
}

// Server holds identity and listener settings.
type Server struct {
	// Name is shown to players and in logs.
	Name string `yaml:"name"`
	// TelnetAddr is the host:port for the line-oriented TCP listener.
	TelnetAddr string `yaml:"telnet_addr"`
	// WebSocketAddr is the host:port for the WebSocket listener. Empty disables it.
	WebSocketAddr string `yaml:"websocket_addr"`
}

// Timing controls the world loop.
type Timing struct {
	// TickMs is the world loop period in milliseconds. Input is processed once per tick.
	TickMs int `yaml:"tick_ms"`
	// RoundSeconds is the combat/regen round length. Everything paced by rounds keys off this.
	RoundSeconds int `yaml:"round_seconds"`
}

// Paths locates on-disk data.
type Paths struct {
	// Data is the root of world, player, and script files.
	Data string `yaml:"data"`
}

// World holds game-level settings that are not rules.
type World struct {
	// StartRoom is the vnum new characters enter the world in.
	StartRoom int `yaml:"start_room"`
}

// Log controls logging output.
type Log struct {
	// Level is one of debug, info, warn, error.
	Level string `yaml:"level"`
	// Format is "text" or "json".
	Format string `yaml:"format"`
}

// Default returns the configuration used when no file is present or a field is omitted.
func Default() Config {
	return Config{
		Server: Server{
			Name:          "Urth",
			TelnetAddr:    "127.0.0.1:4000",
			WebSocketAddr: "",
		},
		Timing: Timing{
			TickMs:       100,
			RoundSeconds: 3,
		},
		Paths: Paths{
			Data: "data",
		},
		World: World{
			StartRoom: 1,
		},
		Log: Log{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load reads path and overlays it on Default. A missing file is not an error;
// the defaults are returned and found is false.
func Load(path string) (cfg Config, found bool, err error) {
	cfg = Default()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, true, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, true, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, true, nil
}

// Validate rejects values the server cannot run with.
func (c Config) Validate() error {
	if c.Server.TelnetAddr == "" && c.Server.WebSocketAddr == "" {
		return errors.New("at least one of server.telnet_addr or server.websocket_addr must be set")
	}
	if c.Timing.TickMs < 10 || c.Timing.TickMs > 5000 {
		return fmt.Errorf("timing.tick_ms must be between 10 and 5000, got %d", c.Timing.TickMs)
	}
	if c.Timing.RoundSeconds < 1 {
		return fmt.Errorf("timing.round_seconds must be at least 1, got %d", c.Timing.RoundSeconds)
	}
	if c.Paths.Data == "" {
		return errors.New("paths.data must be set")
	}
	if c.World.StartRoom <= 0 {
		return fmt.Errorf("world.start_room must be a positive vnum, got %d", c.World.StartRoom)
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level must be debug, info, warn, or error, got %q", c.Log.Level)
	}
	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("log.format must be text or json, got %q", c.Log.Format)
	}
	return nil
}
