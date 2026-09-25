// Package content loads everything static under data/world: rooms, item and
// mob prototypes, and reset schedules, and checks that they refer to each
// other consistently.
package content

import (
	"fmt"
	"os"
	"path/filepath"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/reset"
	"urth/internal/room"
)

// World is the loaded static content.
type World struct {
	Rooms  *room.World
	Items  map[int]*item.Proto
	Mobs   map[int]*mob.Proto
	Resets map[string]*reset.Area // by area directory name
}

// Load reads dir and validates cross-references.
func Load(dir string) (*World, error) {
	rooms, err := room.Load(dir)
	if err != nil {
		return nil, err
	}
	w := &World{Rooms: rooms, Items: map[int]*item.Proto{}, Mobs: map[int]*mob.Proto{}, Resets: map[string]*reset.Area{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		areaDir := filepath.Join(dir, e.Name())
		if err := item.LoadArea(areaDir, e.Name(), w.Items); err != nil {
			return nil, err
		}
		if err := mob.LoadArea(areaDir, e.Name(), w.Mobs); err != nil {
			return nil, err
		}
		resets, err := reset.LoadArea(areaDir)
		if err != nil {
			return nil, err
		}
		w.Resets[e.Name()] = resets
	}
	return w, w.validate()
}

func (w *World) validate() error {
	for area, a := range w.Resets {
		for i, r := range a.Resets {
			where := fmt.Sprintf("area %s reset %d", area, i)
			if _, ok := w.Rooms.Rooms[r.Room]; !ok {
				return fmt.Errorf("%s: room %d does not exist", where, r.Room)
			}
			if r.Mob != 0 {
				if _, ok := w.Mobs[r.Mob]; !ok {
					return fmt.Errorf("%s: mob %d does not exist", where, r.Mob)
				}
				for _, eq := range r.Equip {
					p, ok := w.Items[eq.Item]
					if !ok {
						return fmt.Errorf("%s: equip item %d does not exist", where, eq.Item)
					}
					if eq.Slot != "" {
						if !item.ValidSlot(item.Slot(eq.Slot)) {
							return fmt.Errorf("%s: unknown slot %q", where, eq.Slot)
						}
						if p.WearSlot() != item.Family(item.Slot(eq.Slot)) {
							return fmt.Errorf("%s: item %d (%s) cannot go in slot %s", where, eq.Item, p.Name, eq.Slot)
						}
					}
				}
			}
			if r.Item != 0 {
				if _, ok := w.Items[r.Item]; !ok {
					return fmt.Errorf("%s: item %d does not exist", where, r.Item)
				}
				if r.Into != 0 {
					c, ok := w.Items[r.Into]
					if !ok {
						return fmt.Errorf("%s: container item %d does not exist", where, r.Into)
					}
					if c.Type != item.Container {
						return fmt.Errorf("%s: item %d (%s) is not a container", where, r.Into, c.Name)
					}
				}
			}
		}
	}
	return nil
}
