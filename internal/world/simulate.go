package world

import (
	"math"
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
// fights, read the numbers against docs/RULES.md 4.5.

// SimResult summarises a batch of fights.
type SimResult struct {
	Fights    int
	AWins     int
	BWins     int
	Draws     int
	MeanRound float64
	SDRound   float64
	MinRound  int
	MaxRound  int
	MeanDmgA  float64 // mean total damage dealt by A per fight
	MeanDmgB  float64
	SwingsA   float64 // mean swings per fight
	SwingsB   float64
	// Health A has left, as a fraction of its maximum, over fights A won.
	// The standard deviation is the 4.5 variance target.
	MeanLeftA float64
	SDLeftA   float64
}

const simMaxRounds = 200

// simulate runs n fights between fresh copies made by makeA and B's
// prototype. seed 0 uses the world's random source.
func (w *World) simulate(makeA func() *Character, protoB *mob.Proto, n int, seed uint64) SimResult {
	res := SimResult{Fights: n, MinRound: simMaxRounds}
	rng := w.rng
	if seed != 0 {
		rng = rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	}
	if w.scripts != nil {
		w.scripts.SetRandom(rng)
		defer w.scripts.SetRandom(w.rng)
	}
	var rounds, left []float64
	var totalA, totalB, swingsA, swingsB int
	for i := 0; i < n; i++ {
		a := makeA()
		b := w.simCharacter(protoB)
		f := w.simFight(a, b)
		rounds = append(rounds, float64(f.rounds))
		totalA += f.dmgA
		totalB += f.dmgB
		swingsA += f.swingsA
		swingsB += f.swingsB
		switch {
		case b.Health <= 0 && a.Health > 0:
			res.AWins++
			left = append(left, float64(a.Health)/float64(max(a.HealthMax, 1)))
		case a.Health <= 0 && b.Health > 0:
			res.BWins++
		default:
			res.Draws++
		}
		res.MinRound = min(res.MinRound, f.rounds)
		res.MaxRound = max(res.MaxRound, f.rounds)
	}
	if n > 0 {
		res.MeanRound, res.SDRound = meanSD(rounds)
		res.MeanDmgA = float64(totalA) / float64(n)
		res.MeanDmgB = float64(totalB) / float64(n)
		res.SwingsA = float64(swingsA) / float64(n)
		res.SwingsB = float64(swingsB) / float64(n)
	}
	if len(left) > 0 {
		res.MeanLeftA, res.SDLeftA = meanSD(left)
	}
	return res
}

func meanSD(xs []float64) (mean, sd float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	for _, x := range xs {
		sd += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(sd / float64(len(xs)))
}

// simCharacter builds a detached mob character from a prototype, equipped
// the way its first reset entry would spawn it.
func (w *World) simCharacter(p *mob.Proto) *Character {
	m := w.newMob(p)
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

// cloneForSim copies a player's character sheet, effects, and equipment.
func (w *World) cloneForSim(src *Character) *Character {
	c := newCharacter(src.Name, src.Keywords)
	c.Level = src.Level
	c.Experience = src.Experience
	c.Stats = copyStats(src.Stats)
	c.Effects = append(c.Effects, src.Effects...)
	for slot, it := range src.Equipment {
		clone := item.New(it.Proto)
		clone.Effects = append(clone.Effects, it.Effects...)
		c.Equipment[slot] = clone
	}
	w.recalc(c)
	c.Health, c.Mana = c.HealthMax, c.ManaMax
	return c
}

// simFighter builds a level-N character in the rules' standard kit with
// a fresh sheet and no points spent: the "player at level in standard
// gear" that docs/RULES.md 4.5 measures against.
func (w *World) simFighter(level int) *Character {
	c := newCharacter("a level "+itoa(level)+" fighter", []string{"fighter"})
	c.Level = level
	r := w.onCreate(c)
	for k, v := range r.Stats {
		c.Stats[k] = v
	}
	c.Experience = w.xpToLevel(level)
	for _, proto := range w.standardKit(level) {
		if slot := proto.WearSlot(); slot != "" {
			c.Equipment[slot] = item.New(proto)
		}
	}
	w.recalc(c)
	c.Health, c.Mana = c.HealthMax, c.ManaMax
	return c
}

type simFightResult struct {
	rounds, dmgA, dmgB, swingsA, swingsB int
}

// simFight runs rounds until someone drops or the cap is hit, using the
// same swing meter as live combat. Damage is applied directly; no
// messages, no death handling.
func (w *World) simFight(a, b *Character) simFightResult {
	var f simFightResult
	a.Fighting, b.Fighting = b, a
	a.swing, b.swing = 0, 0
	swing := func(att, def *Character, round int, dmg, swings *int) {
		att.swing += att.Speed
		for att.swing >= 1 && def.Health > 0 {
			att.swing--
			*swings++
			r := w.resolveAttack(att, def, att.Equipment["wield"], round)
			if r.Hit {
				def.Health -= r.Damage
				*dmg += r.Damage
			}
		}
	}
	for f.rounds = 1; f.rounds <= simMaxRounds; f.rounds++ {
		swing(a, b, f.rounds, &f.dmgA, &f.swingsA)
		if b.Health <= 0 {
			return f
		}
		swing(b, a, f.rounds, &f.dmgB, &f.swingsB)
		if a.Health <= 0 {
			return f
		}
		ta, tb := w.onTick(a), w.onTick(b)
		a.Health = clamp(a.Health+ta.HealthDelta, 0, a.HealthMax)
		b.Health = clamp(b.Health+tb.HealthDelta, 0, b.HealthMax)
	}
	f.rounds = simMaxRounds
	return f
}

func copyStats(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cmdSimulate: simulate <mobvnum | me | fighter[:level]> <mobvnum> [fights] [seed]
func cmdSimulate(w *World, p *Player, args string) {
	fields := strings.Fields(args)
	if len(fields) < 2 {
		p.Send("Syntax: simulate <mob vnum | me | fighter[:level]> <mob vnum> [fights] [seed]\n")
		return
	}
	var makeA func() *Character
	nameA := ""
	switch {
	case fields[0] == "me":
		makeA = func() *Character { return w.cloneForSim(p.Character) }
		nameA = "you"
	case strings.HasPrefix(fields[0], "fighter"):
		level := 1
		if _, lv, ok := strings.Cut(fields[0], ":"); ok {
			n, err := strconv.Atoi(lv)
			if err != nil || n < 1 {
				p.Send("Fighter level must be a positive number.\n")
				return
			}
			level = n
		}
		makeA = func() *Character { return w.simFighter(level) }
		nameA = "a level " + itoa(level) + " fighter"
	default:
		v, err := strconv.Atoi(fields[0])
		protoA := w.content.Mobs[v]
		if err != nil || protoA == nil {
			p.Send("No such mob: " + output.Escape(fields[0]) + "\n")
			return
		}
		makeA = func() *Character { return w.simCharacter(protoA) }
		nameA = protoA.Name
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
	res := w.simulate(makeA, protoB, n, seed)
	var b strings.Builder
	b.WriteString("Simulated " + itoa(res.Fights) + " fights: " + output.Escape(nameA) + " vs " + output.Escape(protoB.Name) + "\n")
	b.WriteString("  A wins " + pct(res.AWins, res.Fights) + "  B wins " + pct(res.BWins, res.Fights) + "  draws " + pct(res.Draws, res.Fights) + "\n")
	b.WriteString("  rounds: mean " + ftoa(res.MeanRound) + "  sd " + ftoa(res.SDRound) + "  min " + itoa(res.MinRound) + "  max " + itoa(res.MaxRound) + "\n")
	b.WriteString("  damage per fight: A " + ftoa(res.MeanDmgA) + " over " + ftoa(res.SwingsA) + " swings  B " + ftoa(res.MeanDmgB) + " over " + ftoa(res.SwingsB) + " swings\n")
	if res.AWins > 0 {
		b.WriteString("  A health left when winning: mean " + pctf(res.MeanLeftA) + "  sd " + pctf(res.SDLeftA) + "\n")
	}
	p.Send(b.String())
}

func pct(n, total int) string {
	if total == 0 {
		return "0%"
	}
	return strconv.FormatFloat(100*float64(n)/float64(total), 'f', 1, 64) + "%"
}

func pctf(f float64) string { return strconv.FormatFloat(100*f, 'f', 1, 64) + "%" }
