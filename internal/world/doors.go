package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/room"
)

// Doors: an exit may carry a door (room.Door). A closed door blocks
// movement and hides the exit, so look, exits, and scan show nothing in
// that direction. Doors live on both sides of an exit and open and close
// together. Their state is not saved; an area reset shuts what the room
// file says is shut and opens the rest.

type doorKey struct {
	vnum int
	dir  string
}

// doorClosed reports whether the door on r's exit dir is shut. An exit
// without a door is never closed.
func (w *World) doorClosed(r *room.Room, dir string) bool {
	d := r.Door(dir)
	if d == nil {
		return false
	}
	if closed, ok := w.doors[doorKey{r.Vnum, dir}]; ok {
		return closed
	}
	return d.Closed
}

// setDoor opens or shuts the door on r's exit dir and the matching door
// on the far side, when the far side leads back here.
func (w *World) setDoor(r *room.Room, dir string, closed bool) {
	w.doors[doorKey{r.Vnum, dir}] = closed
	far, ok := w.content.Rooms.Get(r.Exits[dir])
	if !ok {
		return
	}
	back := room.Opposite[dir]
	if far.Exits[back] == r.Vnum && far.Door(back) != nil {
		w.doors[doorKey{far.Vnum, back}] = closed
	}
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
	case w.doorClosed(p.Room, dir):
		p.Send(capitalize(output.Escape(door.Name)) + " is closed.\n")
	default:
		p.Send(capitalize(output.Escape(door.Name)) + " is open.\n")
	}
	return true
}

// cmdScan: scan. Lists who stands in each adjacent room, skipping closed
// doors, rooms too dark to see into, and anyone the scanner cannot see.
func cmdScan(w *World, p *Player, _ string) {
	if !w.canSee(p.Character) {
		p.Send("You can't see a thing.\n")
		return
	}
	var b strings.Builder
	for _, dir := range w.openExits(p.Room) {
		dest := w.passable(p.Room, dir)
		if dest == nil || (dest.Dark() && !w.lightIn(dest)) {
			continue
		}
		var names []string
		for _, c := range w.visibleCharactersIn(p.Character, dest) {
			if c.IsPlayer() {
				names = append(names, c.DisplayName())
			} else {
				names = append(names, output.Escape(c.Name))
			}
		}
		if len(names) == 0 {
			continue
		}
		b.WriteString("{c}" + capitalize(dir) + "{x} - " + output.Escape(dest.Name) + ":\n")
		for _, n := range names {
			b.WriteString("    " + n + "\n")
		}
	}
	if b.Len() == 0 {
		p.Send("You see no one nearby.\n")
		return
	}
	p.Send(b.String())
}
