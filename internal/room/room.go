// Package room holds the static world map: rooms and the exits between them.
// Rooms are loaded from data/world/<area>/rooms/*.yaml, one room per file.
package room

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Directions in the order ROM displays them.
var Directions = []string{"north", "east", "south", "west", "up", "down"}

// Opposite maps a direction to its reverse, for arrival messages.
var Opposite = map[string]string{
	"north": "south", "south": "north",
	"east": "west", "west": "east",
	"up": "down", "down": "up",
}

// Room is one location. Vnum is unique across all areas.
type Room struct {
	Vnum        int            `yaml:"vnum"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Exits       map[string]int `yaml:"exits"`
	// Flags: "safe" forbids fighting here (docs/RULES.md 4.7). Others are
	// free for rules and builders.
	Flags []string `yaml:"flags,omitempty"`
	// Temple names the deity whose temple this is ("good", "neutral",
	// "evil"); sacrifice works only here (docs/RULES.md 6.1).
	Temple string `yaml:"temple,omitempty"`

	// Area is the directory name the room was loaded from.
	Area string `yaml:"-"`
}

// HasFlag reports whether the room carries flag.
func (r *Room) HasFlag(flag string) bool {
	for _, f := range r.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// Safe reports whether fighting is forbidden here.
func (r *Room) Safe() bool { return r.HasFlag("safe") }

// Area is the optional data/world/<area>/area.yaml metadata.
type Area struct {
	Name   string `yaml:"name"`
	Author string `yaml:"author"`
}

// World is the loaded map.
type World struct {
	Rooms map[int]*Room
	Areas map[string]Area
}

// Get returns a room by vnum.
func (w *World) Get(vnum int) (*Room, bool) {
	r, ok := w.Rooms[vnum]
	return r, ok
}

// Load reads every area under dir. It fails on duplicate vnums, malformed
// files, unknown directions, or exits that lead nowhere.
func Load(dir string) (*World, error) {
	w := &World{Rooms: map[int]*Room{}, Areas: map[string]Area{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read world dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := w.loadArea(dir, e.Name()); err != nil {
			return nil, err
		}
	}
	if len(w.Rooms) == 0 {
		return nil, fmt.Errorf("no rooms found under %s", dir)
	}
	if err := w.validate(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *World) loadArea(dir, name string) error {
	areaDir := filepath.Join(dir, name)
	var area Area
	if raw, err := os.ReadFile(filepath.Join(areaDir, "area.yaml")); err == nil {
		if err := yaml.Unmarshal(raw, &area); err != nil {
			return fmt.Errorf("area %s: %w", name, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("area %s: %w", name, err)
	}
	if area.Name == "" {
		area.Name = name
	}
	w.Areas[name] = area

	roomsDir := filepath.Join(areaDir, "rooms")
	files, err := filepath.Glob(filepath.Join(roomsDir, "*.yaml"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var r Room
		if err := yaml.Unmarshal(raw, &r); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if r.Vnum <= 0 {
			return fmt.Errorf("%s: vnum must be positive", f)
		}
		if r.Name == "" {
			return fmt.Errorf("%s: name is required", f)
		}
		if prev, dup := w.Rooms[r.Vnum]; dup {
			return fmt.Errorf("%s: vnum %d already used by %s in area %s", f, r.Vnum, prev.Name, prev.Area)
		}
		r.Area = name
		r.Description = strings.TrimRight(r.Description, "\n")
		if r.Exits == nil {
			r.Exits = map[string]int{}
		}
		w.Rooms[r.Vnum] = &r
	}
	return nil
}

func (w *World) validate() error {
	for _, r := range w.Rooms {
		for dir, to := range r.Exits {
			if Opposite[dir] == "" {
				return fmt.Errorf("room %d (%s): unknown direction %q", r.Vnum, r.Name, dir)
			}
			if _, ok := w.Rooms[to]; !ok {
				return fmt.Errorf("room %d (%s): exit %s leads to missing room %d", r.Vnum, r.Name, dir, to)
			}
		}
	}
	return nil
}

// ExitList returns the room's exits in display order.
func (r *Room) ExitList() []string {
	var out []string
	for _, d := range Directions {
		if _, ok := r.Exits[d]; ok {
			out = append(out, d)
		}
	}
	return out
}
