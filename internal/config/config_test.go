package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, found, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false for missing file")
	}
	if cfg != Default() {
		t.Fatalf("expected defaults, got %+v", cfg)
	}
}

func TestLoadOverlaysDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "server:\n  name: Test\ntiming:\n  tick_ms: 50\nlog:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, found, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if cfg.Server.Name != "Test" || cfg.Timing.TickMs != 50 || cfg.Log.Level != "debug" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.Server.TelnetAddr != Default().Server.TelnetAddr {
		t.Fatalf("default not preserved for telnet_addr: %q", cfg.Server.TelnetAddr)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"bad tick":   "timing:\n  tick_ms: 1\n",
		"bad level":  "log:\n  level: loud\n",
		"no listen":  "server:\n  telnet_addr: \"\"\n",
		"bad yaml":   "server: [\n",
		"bad format": "log:\n  format: xml\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := Load(path); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
