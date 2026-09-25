package world

import "strings"

// Damage words (ROM's dam_message, keyed here to the share of the target's
// health a hit takes so the ladder means the same at every level). The
// rules supply the ladder through damageWords; without it every hit
// "hits".

// damageWord is one rung: hits taking less than Max of the target's
// health use Word; Shout ends the message with an exclamation mark.
type damageWord struct {
	Max   float64 `json:"max"`
	Word  string  `json:"word"`
	Shout bool    `json:"shout"`
}

var defaultLadder = []damageWord{{Max: 2, Word: "hit"}}

// loadDamageWords refreshes the ladder from the rules.
func (w *World) loadDamageWords() {
	var ladder []damageWord
	if w.callOptional("damageWords", &ladder) && len(ladder) > 0 {
		w.damageLadder = ladder
		return
	}
	w.damageLadder = defaultLadder
}

// hitWords picks the verb for dmg against target, in second person for the
// actor ("wound") and third person for onlookers ("wounds"), plus the
// closing punctuation.
func (w *World) hitWords(dmg int, target *Character) (second, third, punct string) {
	ladder := w.damageLadder
	if len(ladder) == 0 {
		ladder = defaultLadder
	}
	frac := float64(dmg) / float64(max(target.HealthMax, 1))
	rung := ladder[len(ladder)-1]
	for _, r := range ladder {
		if frac < r.Max {
			rung = r
			break
		}
	}
	punct = "."
	if rung.Shout {
		punct = "!"
	}
	return rung.Word, thirdPersonVerb(rung.Word), punct
}

// thirdPersonVerb conjugates the first word of a phrase: "wound" to
// "wounds", "MUTILATE" to "MUTILATES", "do UNSPEAKABLE things to" to
// "does UNSPEAKABLE things to".
func thirdPersonVerb(phrase string) string {
	first, rest, hasRest := strings.Cut(phrase, " ")
	lower := strings.ToLower(first)
	var suffix string
	switch {
	case lower == "do":
		suffix = "es"
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "sh"), strings.HasSuffix(lower, "ch"), strings.HasSuffix(lower, "x"), strings.HasSuffix(lower, "o"):
		suffix = "es"
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !strings.ContainsRune("aeiou", rune(lower[len(lower)-2])):
		first = first[:len(first)-1]
		suffix = "ies"
	default:
		suffix = "s"
	}
	if first == strings.ToUpper(first) && strings.ToLower(first) != first {
		suffix = strings.ToUpper(suffix)
	}
	out := first + suffix
	if hasRest {
		out += " " + rest
	}
	return out
}
