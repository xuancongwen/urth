package world

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/output"
)

// Expeditions: a character sets out from a safe room for an area and
// fights a run of its mobs on paper, through the simulator's fight loop
// and the same hooks as live play, sleeping between fights when hurt. It
// comes back with experience, coin, and rolled drops (docs/RULES.md 5.3),
// or falls and comes back with nothing and a death's experience cost.
// A seed makes a run reproducible; every run reports its seed. This is
// the floor of play (2.1): auto-attack only, no skills or spells.

// ExpeditionRules is what the optional expeditionRules hook returns.
type ExpeditionRules struct {
	// LevelBand is how far from the character's level a mob may be.
	LevelBand int `json:"levelBand"`
	// Fights is the default length of a run, and MaxFights the longest.
	Fights    int `json:"fights"`
	MaxFights int `json:"maxFights"`
	// RestBelow is the health fraction under which the character sleeps
	// to full before the next fight; RetreatBelow the fraction under which
	// it turns for home after a win.
	RestBelow    float64 `json:"restBelow"`
	RetreatBelow float64 `json:"retreatBelow"`
}

func (w *World) expeditionRules(c *Character) ExpeditionRules {
	r := ExpeditionRules{LevelBand: 3, Fights: 10, MaxFights: 30, RestBelow: 0.5, RetreatBelow: 0.2}
	var got ExpeditionRules
	if w.callOptional("expeditionRules", &got, w.view(c)) {
		if got.LevelBand > 0 {
			r.LevelBand = got.LevelBand
		}
		if got.Fights > 0 {
			r.Fights = got.Fights
		}
		if got.MaxFights > 0 {
			r.MaxFights = got.MaxFights
		}
		if got.RestBelow > 0 {
			r.RestBelow = got.RestBelow
		}
		if got.RetreatBelow > 0 {
			r.RetreatBelow = got.RetreatBelow
		}
	}
	r.Fights = min(r.Fights, r.MaxFights)
	return r
}

// expeditionFoes lists the mob prototypes an area's resets place that a
// character of level could hunt there, in vnum order.
func (w *World) expeditionFoes(area string, level, band int) []*mob.Proto {
	a := w.content.Resets[area]
	if a == nil || w.content.Rooms.Areas[area].Detached {
		return nil
	}
	seen := map[int]bool{}
	var out []*mob.Proto
	for _, r := range a.Resets {
		p := w.content.Mobs[r.Mob]
		if r.Mob == 0 || p == nil || seen[p.Vnum] {
			continue
		}
		rm, ok := w.content.Rooms.Get(r.Room)
		if !ok || rm.Safe() || p.HasFlag("peaceful") || p.HasFlag("questmaster") || len(p.Teaches) > 0 || len(p.Sells) > 0 || len(p.Trades) > 0 {
			continue
		}
		lv := w.simCharacter(p).Level
		if lv < level-band || lv > level+band {
			continue
		}
		seen[p.Vnum] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Vnum < out[j].Vnum })
	return out
}

// Expedition is the outcome of a run.
type Expedition struct {
	Seed    uint64
	Log     []string
	Kills   int
	XP      int
	Silver  int
	Items   []*item.Item
	Rounds  int
	Died    bool
	HealthF float64 // health left as a fraction, when it came back
}

// runExpedition plays a run for c (which it does not change) against foes.
func (w *World) runExpedition(c *Character, foes []*mob.Proto, rules ExpeditionRules, fights int, seed uint64) Expedition {
	ex := Expedition{Seed: seed}
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	if w.scripts != nil {
		w.scripts.SetRandom(rng)
		defer w.scripts.SetRandom(w.rng)
	}
	me := w.cloneForSim(c)
	me.Health = clamp(c.Health, 1, me.HealthMax)
	for i := 1; i <= fights; i++ {
		if float64(me.Health) < rules.RestBelow*float64(me.HealthMax) {
			n := w.sleepToFull(me)
			ex.Rounds += n
			ex.Log = append(ex.Log, "You sleep for "+plural(n, "round")+".")
		}
		foe := w.simCharacter(foes[rng.IntN(len(foes))])
		f := w.simFight(me, foe)
		me.Fighting, me.swing = nil, 0
		ex.Rounds += f.rounds
		switch {
		case me.Health <= 0:
			ex.Died = true
			ex.Log = append(ex.Log, "Fight "+itoa(i)+": "+output.Escape(foe.Name)+" kills you after "+plural(f.rounds, "round")+".")
			return ex
		case foe.Health > 0:
			ex.Log = append(ex.Log, "Fight "+itoa(i)+": "+output.Escape(foe.Name)+" outlasts you; you break off.")
			continue
		}
		ex.Kills++
		ex.XP += w.xpForKill(me, foe)
		ex.Silver += w.moneyFor(foe)
		line := "Fight " + itoa(i) + ": " + output.Escape(foe.Name) + " falls in " + plural(f.rounds, "round") +
			", " + itoa(100*me.Health/max(me.HealthMax, 1)) + "% health left."
		for _, it := range w.lootDrops(foe, rng) {
			ex.Items = append(ex.Items, it)
			line += " Found " + shownName(it) + "."
		}
		ex.Log = append(ex.Log, line)
		if float64(me.Health) < rules.RetreatBelow*float64(me.HealthMax) && i < fights {
			ex.Log = append(ex.Log, "Badly hurt, you turn for home.")
			break
		}
	}
	ex.HealthF = float64(me.Health) / float64(max(me.HealthMax, 1))
	return ex
}

