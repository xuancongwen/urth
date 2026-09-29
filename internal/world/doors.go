package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/room"
)

// Doors: an exit may carry a door (room.Door). A closed door blocks
// movement and hides the exit, so look, exits, and scan show nothing in
// that direction. A locked door is closed and will not open until it is
// unlocked with its key or picked. Doors live on both sides of an exit
// and change together. Their state is not saved; an area reset puts
// every door back the way the room file says.

type doorKey struct {
	vnum int
	dir  string
}

// doorState is a door's state where it differs from its room file.
type doorState struct {
	closed, locked bool
}

// doorNow is the state of the door on r's exit dir, which must exist.
func (w *World) doorNow(r *room.Room, dir string, d *room.Door) doorState {
	if s, ok := w.doors[doorKey{r.Vnum, dir}]; ok {
		return s
	}
	return doorState{closed: d.Closed, locked: d.Locked}
}

// doorClosed reports whether the door on r's exit dir is shut. An exit
// without a door is never closed.
func (w *World) doorClosed(r *room.Room, dir string) bool {
	d := r.Door(dir)
	return d != nil && w.doorNow(r, dir, d).closed
}

// doorLocked reports whether the door on r's exit dir is locked.
func (w *World) doorLocked(r *room.Room, dir string) bool {
	d := r.Door(dir)
	return d != nil && w.doorNow(r, dir, d).locked
}

// setDoorState sets the door on r's exit dir, and the matching door on
// the far side when the far side leads back here.
func (w *World) setDoorState(r *room.Room, dir string, s doorState) {
	w.doors[doorKey{r.Vnum, dir}] = s
	far, ok := w.content.Rooms.Get(r.Exits[dir])
	if !ok {
		return
	}
	back := room.Opposite[dir]
	if far.Exits[back] == r.Vnum && far.Door(back) != nil {
		w.doors[doorKey{far.Vnum, back}] = s
	}
}

// setDoor opens or shuts the door on r's exit dir, both sides.
func (w *World) setDoor(r *room.Room, dir string, closed bool) {
	s := w.doorNow(r, dir, r.Door(dir))
	s.closed = closed
	w.setDoorState(r, dir, s)
}

// setLock locks or unlocks the door on r's exit dir, both sides.
func (w *World) setLock(r *room.Room, dir string, locked bool) {
	s := w.doorNow(r, dir, r.Door(dir))
	s.locked = locked
	w.setDoorState(r, dir, s)
}

// resetDoors returns every door in an area to its room-file state, along
// with the far side of each, which may lie in another area.
func (w *World) resetDoors(area string) {
	for k := range w.doors {
		r, ok := w.content.Rooms.Get(k.vnum)
		if !ok || r.Area != area {
			continue
		}
		delete(w.doors, k)
		if far, ok := w.content.Rooms.Get(r.Exits[k.dir]); ok {
			delete(w.doors, doorKey{far.Vnum, room.Opposite[k.dir]})
		}
	}
}

// passable returns the room beyond r's exit dir, or nil when there is no
// exit that way or its door is shut.
func (w *World) passable(r *room.Room, dir string) *room.Room {
	to, ok := r.Exits[dir]
	if !ok || w.doorClosed(r, dir) {
		return nil
	}
	dest, ok := w.content.Rooms.Get(to)
	if !ok {
		return nil
	}
	return dest
}

// openExits lists r's exits in display order, leaving out closed doors.
func (w *World) openExits(r *room.Room) []string {
	var out []string
	for _, d := range r.ExitList() {
		if !w.doorClosed(r, d) {
			out = append(out, d)
		}
	}
	return out
}

// findDoor resolves a direction ("n", "north") or a door's name ("oak",
// "gate") to the direction of a door in the player's room.
func (w *World) findDoor(p *Player, ref string) (string, *room.Door) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return "", nil
	}
	for _, d := range room.Directions {
		if strings.HasPrefix(d, ref) {
			return d, p.Room.Door(d)
		}
	}
	for _, d := range room.Directions {
		door := p.Room.Door(d)
		if door == nil {
			continue
		}
		for _, word := range strings.Fields(door.Name) {
			if strings.HasPrefix(strings.ToLower(word), ref) {
				return d, door
			}
		}
	}
	return "", nil
}

