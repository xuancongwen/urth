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
	// Doors are exits that can be shut. A closed door hides the exit
	// and whoever is beyond it. Declare a door on one side; the loader
	// mirrors it to the room on the far side.
	Doors map[string]*Door `yaml:"doors,omitempty"`
	// Flags: "safe" forbids fighting here (docs/RULES.md 4.7). Others are
	// free for rules and builders.
	Flags []string `yaml:"flags,omitempty"`
	// Temple names the deity whose temple this is ("good", "neutral",
	// "evil"); sacrifice works only here (docs/RULES.md 6.1).
	Temple string `yaml:"temple,omitempty"`
	// Position anchors the room on the area map (layout.go). Most rooms
	// leave it unset and are placed by walking exits from an anchor.
	Position *Coord `yaml:"position,omitempty"`

	// Area is the directory name the room was loaded from.
	Area string `yaml:"-"`
	// File is the path the room was read from, for tooling.
	File string `yaml:"-"`
	// X, Y, Z are the map position Layout assigned; Placed is false for a
	// room it could not fit, which the map leaves out.
	X, Y, Z int  `yaml:"-"`
	Placed  bool `yaml:"-"`
}

// Door is a shuttable exit. Closed is the state a reset restores.
type Door struct {
	// Name is how messages call it: "the oak door", "the iron gate".
	Name string `yaml:"name"`
	// Closed is the initial state; open doors are the default.
	Closed bool `yaml:"closed,omitempty"`
}

// Door returns the door on the exit in direction dir, or nil.
func (r *Room) Door(dir string) *Door {
	return r.Doors[dir]
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

// Dark reports whether the room needs a light to be seen.
func (r *Room) Dark() bool { return r.HasFlag("dark") }

// Area is the optional data/world/<area>/area.yaml metadata.
type Area struct {
	Name   string `yaml:"name"`
	Author string `yaml:"author"`
	// Detached marks an area that is not meant to be reachable on foot
	// from the start room, nor to place its prototypes by reset (the
	// balance range), so the content check does not report either.
	Detached bool `yaml:"detached,omitempty"`
}

// World is the loaded map.
type World struct {
	Rooms map[int]*Room
	Areas map[string]Area
	// Warnings are layout conflicts found at load, for the log.
	Warnings []string
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
	w.Warnings = w.Layout()
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
		r.File = f
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
		for dir, d := range r.Doors {
			if _, ok := r.Exits[dir]; !ok {
				return fmt.Errorf("room %d (%s): door %s has no exit", r.Vnum, r.Name, dir)
			}
			if d == nil || d.Name == "" {
				return fmt.Errorf("room %d (%s): door %s needs a name", r.Vnum, r.Name, dir)
			}
		}
	}
	w.mirrorDoors()
	return nil
}

// mirrorDoors copies each door to the far side of its exit when that
// side leads back and has no door of its own, so a builder declares a
// door once. Both sides share one Door value, so they open and close
// together.
func (w *World) mirrorDoors() {
	vnums := make([]int, 0, len(w.Rooms))
	for v := range w.Rooms {
		vnums = append(vnums, v)
	}
	sort.Ints(vnums)
	for _, v := range vnums {
		r := w.Rooms[v]
		for dir, d := range r.Doors {
			far := w.Rooms[r.Exits[dir]]
			back := Opposite[dir]
			if far.Exits[back] != r.Vnum {
				continue
			}
			if far.Doors == nil {
				far.Doors = map[string]*Door{}
			}
			if far.Doors[back] == nil {
				far.Doors[back] = d
			}
		}
	}
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
