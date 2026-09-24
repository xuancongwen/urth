package world

import (
	"strings"
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
//	$p  item name          $t  the text argument
//	$$  a literal dollar sign
//
// Names and text are already escaped by the caller where they come from
// player input. Every message ends with a newline and starts with a
// capital letter, so mob names like "a city guard" read correctly.
func (w *World) act(format string, actor *Character, victim *Character, text string, to actTarget) {
	w.actItem(format, actor, victim, "", text, to)
}

func (w *World) actItem(format string, actor *Character, victim *Character, itemName string, text string, to actTarget) {
	msg := capitalize(expandAct(format, actor, victim, itemName, text))
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
			if p.Character == actor || (to == toNotVict && p.Character == victim) {
				continue
			}
			p.Send(msg)
		}
	}
}

func expandAct(format string, actor, victim *Character, itemName, text string) string {
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
		case 'p':
			b.WriteString(itemName)
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

func actorName(c *Character) string {
	if c == nil {
		return "someone"
	}
	return escapeName(c.Name)
}

func escapeName(s string) string {
	return strings.ReplaceAll(s, "{", "{{")
}
