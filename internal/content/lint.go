package content

import (
	"fmt"
	"sort"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/room"
)

// Lint is the content check that runs after a successful load: things
// that are legal but almost certainly mistakes. Load already rejects what
// the engine cannot run with (missing rooms, bad slots, duplicate vnums);
// this reports what a builder would want to know before shipping.

// Level is how serious a Problem is. Error fails the check; Warn is
// reported and does not.
type Level string

const (
	Error Level = "error"
	Warn  Level = "warn"
)

// Problem is one finding.
type Problem struct {
	Level Level  `json:"level"`
	Area  string `json:"area,omitempty"`
	// Kind names the check: "unreachable", "one-way", "unplaced", ...
	Kind string `json:"kind"`
	Text string `json:"text"`
	// File is the entity's source file when the finding is about one.
	File string `json:"file,omitempty"`
	// Vnum is the room, item, or mob concerned, when there is one.
	Vnum int `json:"vnum,omitempty"`
}

func (p Problem) String() string {
	s := string(p.Level) + ": "
	if p.Area != "" {
		s += p.Area + ": "
	}
	return s + p.Text
}

// Lint checks a loaded world. startRoom is where new characters begin;
// every room in a non-detached area should be reachable from it on foot.
// Findings are sorted by area, then level, then text.
func Lint(w *World, startRoom int) []Problem {
	var out []Problem
	add := func(p Problem) { out = append(out, p) }

	start, ok := w.Rooms.Get(startRoom)
	if !ok {
		add(Problem{Level: Error, Kind: "start", Text: fmt.Sprintf("start room %d does not exist", startRoom)})
	} else {
		reached := map[int]bool{start.Vnum: true}
		queue := []*room.Room{start}
		for len(queue) > 0 {
			r := queue[0]
			queue = queue[1:]
			for _, to := range r.Exits {
				if !reached[to] {
					reached[to] = true
					queue = append(queue, w.Rooms.Rooms[to])
				}
			}
		}
		for _, r := range w.Rooms.Rooms {
			if reached[r.Vnum] || w.Rooms.Areas[r.Area].Detached {
				continue
			}
			add(Problem{Level: Error, Area: r.Area, Kind: "unreachable", File: r.File, Vnum: r.Vnum,
				Text: fmt.Sprintf("room %d (%s) cannot be reached from the start room", r.Vnum, r.Name)})
		}
	}

	for _, r := range w.Rooms.Rooms {
		if r.Description == "" {
			add(Problem{Level: Warn, Area: r.Area, Kind: "description", File: r.File, Vnum: r.Vnum,
				Text: fmt.Sprintf("room %d (%s) has no description", r.Vnum, r.Name)})
		}
		if !r.Placed {
			add(Problem{Level: Warn, Area: r.Area, Kind: "layout", File: r.File, Vnum: r.Vnum,
				Text: fmt.Sprintf("room %d (%s) could not be placed on the map", r.Vnum, r.Name)})
		}
		for dir, to := range r.Exits {
			next := w.Rooms.Rooms[to]
			back, hasBack := next.Exits[room.Opposite[dir]]
			switch {
			case !hasBack:
				add(Problem{Level: Warn, Area: r.Area, Kind: "one-way", File: r.File, Vnum: r.Vnum,
					Text: fmt.Sprintf("room %d (%s) exit %s to %d (%s) has no way back", r.Vnum, r.Name, dir, to, next.Name)})
			case back != r.Vnum:
				add(Problem{Level: Warn, Area: r.Area, Kind: "mismatch", File: r.File, Vnum: r.Vnum,
					Text: fmt.Sprintf("room %d (%s) exit %s to %d (%s), but its %s leads to %d instead", r.Vnum, r.Name, dir, to, next.Name, room.Opposite[dir], back)})
			}
		}
	}
	for _, warn := range w.Rooms.Warnings {
		add(Problem{Level: Warn, Kind: "layout", Text: warn})
	}

	// Prototypes nobody places.
	placedItem, placedMob := map[int]bool{}, map[int]bool{}
	for area, a := range w.Resets {
		for i, rs := range a.Resets {
			if rs.Mob != 0 {
				placedMob[rs.Mob] = true
			}
			if rs.Item != 0 {
				placedItem[rs.Item] = true
			}
			if rs.Into != 0 {
				placedItem[rs.Into] = true
			}
			for _, eq := range rs.Equip {
				placedItem[eq.Item] = true
			}
			if r := w.Rooms.Rooms[rs.Room]; r != nil && r.Area != area {
				add(Problem{Level: Warn, Area: area, Kind: "foreign-reset", Vnum: rs.Room,
					Text: fmt.Sprintf("reset %d places into room %d (%s), which belongs to area %s", i, rs.Room, r.Name, r.Area)})
			}
		}
	}
	for _, m := range w.Mobs {
		if m.Spawns != nil && placedMob[m.Vnum] {
			placedMob[m.Spawns.Mob] = true // a generator's brood
		}
		for _, s := range m.Sells {
			placedItem[s.Item] = true // a quest vendor's stock
		}
		for _, t := range m.Trades {
			placedItem[t.Item] = true // a merchant's stock
		}
		for _, e := range m.Loot {
			placedItem[e.Item] = true // found in its corpse
		}
	}
	for _, a := range w.Rooms.Areas {
		for _, e := range a.Loot {
			placedItem[e.Item] = true // found in the area's corpses
		}
	}
	for _, p := range w.Items {
		if p.HasFlag("quest") {
			continue // handed out by the quest system, never by a reset
		}
		if p.HasFlag("newbie") {
			continue // sub issue gear, handed to new characters
		}
		if p.HasFlag("drop") {
			continue // found only in corpses, as an extra drop (docs/RULES.md 5.3)
		}
		if !placedItem[p.Vnum] && !w.Rooms.Areas[p.Area].Detached {
			add(Problem{Level: Warn, Area: p.Area, Kind: "unplaced", File: p.File, Vnum: p.Vnum,
				Text: fmt.Sprintf("item %d (%s) is never placed by a reset", p.Vnum, p.Name)})
		}
	}
	for _, p := range w.Mobs {
		if !placedMob[p.Vnum] && !w.Rooms.Areas[p.Area].Detached {
			add(Problem{Level: Warn, Area: p.Area, Kind: "unplaced", File: p.File, Vnum: p.Vnum,
				Text: fmt.Sprintf("mob %d (%s) is never placed by a reset", p.Vnum, p.Name)})
		}
	}

	// Rarity is a material's tier (docs/RULES.md 6.2); on anything else it
	// does nothing, and a named item is colored as one whatever it says.
	for _, p := range w.Items {
		if p.Rarity != "" && p.Type != item.Material {
			add(Problem{Level: Warn, Area: p.Area, Kind: "stray-rarity", File: p.File, Vnum: p.Vnum,
				Text: fmt.Sprintf("item %d (%s) is not a material, so its rarity %q does nothing", p.Vnum, p.Name, p.Rarity)})
		}
	}

	// Vnum ranges: one area's rooms, items, and mobs should not interleave
	// with another's, or the next builder cannot tell where a number lives.
	ranges := map[string]*[2]int{}
	note := func(area string, vnum int) {
		r, ok := ranges[area]
		if !ok {
			ranges[area] = &[2]int{vnum, vnum}
			return
		}
		if vnum < r[0] {
			r[0] = vnum
		}
		if vnum > r[1] {
			r[1] = vnum
		}
	}
	for _, r := range w.Rooms.Rooms {
		note(r.Area, r.Vnum)
	}
	for _, p := range w.Items {
		note(p.Area, p.Vnum)
	}
	for _, p := range w.Mobs {
		note(p.Area, p.Vnum)
	}
	names := make([]string, 0, len(ranges))
	for a := range ranges {
		names = append(names, a)
	}
	sort.Strings(names)
	for i, a := range names {
		for _, b := range names[i+1:] {
			ra, rb := ranges[a], ranges[b]
			if ra[0] <= rb[1] && rb[0] <= ra[1] {
				add(Problem{Level: Warn, Kind: "vnum-overlap",
					Text: fmt.Sprintf("vnums of area %s (%d to %d) overlap area %s (%d to %d)", a, ra[0], ra[1], b, rb[0], rb[1])})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		if out[i].Level != out[j].Level {
			return out[i].Level == Error
		}
		if out[i].Vnum != out[j].Vnum {
			return out[i].Vnum < out[j].Vnum
		}
		return out[i].Text < out[j].Text
	})
	return out
}

// Errors counts the findings that fail the check.
func Errors(problems []Problem) int {
	n := 0
	for _, p := range problems {
		if p.Level == Error {
			n++
		}
	}
	return n
}

// LintKinds warns on damage kinds the rules do not know (docs/RULES.md
// 4.8): a weapon's or a natural attack's kind, and the kind a resist,
// proc, added, aura, or thorns effect names. The kinds come from the rules'
// damageKinds hook, with the families a resistance may name besides.
func LintKinds(w *World, kinds []string) []Problem {
	known := map[string]bool{}
	for _, k := range kinds {
		known[k] = true
	}
	family := map[string]bool{"physical": true, "elemental": true, "all": true}
	var out []Problem
	warn := func(area, file string, vnum int, what, kind string) {
		out = append(out, Problem{Level: Warn, Area: area, Kind: "damage-kind", File: file, Vnum: vnum,
			Text: fmt.Sprintf("%s names damage kind %q, which the rules do not know", what, kind)})
	}
	effects := func(area, file string, vnum int, what string, specs []effect.Spec) {
		for _, e := range specs {
			k, _ := e.Params["kind"].(string)
			switch {
			case k == "":
			case e.Kind == "resist" && !known[k] && !family[k]:
				warn(area, file, vnum, what+"'s resist effect", k)
			case (e.Kind == "proc" || e.Kind == "added" || e.Kind == "aura" || e.Kind == "thorns") && !known[k]:
				warn(area, file, vnum, what+"'s "+e.Kind+" effect", k)
			}
		}
	}
	for _, p := range w.Items {
		what := fmt.Sprintf("item %d (%s)", p.Vnum, p.Name)
		if p.Weapon != nil && p.Weapon.Kind != "" && !known[p.Weapon.Kind] {
			warn(p.Area, p.File, p.Vnum, what, p.Weapon.Kind)
		}
		effects(p.Area, p.File, p.Vnum, what, p.Effects)
	}
	for _, p := range w.Mobs {
		what := fmt.Sprintf("mob %d (%s)", p.Vnum, p.Name)
		if p.Attack != nil && p.Attack.Kind != "" && !known[p.Attack.Kind] {
			warn(p.Area, p.File, p.Vnum, what, p.Attack.Kind)
		}
		effects(p.Area, p.File, p.Vnum, what, p.Effects)
	}
	return out
}
