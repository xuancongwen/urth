package world

import (
	"strings"

	"urth/internal/output"
)

// The score sheet: a fixed-width bordered card. Padding is computed on
// plain text and color tokens are added afterwards, so the borders line
// up whether or not the client shows color.

const sheetWidth = 62

// statOrder is the display order for the six stats; anything else the
// rules define follows alphabetically.
var statOrder = []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}

type sheet struct{ b strings.Builder }

func (s *sheet) rule() { s.b.WriteString("{c}+" + strings.Repeat("-", sheetWidth-2) + "+{x}\n") }

// line writes one bordered row from plain text, padding to the width.
func (s *sheet) line(plain string) {
	s.b.WriteString("{c}|{x} " + padRight(plain, sheetWidth-4) + " {c}|{x}\n")
}

// pair writes two label/value cells side by side.
func (s *sheet) pair(l1, v1, l2, v2 string) {
	half := (sheetWidth - 4) / 2
	left := padRight(padRight(l1, 13)+v1, half)
	right := ""
	if l2 != "" {
		right = padRight(l2, 13) + v2
	}
	s.line(left + right)
}

// labelled writes a label and a value that may wrap onto further rows.
func (s *sheet) labelled(label, value string) {
	width := sheetWidth - 4 - 10
	words := strings.Fields(value)
	row := ""
	first := true
	flush := func() {
		if first {
			s.line(padRight(label, 10) + row)
			first = false
		} else {
			s.line(padRight("", 10) + row)
		}
		row = ""
	}
	for _, w := range words {
		if row != "" && len(row)+1+len(w) > width {
			flush()
		}
		if row != "" {
			row += " "
		}
		row += w
	}
	if row != "" || first {
		flush()
	}
}

func capitalizeWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// cmdScore shows the character sheet.
func cmdScore(w *World, p *Player, _ string) {
	var s sheet
	s.rule()
	s.line(padRight(output.Escape(p.Name), 36) + padRight("Level "+itoa(p.Level), 12) + "Speed " + ftoa(p.Speed))
	s.rule()
	next := w.xpToLevel(p.Level + 1)
	xp := itoa(p.Experience)
	if next < 1<<29 {
		xp += " (" + itoa(max(next-p.Experience, 0)) + " to next)"
	}
	health := itoa(p.Health) + "/" + itoa(p.HealthMax)
	s.pair("Health", health, "Experience", xp)
	s.pair("Coins", moneyString(p.Silver), "Quest points", itoa(p.questPoints))
	if p.ManaMax > 0 {
		s.pair("Mana", itoa(p.Mana)+"/"+itoa(p.ManaMax), "", "")
	}
	if p.StatPoints > 0 || p.FeatPoints > 0 {
		s.pair("Stat points", itoa(p.StatPoints), "Feat picks", itoa(p.FeatPoints))
	}
	if len(p.Stats) > 0 {
		s.rule()
		keys := orderedStats(p.Stats)
		rows := (len(keys) + 1) / 2
		show := func(k string) string {
			v := itoa(p.Stats[k])
			if eff, ok := p.EffStats[k]; ok && eff != p.Stats[k] {
				v += " (" + itoa(eff) + ")"
			}
			return v
		}
		for i := 0; i < rows; i++ {
			l1, v1 := capitalizeWord(keys[i]), show(keys[i])
			l2, v2 := "", ""
			if j := i + rows; j < len(keys) {
				l2, v2 = capitalizeWord(keys[j]), show(keys[j])
			}
			s.pair(l1, v1, l2, v2)
		}
	}
	var extras []func()
	if names := w.featNames(p.Character); len(names) > 0 {
		extras = append(extras, func() { s.labelled("Feats", strings.Join(names, ", ")) })
	}
	if len(p.Schools) > 0 {
		extras = append(extras, func() { s.labelled("Schools", output.Escape(strings.Join(p.Schools, ", "))) })
	}
	if p.Deity != "" {
		extras = append(extras, func() { s.labelled("Deity", "apostle of the "+output.Escape(p.Deity)+" god") })
	}
	var timed []string
	for _, e := range p.Effects {
		if e.Rounds > 0 {
			timed = append(timed, output.Escape(e.Kind)+" ("+itoa(e.Rounds)+")")
		}
	}
	if len(timed) > 0 {
		extras = append(extras, func() { s.labelled("Effects", strings.Join(timed, ", ")) })
	}
	if p.group != nil {
		var names []string
		for _, m := range p.group.members {
			if m != p.Character {
				names = append(names, output.Escape(m.Name))
			}
		}
		extras = append(extras, func() { s.labelled("Group", "with "+strings.Join(names, ", ")) })
	}
	if p.Fighting != nil {
		extras = append(extras, func() {
			s.labelled("Fighting", output.Escape(p.Fighting.Name)+", who "+condition(p.Fighting))
		})
	}
	if len(extras) > 0 {
		s.rule()
		for _, f := range extras {
			f()
		}
	}
	s.rule()
	p.Send(s.b.String())
}

// orderedStats returns the six stats in their canonical order, then any
// others alphabetically.
func orderedStats(stats map[string]int) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range statOrder {
		if _, ok := stats[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	var rest []string
	for k := range stats {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sortStrings(rest)
	return append(out, rest...)
}
