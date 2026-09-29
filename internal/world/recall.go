package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/room"
)

// Recall: every character has a recall point, the respawn room until they
// set another. Any room will do. recall takes them there at once, but
// not out of a fight. recall tutorial goes back to the start of the
// tutorial, the start room, whatever the recall point.

// recallRoom is where recall takes p, and whether it is their own choice.
// A chosen room that no longer exists falls back to the respawn room.
func (w *World) recallRoom(p *Player) (*room.Room, bool) {
	if p.rec != nil && p.rec.Recall != 0 {
		if r, ok := w.content.Rooms.Get(p.rec.Recall); ok {
			return r, true
		}
	}
	r, _ := w.content.Rooms.Get(w.cfg.World.Respawn())
	return r, false
}

// cmdRecall: recall | recall set | recall clear | recall show | recall tutorial
func cmdRecall(w *World, p *Player, args string) {
	switch strings.ToLower(strings.TrimSpace(args)) {
	case "":
		dest, _ := w.recallRoom(p)
		w.recall(p, dest)
	case "tutorial":
		r, _ := w.content.Rooms.Get(w.cfg.World.StartRoom)
		w.recall(p, r)
	case "set":
		if p.Room == nil || p.rec == nil {
			return
		}
		p.rec.Recall = p.Room.Vnum
		p.Send("You fix " + output.Escape(p.Room.Name) + " in your memory. Recall will bring you here.\n")
		w.save(p)
	case "clear":
		if p.rec == nil {
			return
		}
		p.rec.Recall = 0
		r, _ := w.recallRoom(p)
		p.Send("Your recall point is " + roomName(r) + " again.\n")
		w.save(p)
	case "show":
		r, chosen := w.recallRoom(p)
		if !chosen {
			p.Send("Your recall point is " + roomName(r) + ", where everyone starts. 'recall set' chooses another.\n")
			return
		}
		p.Send("Your recall point is " + roomName(r) + ".\n")
	default:
		p.Send("recall | recall set | recall clear | recall show | recall tutorial\n")
	}
}

func (w *World) recall(p *Player, dest *room.Room) {
	switch {
	case dest == nil:
		p.Send("You have nowhere to recall to.\n")
	case p.Fighting != nil:
		p.Send("You can't recall in the middle of a fight. Flee first.\n")
	case p.Room == dest:
		p.Send("You are already there.\n")
	default:
		p.Send("You close your eyes and think of " + roomName(dest) + ".\n")
		w.teleport(p, dest, "$n fades away.", "$n appears, blinking.")
		w.save(p)
	}
}

func roomName(r *room.Room) string {
	if r == nil {
		return "nowhere"
	}
	return output.Escape(r.Name)
}
