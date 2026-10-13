package world

import (
	"slices"
	"strings"

	"urth/internal/output"
)

// Arcane specialties (docs/RULES.md 6.1, decided 2026-10-12). A character
// swears to one school, for good, at the end of a Tower of High Sorcery's
// Test: a room whose Order holds the school. The rules say which schools
// exist, which Order holds each, which school opposes it, and what the
// oath requires; the engine records the choice and bars the opposite
// school's spells.

// School is one arcane school as the rules define it.
type School struct {
	ID       string `json:"id"`
	Order    string `json:"order"`
	Opposite string `json:"opposite"`
}

// Schools is what the optional magicSchools hook returns: the schools,
// the Orders' names, and what swearing requires.
type Schools struct {
	Schools []School          `json:"schools"`
	Orders  map[string]string `json:"orders"` // order id -> "the Black Robes"
	Level   int               `json:"level"`
	Stats   map[string]int    `json:"stats"`
}

// magicSchools asks the rules for the arcane schools. Without the hook
// there are none, and no specialty can be sworn.
func (w *World) magicSchools() Schools {
	var s Schools
	w.callOptional("magicSchools", &s)
	return s
}

func (s Schools) school(id string) (School, bool) {
	for _, sc := range s.Schools {
		if sc.ID == id {
			return sc, true
		}
	}
	return School{}, false
}

// specialtyWords is a specialty as score and spells show it:
// "necromancy, of the Black Robes".
func (w *World) specialtyWords(id string) string {
	s := w.magicSchools()
	if sc, ok := s.school(id); ok && s.Orders[sc.Order] != "" {
		return id + ", of " + s.Orders[sc.Order]
	}
	return id
}

// barred reports whether c's specialty closes sp to it: sp belongs to
// the school opposite the one c is sworn to.
func barred(c *Character, sp Spell) bool {
	return c.Specialty != "" && sp.Branch != "divine" && sp.Opposite != "" && sp.Opposite == c.Specialty
}

// barredWords says why a barred spell is closed.
func barredWords(c *Character, sp Spell) string {
	return capitalize(sp.School) + " is closed to you: you are sworn to " + c.Specialty + "."
}

// cmdSwear: swear | swear <school>. At the end of a Tower's Test, a
// character with no specialty takes one of the Order's schools, for good.
func cmdSwear(w *World, p *Player, args string) {
	order := p.Room.Order
	s := w.magicSchools()
	var offered []string
	for _, sc := range s.Schools {
		if order != "" && sc.Order == order {
			offered = append(offered, sc.ID)
		}
	}
	if len(offered) == 0 {
		p.Send("There is nothing here to swear to.\n")
		return
	}
	slices.Sort(offered)
	name := s.Orders[order]
	if name == "" {
		name = "the " + order + " Order"
	}
	if p.Specialty != "" {
		p.Send("You are already sworn to " + output.Escape(w.specialtyWords(p.Specialty)) + ", and an oath here is for life.\n")
		return
	}
	want := strings.ToLower(strings.TrimSpace(args))
	if want == "" {
		p.Send("Here you may swear to " + output.Escape(name) + ", in " + output.Escape(strings.Join(offered, " or ")) +
			". It is for life, and it closes the opposite school to you. Type 'swear <school>'.\n")
		if why := swearUnready(p.Character, s); why != "" {
			p.Send("You are not ready: it takes " + why + ".\n")
		}
		return
	}
	var school School
	for _, id := range offered {
		if strings.HasPrefix(id, want) {
			school, _ = s.school(id)
			break
		}
	}
	if school.ID == "" {
		p.Send(capitalize(output.Escape(name)) + " keep " + output.Escape(strings.Join(offered, " and ")) + ", not that.\n")
		return
	}
	if why := swearUnready(p.Character, s); why != "" {
		p.Send("The Order will not take your oath yet: it takes " + why + ".\n")
		return
	}
	p.Specialty = school.ID
	w.act("You swear yourself to "+output.Escape(school.ID)+", and "+output.Escape(name)+" take you in.", p.Character, nil, "", toChar)
	w.act("$n swears an oath to "+output.Escape(name)+".", p.Character, nil, "", toRoom)
	if school.Opposite != "" {
		p.Send("{Y}" + capitalize(output.Escape(school.Opposite)) + " is closed to you now.{x}\n")
	}
	w.save(p)
}

// swearUnready names what c lacks to swear, or "".
func swearUnready(c *Character, s Schools) string {
	var parts []string
	if c.Level < s.Level {
		parts = append(parts, "level "+itoa(s.Level))
	}
	if t := unmetStats(c, s.Stats); t != "" {
		parts = append(parts, needsText(s.Stats))
	}
	return strings.Join(parts, " and ")
}

// offeredSchools lists the schools of an Order, for tests and listings.
func (s Schools) offeredSchools(order string) []string {
	var out []string
	for _, sc := range s.Schools {
		if sc.Order == order {
			out = append(out, sc.ID)
		}
	}
	slices.Sort(out)
	return out
}
