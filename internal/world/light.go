package world

import (
	"urth/internal/item"
	"urth/internal/output"
	"urth/internal/room"
)

// Light: a room flagged dark cannot be seen without a lit light worn by
// someone in it. Lights with a burn count go out after that many rounds
// of being worn.

// lightIn reports whether anyone in r wears a lit light.
func (w *World) lightIn(r *room.Room) bool {
	for _, c := range w.charactersIn(r) {
		if l := c.Equipment["light"]; l != nil && l.Lit() {
			return true
		}
	}
	return false
}

// canSee reports whether c can see its room.
func (w *World) canSee(c *Character) bool {
	return c.Room == nil || !c.Room.Dark() || w.lightIn(c.Room)
}

// burnLights counts down every worn light and puts out the spent ones.
func (w *World) burnLights() {
	for _, c := range w.allCharacters() {
		l := c.Equipment["light"]
		if l == nil || l.Burn <= 0 {
			continue
		}
		l.Burn--
		switch l.Burn {
		case 0:
			w.actItem("$p flickers and goes out.", c, nil, capitalize(output.Escape(l.Name())), "", toChar)
			w.actItem("$n's $p goes out.", c, nil, output.Escape(l.Name()), "", toRoom)
		case 10:
			w.actItem("$p is guttering.", c, nil, capitalize(output.Escape(l.Name())), "", toChar)
		}
	}
}

// wearLight puts a light in the light slot.
func (w *World) wearLight(p *Player, it *item.Item) {
	if !it.Lit() {
		w.actItem("$p has burned out.", p.Character, nil, capitalize(output.Escape(it.Name())), "", toChar)
		return
	}
	dark := !w.canSee(p.Character)
	w.wearItem(p, it, "light", "You light $p.", "$n lights $p.")
	if dark {
		w.look(p)
	}
}
