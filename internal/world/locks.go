package world

import (
	"urth/internal/output"
	"urth/internal/room"
)

// Locks: a door may be locked. lock and unlock need the door's key, in
// the pack or in hand; pick tries without one, once a round, at a chance
// the rules set. A pickproof door cannot be picked.

// hasKey reports whether p carries or holds the item with vnum key.
func hasKey(p *Player, key int) bool {
	if key == 0 {
		return false
	}
	for _, it := range p.Inventory {
		if it.Proto.Vnum == key {
			return true
		}
	}
	for _, it := range p.Equipment {
		if it != nil && it.Proto.Vnum == key {
			return true
		}
	}
	return false
}

// lockTarget resolves args to a closed door here, telling the player why
// not when there is none.
func (w *World) lockTarget(p *Player, args, verb string) (string, *room.Door) {
	if args == "" {
		p.Send(capitalize(verb) + " what?\n")
		return "", nil
	}
	dir, door := w.findDoor(p, args)
	switch {
	case door == nil:
		p.Send("There is no door there.\n")
		return "", nil
	case !w.doorClosed(p.Room, dir):
		p.Send("It's not closed.\n")
		return "", nil
	}
	return dir, door
}

// cmdLock: lock <direction|door>. Needs the key.
func cmdLock(w *World, p *Player, args string) {
	dir, door := w.lockTarget(p, args, "lock")
	switch {
	case door == nil:
		return
	case door.Key == 0:
		p.Send("It has no lock you could turn.\n")
		return
	case w.doorLocked(p.Room, dir):
		p.Send("It's already locked.\n")
		return
	case !hasKey(p, door.Key):
		p.Send("You lack the key.\n")
		return
	}
	w.setLock(p.Room, dir, true)
	name := output.Escape(door.Name)
	w.act("*Click* You lock $t.", p.Character, nil, name, toChar)
	w.act("$n locks $t.", p.Character, nil, name, toRoom)
	w.tellFarSide(p.Room, dir, "You hear a key turn in "+name+".\n")
}

// cmdUnlock: unlock <direction|door>. Needs the key.
func cmdUnlock(w *World, p *Player, args string) {
	dir, door := w.lockTarget(p, args, "unlock")
	switch {
	case door == nil:
		return
	case !w.doorLocked(p.Room, dir):
		p.Send("It's not locked.\n")
		return
	case !hasKey(p, door.Key):
		p.Send("You lack the key.\n")
		return
	}
	w.setLock(p.Room, dir, false)
	name := output.Escape(door.Name)
	w.act("*Click* You unlock $t.", p.Character, nil, name, toChar)
	w.act("$n unlocks $t.", p.Character, nil, name, toRoom)
	w.tellFarSide(p.Room, dir, "You hear a key turn in "+name+".\n")
}

// defaultPickChance is the chance to pick a lock when the rules do not say.
const defaultPickChance = 0.5

// pickChance asks the rules how likely c is to pick door.
func (w *World) pickChance(c *Character, door *room.Door) float64 {
	chance := defaultPickChance
	var got float64
	if w.callOptional("pickChance", &got, w.view(c), map[string]any{"name": door.Name, "key": door.Key}) {
		chance = got
	}
	return min(max(chance, 0), 1)
}

// cmdPick: pick <direction|door>. Opens a lock without its key, sometimes.
func cmdPick(w *World, p *Player, args string) {
	dir, door := w.lockTarget(p, args, "pick")
	switch {
	case door == nil:
		return
	case !w.doorLocked(p.Room, dir):
		p.Send("It's not locked.\n")
		return
	case p.lastSkillRound == w.roundCount+1:
		p.Send("Your fingers are still busy. Give it a moment.\n")
		return
	}
	p.lastSkillRound = w.roundCount + 1
	name := output.Escape(door.Name)
	if door.Pickproof || w.rng.Float64() >= w.pickChance(p.Character, door) {
		w.act("You work at the lock on $t, but it holds.", p.Character, nil, name, toChar)
		w.act("$n fiddles with the lock on $t.", p.Character, nil, name, toRoom)
		return
	}
	w.setLock(p.Room, dir, false)
	w.act("*Click* You pick the lock on $t.", p.Character, nil, name, toChar)
	w.act("$n picks the lock on $t.", p.Character, nil, name, toRoom)
}
