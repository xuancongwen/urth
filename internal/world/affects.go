package world

import (
	"strings"

	"urth/internal/effect"
)

// cmdAffects: affects. What is acting on you: timed effects (buffs,
// poisons, burning) with the time left, then the permanent ones, most
// of them from feats. Gear's effects are on the gear; 'look' at it.
func cmdAffects(w *World, p *Player, _ string) {
	var timed, lasting []effect.Active
	for _, e := range p.Effects {
		if e.Rounds > 0 {
			timed = append(timed, e)
		} else {
			lasting = append(lasting, e)
		}
	}
	if len(timed)+len(lasting) == 0 {
		p.Send("You are not affected by anything.\n")
		return
	}
	var b strings.Builder
	if len(timed) > 0 {
		b.WriteString("You are affected by:\n")
		for _, e := range timed {
			// The rounds are shown as time left, so describe without them.
			bare := e
			bare.Rounds = 0
			b.WriteString("  " + padRight(w.describeEffect(bare), 48) + "{c}" + w.roundsLeft(e.Rounds) + "{x}\n")
		}
	}
	if len(lasting) > 0 {
		b.WriteString("Always:\n")
		for _, e := range lasting {
			b.WriteString("  " + w.describeEffect(e) + "\n")
		}
	}
	p.Send(b.String())
}

// roundsLeft renders a count of rounds as seconds or minutes.
func (w *World) roundsLeft(rounds int) string {
	secs := rounds * max(w.cfg.Timing.RoundMs, 1) / 1000
	if secs < 60 {
		return plural(max(secs, 1), "second")
	}
	return w.questMinutes(rounds)
}
