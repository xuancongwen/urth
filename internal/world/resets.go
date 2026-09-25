package world

import (
	"math/rand/v2"

	"urth/internal/item"
	"urth/internal/reset"
	"urth/internal/room"
)

// Resets repopulate areas on a schedule; wander moves mobs about. Both run
// from round(). Neither is a rule: how often mobs move and how many spawn
// is content, set in area files.

// wanderChance is the per-round chance a non-sentinel mob moves, as ROM's
// one-in-eight per mobile pulse.
const wanderChance = 8

type areaState struct {
	dir        string
	resets     *reset.Area
	roundsLeft int
}

func (w *World) initAreas() {
	for name, r := range w.content.Resets {
		a := &areaState{dir: name, resets: r}
		a.roundsLeft = w.roundsFor(r.IntervalSeconds)
		w.areas[name] = a
		w.resetArea(a)
	}
}

func (w *World) roundsFor(seconds int) int {
	n := seconds * 1000 / w.cfg.Timing.RoundMs
	if n < 1 {
		n = 1
	}
	return n
}

// tickAreas counts down each area and repopulates when due.
func (w *World) tickAreas() {
	for _, a := range w.areas {
		a.roundsLeft--
		if a.roundsLeft <= 0 {
			a.roundsLeft = w.roundsFor(a.resets.IntervalSeconds)
			w.resetArea(a)
		}
	}
}

// resetArea applies every reset in the area's list. Limits make it safe to
// run repeatedly: nothing is duplicated.
func (w *World) resetArea(a *areaState) {
	spawned := 0
	for _, r := range a.resets.Resets {
		rm, ok := w.content.Rooms.Get(r.Room)
		if !ok {
			continue
		}
		switch {
		case r.Mob != 0:
			proto := w.content.Mobs[r.Mob]
			if w.countMobs(proto) >= r.Max {
				continue
			}
			m := w.newMob(proto)
			for _, eq := range r.Equip {
				it := item.New(w.content.Items[eq.Item])
				if eq.Slot != "" {
					m.Equipment[item.Slot(eq.Slot)] = it
				} else {
					m.Inventory = append(m.Inventory, it)
				}
			}
			w.recalc(m.Character)
			m.Health, m.Mana = m.HealthMax, m.ManaMax
			w.placeMob(m, rm)
			spawned++
		case r.Into != 0:
			var container *item.Item
			for _, it := range w.contents(rm).items {
				if it.Proto.Vnum == r.Into && it.IsContainer() {
					container = it
					break
				}
			}
			if container == nil || hasVnum(container.Contents, r.Item) {
				continue
			}
			container.Contents = append(container.Contents, item.New(w.content.Items[r.Item]))
			spawned++
		default:
			c := w.contents(rm)
			if hasVnum(c.items, r.Item) {
				continue
			}
			c.items = append(c.items, item.New(w.content.Items[r.Item]))
			spawned++
		}
	}
	w.log.Debug("area reset", "area", a.dir, "spawned", spawned)
}

func hasVnum(list []*item.Item, vnum int) bool {
	for _, it := range list {
		if it.Proto.Vnum == vnum {
			return true
		}
	}
	return false
}

// wanderMobs gives each mobile mob a chance to step through a random exit.
func (w *World) wanderMobs() {
	// Collect first: moving mutates the per-room lists we iterate.
	var movers []*Mob
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			if !m.Proto.Sentinel() && m.Fighting == nil && w.rng.IntN(wanderChance) == 0 {
				movers = append(movers, m)
			}
		}
	}
	for _, m := range movers {
		w.wander(m)
	}
}

func (w *World) wander(m *Mob) {
	exits := m.Room.ExitList()
	if len(exits) == 0 {
		return
	}
	dir := exits[w.rng.IntN(len(exits))]
	dest, ok := w.content.Rooms.Get(m.Room.Exits[dir])
	if !ok {
		return
	}
	if m.Proto.StaysInArea() && dest.Area != m.Room.Area {
		return
	}
	w.moveMob(m, dest, dir)
}

// moveMob relocates a mob with leave and arrive messages.
func (w *World) moveMob(m *Mob, dest *room.Room, dir string) {
	w.act("$n leaves $t.", m.Character, nil, dir, toRoom)
	w.removeMobFromRoom(m)
	w.placeMob(m, dest)
	w.act("$n arrives from the $t.", m.Character, nil, room.Opposite[dir], toRoom)
}

// newRNG seeds the world's random source. Tests replace it for determinism.
func newRNG() *rand.Rand {
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
}
