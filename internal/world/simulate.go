package world

import (
	"math/rand/v2"
	"strconv"
	"strings"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/output"
)

// The simulator runs fights between throwaway characters using the same
// hooks as live combat, with a seeded random source, and reports the
// distribution. It is the balance tool: change a formula, run a thousand
// fights, read the numbers.

// SimResult summarises a batch of fights.
type SimResult struct {
	Fights    int
	AWins     int
	BWins     int
	Draws     int
	MeanRound float64
	MinRound  int
	MaxRound  int
	MeanDmgA  float64 // mean total damage dealt by A per fight
	MeanDmgB  float64
}

const simMaxRounds = 200

// simulate runs n fights. seed 0 uses the world's random source.
func (w *World) simulate(protoA, protoB *mob.Proto, playerA *Player, n int, seed uint64) SimResult {
	res := SimResult{Fights: n, MinRound: simMaxRounds}
	rng := w.rng
	if seed != 0 {
		rng = rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	}
	if w.scripts != nil {
		w.scripts.SetRandom(rng)
		defer w.scripts.SetRandom(w.rng)
	}
	var totalRounds, totalA, totalB int
	for i := 0; i < n; i++ {
		var a *Character
		if playerA != nil {
			a = w.cloneForSim(playerA.Character)
		} else {
			a = w.simCharacter(protoA)
		}
		b := w.simCharacter(protoB)
		rounds, dmgA, dmgB := w.simFight(a, b)
		totalRounds += rounds
		totalA += dmgA
		totalB += dmgB
		switch {
		case b.Health <= 0 && a.Health > 0:
			res.AWins++
		case a.Health <= 0 && b.Health > 0:
			res.BWins++
		default:
			res.Draws++
		}
		res.MinRound = min(res.MinRound, rounds)
		res.MaxRound = max(res.MaxRound, rounds)
	}
	if n > 0 {
		res.MeanRound = float64(totalRounds) / float64(n)
		res.MeanDmgA = float64(totalA) / float64(n)
		res.MeanDmgB = float64(totalB) / float64(n)
	}
	return res
}

// simCharacter builds a detached mob character from a prototype, equipped
// the way its first reset entry would spawn it.
func (w *World) simCharacter(p *mob.Proto) *Character {
	m := w.newMob(p)
	m.Stats = copyStats(p.Stats)
	for _, a := range w.content.Resets {
		for _, r := range a.Resets {
			if r.Mob != p.Vnum {
				continue
			}
			for _, eq := range r.Equip {
				if eq.Slot != "" {
					m.Equipment[item.Slot(eq.Slot)] = item.New(w.content.Items[eq.Item])
				}
			}
			goto equipped
		}
	}
equipped:
	w.recalc(m.Character)
	m.Health, m.Mana = m.HealthMax, m.ManaMax
	return m.Character
}

// cloneForSim copies a player's character sheet and equipment views.
func (w *World) cloneForSim(src *Character) *Character {
	c := newCharacter(src.Name, src.Keywords)
	c.Level = src.Level
	c.Experience = src.Experience
	c.Stats = copyStats(src.Stats)
	for slot, it := range src.Equipment {
		c.Equipment[slot] = item.New(it.Proto)
	}
	w.recalc(c)
	c.Health, c.Mana = c.HealthMax, c.ManaMax
	return c
}

// simFight runs rounds until someone drops or the cap is hit. Damage is
// applied directly; no messages, no death handling.
func (w *World) simFight(a, b *Character) (rounds, dmgA, dmgB int) {
	a.Fighting, b.Fighting = b, a
	for rounds = 1; rounds <= simMaxRounds; rounds++ {
		for i := 0; i < a.AttacksPerRound && b.Health > 0; i++ {
			r := w.resolveAttack(a, b, a.Equipment["wield"], rounds)
			if r.Hit {
				b.Health -= r.Damage
				dmgA += r.Damage
			}
		}
		if b.Health <= 0 {
			return rounds, dmgA, dmgB
		}
		for i := 0; i < b.AttacksPerRound && a.Health > 0; i++ {
			r := w.resolveAttack(b, a, b.Equipment["wield"], rounds)
			if r.Hit {
				a.Health -= r.Damage
				dmgB += r.Damage
			}
		}
		if a.Health <= 0 {
			return rounds, dmgA, dmgB
		}
		ta, tb := w.onTick(a), w.onTick(b)
		a.Health = clamp(a.Health+ta.HealthDelta, 0, a.HealthMax)
		b.Health = clamp(b.Health+tb.HealthDelta, 0, b.HealthMax)
	}
	return simMaxRounds, dmgA, dmgB
}

func copyStats(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cmdSimulate: simulate <mobvnum|me> <mobvnum> [fights] [seed]
func cmdSimulate(w *World, p *Player, args string) {
	fields := strings.Fields(args)
	if len(fields) < 2 {
		p.Send("Syntax: simulate <mob vnum | me> <mob vnum> [fights] [seed]\n")
		return
	}
	var protoA *mob.Proto
	var playerA *Player
	if fields[0] == "me" {
		playerA = p
	} else {
		v, err := strconv.Atoi(fields[0])
		protoA = w.content.Mobs[v]
		if err != nil || protoA == nil {
			p.Send("No such mob: " + output.Escape(fields[0]) + "\n")
			return
		}
	}
	v, err := strconv.Atoi(fields[1])
	protoB := w.content.Mobs[v]
	if err != nil || protoB == nil {
		p.Send("No such mob: " + output.Escape(fields[1]) + "\n")
		return
	}
	n := 100
	if len(fields) > 2 {
		if n, err = strconv.Atoi(fields[2]); err != nil || n < 1 || n > 100000 {
			p.Send("Fights must be 1 to 100000.\n")
			return
		}
	}
	var seed uint64
	if len(fields) > 3 {
		if seed, err = strconv.ParseUint(fields[3], 10, 64); err != nil {
			p.Send("Seed must be a number.\n")
			return
		}
	}
	res := w.simulate(protoA, protoB, playerA, n, seed)
	nameA := "you"
	if protoA != nil {
		nameA = protoA.Name
	}
	var b strings.Builder
	b.WriteString("Simulated " + itoa(res.Fights) + " fights: " + output.Escape(nameA) + " vs " + output.Escape(protoB.Name) + "\n")
	b.WriteString("  A wins " + pct(res.AWins, res.Fights) + "  B wins " + pct(res.BWins, res.Fights) + "  draws " + pct(res.Draws, res.Fights) + "\n")
	b.WriteString("  rounds: mean " + ftoa(res.MeanRound) + "  min " + itoa(res.MinRound) + "  max " + itoa(res.MaxRound) + "\n")
	b.WriteString("  damage per fight: A " + ftoa(res.MeanDmgA) + "  B " + ftoa(res.MeanDmgB) + "\n")
	p.Send(b.String())
}

func pct(n, total int) string {
	if total == 0 {
		return "0%"
	}
	return strconv.FormatFloat(100*float64(n)/float64(total), 'f', 1, 64) + "%"
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
