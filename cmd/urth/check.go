package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	"urth/internal/config"
	"urth/internal/content"
	"urth/internal/script"
	"urth/internal/store"
)

// runCheck is `urth check`: load and lint the content, load the scripts,
// and parse every player file, without starting a server. It exits
// non-zero on anything the server would refuse to boot with and on lint
// errors; warnings are printed and do not fail it. The deploy script runs
// it before pushing content.
func runCheck(args []string) error {
	fs := flag.NewFlagSet("urth check", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path to the server configuration file")
	quiet := fs.Bool("quiet", false, "print only errors and the summary")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: urth check [-config path] [-quiet]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, found, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if !found {
		fmt.Fprintf(os.Stderr, "note: %s not found, using defaults (data=%s)\n", *configPath, cfg.Paths.Data)
	}

	failed := false
	fail := func(format string, a ...any) {
		failed = true
		fmt.Printf("error: "+format+"\n", a...)
	}

	worldDir := filepath.Join(cfg.Paths.Data, "world")
	w, err := content.Load(worldDir)
	if err != nil {
		fail("%v", err)
	} else {
		problems := content.Lint(w, cfg.World.StartRoom)
		warnings := 0
		for _, p := range problems {
			if p.Level == content.Error {
				failed = true
			} else {
				warnings++
				if *quiet {
					continue
				}
			}
			if p.File != "" {
				fmt.Printf("%s  [%s]\n", p, p.File)
			} else {
				fmt.Println(p)
			}
		}
		fmt.Printf("world: %d areas, %d rooms, %d items, %d mobs; %d errors, %d warnings\n",
			len(w.Rooms.Areas), len(w.Rooms.Rooms), len(w.Items), len(w.Mobs), content.Errors(problems), warnings)
	}

	scriptDir := filepath.Join(cfg.Paths.Data, "scripts")
	engine := script.New(scriptDir, slog.New(slog.NewTextHandler(io.Discard, nil)), script.DefaultBudget)
	if err := engine.Load(); err != nil {
		fail("scripts: %v", err)
	} else {
		fmt.Println("scripts: ok")
	}

	playerDir := filepath.Join(cfg.Paths.Data, "players")
	if _, err := os.Stat(playerDir); err == nil {
		st, err := store.New(playerDir, bcrypt.DefaultCost)
		if err != nil {
			fail("players: %v", err)
		} else {
			names, err := st.List()
			if err != nil {
				fail("players: %v", err)
			}
			bad := 0
			for _, n := range names {
				if _, err := st.Load(n); err != nil {
					fail("player %s: %v", n, err)
					bad++
				}
			}
			fmt.Printf("players: %d files, %d unreadable\n", len(names), bad)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fail("players: %v", err)
	}

	if failed {
		return errors.New("check failed")
	}
	return nil
}
