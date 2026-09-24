package world

import (
	"strings"

	"urth/internal/output"
)

// actTarget selects who receives an act message, following ROM's convention
// of one format string per perspective rather than conjugating verbs.
type actTarget int

const (
	// toChar: the actor only.
	toChar actTarget = iota
	// toVict: the victim only.
	toVict
	// toNotVict: everyone in the actor's room except actor and victim.
	toNotVict
	// toRoom: everyone in the actor's room except the actor.
	toRoom
)

// act formats and delivers a perspective message.
//
//	$n  actor's name       $N  victim's name
//	$t  the text argument  $$  a literal dollar sign
//
// Names and text are already escaped by the caller where they come from
// player input. Every message ends with a newline.
func (w *World) act(format string, actor *Player, victim *Player, text string, to actTarget) {
	msg := expandAct(format, actor, victim, text)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	switch to {
	case toChar:
		actor.Send(msg)
	case toVict:
		if victim != nil {
			victim.Send(msg)
		}
	case toNotVict, toRoom:
		if actor.Room == nil {
			return
		}
		for _, p := range w.playersIn(actor.Room) {
			if p == actor || (to == toNotVict && p == victim) {
				continue
			}
			p.Send(msg)
		}
	}
}

func expandAct(format string, actor, victim *Player, text string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '$' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		i++
		switch format[i] {
		case 'n':
			b.WriteString(actorName(actor))
		case 'N':
			b.WriteString(actorName(victim))
		case 't':
			b.WriteString(text)
		case '$':
			b.WriteByte('$')
		default:
			b.WriteByte('$')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

func actorName(p *Player) string {
	if p == nil {
		return "someone"
	}
	return output.Escape(p.Name)
}
