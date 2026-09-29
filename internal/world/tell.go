package world

import (
	"strings"

	"urth/internal/output"
)

// Tells: private messages between players anywhere in the world. reply
// answers whoever last sent you one.

// findPlayer finds a playing character by name, or by the start of one
// when that is unambiguous, among those viewer can see.
func (w *World) findPlayer(viewer *Player, name string) *Player {
	name = strings.ToLower(name)
	var prefix []*Player
	for _, o := range w.players {
		if o.State != StatePlaying || !w.canSeeChar(viewer.Character, o.Character) {
			continue
		}
		lower := strings.ToLower(o.Name)
		if lower == name {
			return o
		}
		if strings.HasPrefix(lower, name) {
			prefix = append(prefix, o)
		}
	}
	if len(prefix) == 1 {
		return prefix[0]
	}
	return nil
}

// cmdTell: tell <player> <message>.
func cmdTell(w *World, p *Player, args string) {
	name, msg, _ := strings.Cut(args, " ")
	msg = strings.TrimSpace(msg)
	if name == "" || msg == "" {
		p.Send("Tell whom what?\n")
		return
	}
	target := w.findPlayer(p, name)
	if target == nil {
		p.Send("Nobody by that name is playing.\n")
		return
	}
	w.tell(p, target, msg)
}

// cmdReply: reply <message>. A tell to whoever last told you something.
func cmdReply(w *World, p *Player, args string) {
	if p.replyTo == "" {
		p.Send("Nobody has told you anything to reply to.\n")
		return
	}
	if args == "" {
		p.Send("Reply what?\n")
		return
	}
	for _, o := range w.players {
		if o.State == StatePlaying && o.Name == p.replyTo {
			w.tell(p, o, args)
			return
		}
	}
	p.Send(output.Escape(p.replyTo) + " is not playing any more.\n")
}

func (w *World) tell(from, to *Player, msg string) {
	if to == from {
		from.Send("You mutter to yourself.\n")
		return
	}
	msg = output.Escape(msg)
	from.Send("{M}You tell " + to.DisplayName() + " '" + msg + "'{x}\n")
	sender := from.DisplayName()
	if !w.canSeeChar(to.Character, from.Character) {
		sender = "Someone"
	}
	to.Send("{M}" + sender + " tells you '" + msg + "'{x}\n")
	if to.Position == posSleeping {
		from.Send(to.DisplayName() + " is asleep.\n")
	}
	to.replyTo = from.Name
}
