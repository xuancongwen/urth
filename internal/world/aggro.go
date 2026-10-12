package world

import "urth/internal/mob"

// Aggression (docs/RULES.md 8.1): a mob flagged "aggressive" attacks a
// player in its room on sight, once a round, as long as the player's
// level lies in the mob's aggro range. The range comes from the
// prototype or, left out, from the rules' mobBaseline, near the mob's
// own level, so a weak player is not run down by what would kill them
// and a strong one is not pestered by what cannot hurt them.

// aggression has every idle aggressive mob pick one player it can see in
// its room, in range, and attack. The fight starts now; the mob swings
// in the next round's violence, so the player has a moment to act.
func (w *World) aggression() {
	type attack struct {
		m    *Mob
		prey *Character
	}
	var attacks []attack
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			if p := w.aggroTarget(m); p != nil {
				attacks = append(attacks, attack{m, p})
			}
		}
	}
	for _, a := range attacks {
		if a.m.Fighting != nil || a.m.Health <= 0 || a.prey.Room != a.m.Room {
			continue
		}
		w.act("$n attacks you!", a.m.Character, a.prey, "", toVict)
		w.act("$n attacks $N!", a.m.Character, a.prey, "", toNotVict)
		w.startFight(a.m.Character, a.prey)
	}
}

// aggroTarget is the player m attacks this round, or nil.
func (w *World) aggroTarget(m *Mob) *Character {
	if !m.Proto.HasFlag("aggressive") || m.Proto.HasFlag("peaceful") || m.Fighting != nil || m.Health <= 0 || m.Room == nil || m.Room.Safe() {
		return nil
	}
	var prey []*Character
	for _, p := range w.playersIn(m.Room) {
		if w.aggroOn(m, p) {
			prey = append(prey, p.Character)
		}
	}
	if len(prey) == 0 {
		return nil
	}
	return prey[w.rng.IntN(len(prey))]
}

// aggroOn reports whether m would attack p on sight.
func (w *World) aggroOn(m *Mob, p *Player) bool {
	if p.State != StatePlaying || p.linkdead() || p.Admin || p.Health <= 0 {
		return false
	}
	return m.Proto.Resolved.Aggro.Holds(p.Level) && w.noticesChar(m.Character, p.Character)
}

// aggroWords words a range for builders: "6 to 15", "6 and up".
func aggroWords(r mob.Range) string {
	switch {
	case r.Min == 0 && r.Max == 0:
		return "any"
	case r.Max == 0:
		return itoa(r.Min) + " and up"
	case r.Min == 0:
		return "up to " + itoa(r.Max)
	}
	return itoa(r.Min) + " to " + itoa(r.Max)
}