// cmdOpen: open <direction|door>.
func cmdOpen(w *World, p *Player, args string) {
	dir, door := w.findDoor(p, args)
	switch {
	case args == "":
		p.Send("Open what?\n")
		return
	case door == nil:
		p.Send("There is no door there.\n")
		return
	case !w.doorClosed(p.Room, dir):
		p.Send("It's already open.\n")
		return
	case w.doorLocked(p.Room, dir):
		p.Send("It's locked.\n")
		return
	}
	w.setDoor(p.Room, dir, false)
	name := output.Escape(door.Name)
	w.act("You open $t.", p.Character, nil, name, toChar)
	w.act("$n opens $t.", p.Character, nil, name, toRoom)
	w.tellFarSide(p.Room, dir, capitalize(name)+" opens.\n")
}

// cmdClose: close <direction|door>.
func cmdClose(w *World, p *Player, args string) {
	dir, door := w.findDoor(p, args)
	switch {
	case args == "":
		p.Send("Close what?\n")
		return
	case door == nil:
		p.Send("There is no door there.\n")
		return
	case w.doorClosed(p.Room, dir):
		p.Send("It's already closed.\n")
		return
	}
	w.setDoor(p.Room, dir, true)
	name := output.Escape(door.Name)
	w.act("You close $t.", p.Character, nil, name, toChar)
	w.act("$n closes $t.", p.Character, nil, name, toRoom)
	w.tellFarSide(p.Room, dir, capitalize(name)+" closes.\n")
}

// tellFarSide sends text to everyone in the room beyond r's exit dir.
func (w *World) tellFarSide(r *room.Room, dir string, text string) {
	far, ok := w.content.Rooms.Get(r.Exits[dir])
	if !ok {
		return
	}
	for _, other := range w.playersIn(far) {
		other.Send(text)
	}
}

// lookDirection describes what lies in a direction, or a named door: the
// door and its state, or the open way. It reports false when ref is
// neither a direction nor a door here.
func (w *World) lookDirection(p *Player, ref string) bool {
	dir, door := w.findDoor(p, ref)
	if dir == "" {
		return false
	}
	if _, ok := p.Room.Exits[dir]; !ok {
		p.Send("Nothing special there.\n")
		return true
	}
	switch {
	case door == nil:
		dest := w.passable(p.Room, dir)
		p.Send("To the " + dir + " lies " + output.Escape(dest.Name) + ".\n")
	case w.doorLocked(p.Room, dir):
		p.Send(capitalize(output.Escape(door.Name)) + " is closed and locked.\n")
	case w.doorClosed(p.Room, dir):
		p.Send(capitalize(output.Escape(door.Name)) + " is closed.\n")
	default:
		p.Send(capitalize(output.Escape(door.Name)) + " is open.\n")
	}
	return true
}

// cmdScan: scan. Lists who stands here with the scanner, then in each
// adjacent room, skipping closed doors, rooms too dark to see into, and
// anyone the scanner cannot see.
func cmdScan(w *World, p *Player, _ string) {
	if !w.canSee(p.Character) {
		p.Send("You can't see a thing.\n")
		return
	}
	var b strings.Builder
	w.scanRoom(&b, p, "Here", p.Room)
	for _, dir := range w.openExits(p.Room) {
		dest := w.passable(p.Room, dir)
		if dest == nil || (dest.Dark() && !w.lightIn(dest)) {
			continue
		}
		w.scanRoom(&b, p, capitalize(dir), dest)
	}
	if b.Len() == 0 {
		p.Send("You see no one nearby.\n")
		return
	}
	p.Send(b.String())
}

// scanRoom writes one section of a scan: the room under label and everyone
// in it the scanner can see, other than the scanner. Nothing is written
// for an empty room.
func (w *World) scanRoom(b *strings.Builder, p *Player, label string, r *room.Room) {
	var names []string
	for _, c := range w.visibleCharactersIn(p.Character, r) {
		switch {
		case c == p.Character:
		case c.IsPlayer():
			names = append(names, c.DisplayName())
		default:
			names = append(names, output.Escape(c.Name))
		}
	}
	if len(names) == 0 {
		return
	}
	b.WriteString("{c}" + label + "{x} - " + output.Escape(r.Name) + ":\n")
	for _, n := range names {
		b.WriteString("    " + n + "\n")
	}
}