// sleepToFull regenerates c asleep through onTick until it is full, its
// timed effects running down meanwhile, and returns the rounds it took.
func (w *World) sleepToFull(c *Character) int {
	c.Position = posSleeping
	defer func() { c.Position = posStanding }()
	n := 0
	for ; n < 500 && c.Health < c.HealthMax; n++ {
		r := w.onTick(c)
		if r.HealthDelta <= 0 && len(c.Effects) == 0 {
			break
		}
		c.Health = clamp(c.Health+r.HealthDelta, 0, c.HealthMax)
		before := len(c.Effects)
		c.Effects = effect.Tick(c.Effects)
		if len(c.Effects) != before {
			w.recalc(c)
		}
	}
	return n
}

// cmdExpedition: expedition | expedition <area> [fights] [seed]
func cmdExpedition(w *World, p *Player, args string) {
	rules := w.expeditionRules(p.Character)
	fields := strings.Fields(strings.ToLower(args))
	if len(fields) == 0 {
		w.listExpeditions(p, rules)
		return
	}
	switch {
	case p.Fighting != nil:
		p.Send("Not while you are fighting.\n")
		return
	case p.Room == nil || !p.Room.Safe():
		p.Send("Expeditions set out from somewhere safe, like a town.\n")
		return
	case p.expeditionWait > 0:
		p.Send("You are still recovering from the last one. Rest another " + w.questMinutes(p.expeditionWait) + ".\n")
		return
	}
	area := w.matchArea(fields[0])
	if area == "" {
		p.Send("No such area. Type expedition for a list.\n")
		return
	}
	foes := w.expeditionFoes(area, p.Level, rules.LevelBand)
	if len(foes) == 0 {
		p.Send("Nothing in " + output.Escape(w.areaName(area)) + " is near your level.\n")
		return
	}
	fights := rules.Fights
	if len(fields) > 1 {
		n, err := strconv.Atoi(fields[1])
		if err != nil || n < 1 || n > rules.MaxFights {
			p.Send("Fights must be 1 to " + itoa(rules.MaxFights) + ".\n")
			return
		}
		fights = n
	}
	seed := w.rng.Uint64()
	if len(fields) > 2 {
		s, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			p.Send("Seed must be a number.\n")
			return
		}
		seed = s
	}
	ex := w.runExpedition(p.Character, foes, rules, fights, seed)
	w.finishExpedition(p, area, ex)
}

func (w *World) finishExpedition(p *Player, area string, ex Expedition) {
	var b strings.Builder
	b.WriteString("{C}You set out for " + output.Escape(w.areaName(area)) + ".{x} (seed " + strconv.FormatUint(ex.Seed, 10) + ")\n")
	for _, l := range ex.Log {
		b.WriteString("  " + l + "\n")
	}
	p.expeditionWait = max(ex.Rounds/2, 1)
	if ex.Died {
		rules := w.deathRules()
		loss := w.deathXPLoss(p.Character, rules)
		p.Experience -= loss
		p.Health = max(1, int(float64(p.HealthMax)*rules.RespawnHealth))
		b.WriteString("{R}You are carried home, empty-handed.{x} The run took " + w.questMinutes(ex.Rounds) + ".\n")
		if loss > 0 {
			b.WriteString("You lose {C}" + itoa(loss) + "{x} experience points.\n")
		}
		p.Send(b.String())
		w.save(p)
		return
	}
	p.Health = clamp(int(ex.HealthF*float64(p.HealthMax)+0.5), 1, p.HealthMax)
	p.Silver += ex.Silver
	p.Inventory = append(p.Inventory, ex.Items...)
	b.WriteString("You return after " + w.questMinutes(ex.Rounds) + ": " + plural(ex.Kills, "kill") +
		", " + moneyString(ex.Silver) + ", " + plural(len(ex.Items), "item") + ".\n")
	p.Send(b.String())
	w.grantXP(p.Character, ex.XP)
	w.save(p)
}

// listExpeditions shows the areas with something near the player's level.
func (w *World) listExpeditions(p *Player, rules ExpeditionRules) {
	var areas []string
	for a := range w.content.Resets {
		areas = append(areas, a)
	}
	sort.Strings(areas)
	var b strings.Builder
	b.WriteString("Expeditions near level " + itoa(p.Level) + ": expedition <area> [fights] [seed]\n")
	n := 0
	list := []output.Expedition{}
	safe := p.Room != nil && p.Room.Safe()
	for _, a := range areas {
		foes := w.expeditionFoes(a, p.Level, rules.LevelBand)
		if len(foes) == 0 {
			continue
		}
		n++
		lo, hi := 1<<30, 0
		for _, f := range foes {
			lv := w.simCharacter(f).Level
			lo, hi = min(lo, lv), max(hi, lv)
		}
		list = append(list, output.Expedition{Area: a, Name: w.areaName(a), Kinds: len(foes), Low: lo, High: hi, Safe: safe, Wait: p.expeditionWait})
		levels := "level " + itoa(lo)
		if hi != lo {
			levels = "levels " + itoa(lo) + "-" + itoa(hi)
		}
		b.WriteString("  " + padRight(a, 12) + padRight(output.Escape(w.areaName(a)), 28) + plural(len(foes), "kind") + " of foe, " + levels + "\n")
	}
	if n == 0 {
		b.WriteString("  Nowhere. Nothing anywhere is near your level.\n")
	}
	p.Send(b.String())
	p.SendMsg(output.Message{Type: output.Expeditions, Data: list})
}

// matchArea finds an area directory by name or prefix.
func (w *World) matchArea(word string) string {
	var match string
	for a := range w.content.Resets {
		if a == word {
			return a
		}
		if strings.HasPrefix(a, word) && (match == "" || a < match) {
			match = a
		}
	}
	return match
}

// tickExpeditions counts down the rest owed after a run.
func (w *World) tickExpeditions() {
	for _, p := range w.players {
		if p.expeditionWait > 0 {
			p.expeditionWait--
		}
	}
}
