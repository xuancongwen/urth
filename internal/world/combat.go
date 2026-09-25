package world

import (
	"strconv"
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
)

// Combat orchestration: who is fighting whom, when swings happen, what
// death does. Every number comes from a rule hook; nothing here decides
// how much anything hurts.

const maxFleeTries = 3

// cmdKill starts a fight with a character in the room.
func cmdKill(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Kill whom?\n")
		return
	}
	if p.Fighting != nil {
		p.Send("You are already fighting!\n")
		return
	}
	target := w.findCharacter(p.Room, p.Character, args)
	if target == nil {
		p.Send("They aren't here.\n")
		return
	}
	if target.mob != nil && target.mob.Proto.HasFlag("peaceful") {
		p.Send("You can't bring yourself to attack " + output.Escape(target.Name) + ".\n")
		return
	}
	w.startFight(p.Character, target)
	w.attackRound(p.Character, target)
}

// startFight makes att fight def, and def fight back if idle. Swing meters
// start empty so the first round is a clean one.
func (w *World) startFight(att, def *Character) {
	att.Fighting = def
	att.swing = 0
	if def.Fighting == nil {
		def.Fighting = att
		def.swing = 0
	}
}

// stopFighting ends c's fight and any fight aimed at c.
func (w *World) stopFighting(c *Character, both bool) {
	c.Fighting = nil
	if !both {
		return
	}
	for _, o := range w.allCharacters() {
		if o.Fighting == c {
			o.Fighting = nil
		}
	}
}

// allCharacters returns every player and mob in the world.
func (w *World) allCharacters() []*Character {
	var out []*Character
	for _, p := range w.players {
		if p.State == StatePlaying {
			out = append(out, p.Character)
		}
	}
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			out = append(out, m.Character)
		}
	}
	return out
}

// violence runs one combat round for everyone fighting. Each combatant's
// swing meter gains its speed; every whole point is one swing, so speed 2
// swings twice a round and speed 0.5 swings every other round.
func (w *World) violence() {
	for _, c := range w.allCharacters() {
		def := c.Fighting
		if def == nil {
			continue
		}
		if def.Room != c.Room || c.Room == nil || def.Health <= 0 {
			c.Fighting = nil
			continue
		}
		c.swing += c.Speed
		for c.swing >= 1 {
			c.swing--
			if c.Fighting != def || def.Health <= 0 || c.Health <= 0 {
				break
			}
			w.attackRound(c, def)
		}
	}
}

// attackRound is one swing: ask the rules, apply, narrate, handle death.
// Messages name the weapon's verb: "Your slash hits a guard. [12]".
func (w *World) attackRound(att, def *Character) {
	weapon := att.Equipment["wield"]
	w.rounds++
	r := w.resolveAttack(att, def, weapon, int(w.roundCount))
	verb := output.Escape(r.Verb)
	if !r.Hit {
		switch r.Stage {
		case "dodge":
			w.act("$N dodges your "+verb+".", att, def, "", toChar)
			w.act("You dodge $n's "+verb+".", att, def, "", toVict)
			w.act("$N dodges $n's "+verb+".", att, def, "", toNotVict)
		case "block":
			w.act("$N blocks your "+verb+".", att, def, "", toChar)
			w.act("You block $n's "+verb+".", att, def, "", toVict)
			w.act("$N blocks $n's "+verb+".", att, def, "", toNotVict)
		default:
			w.act("Your "+verb+" misses $N.", att, def, "", toChar)
			w.act("$n's "+verb+" misses you.", att, def, "", toVict)
			w.act("$n's "+verb+" misses $N.", att, def, "", toNotVict)
		}
		return
	}
	crit := ""
	if r.Crit {
		crit = " {Y}Critical!{x}"
	}
	dmg := itoa(r.Damage)
	w.act("Your "+verb+" hits $N. {W}["+dmg+"]{x}"+crit, att, def, "", toChar)
	w.act("$n's "+verb+" hits you. {R}["+dmg+"]{x}"+crit, att, def, "", toVict)
	w.act("$n's "+verb+" hits $N."+crit, att, def, "", toNotVict)
	def.Health -= r.Damage
	if def.Health <= 0 {
		w.die(def, att)
	}
}

// die handles a character reaching zero health (docs/RULES.md 4.6). Both
// players and mobs leave a corpse holding everything they carried and
// wore. Mobs are removed; players lose a capped slice of experience and
// respawn at the start room.
func (w *World) die(victim, killer *Character) {
	w.act("$n is DEAD!!", victim, nil, "", toRoom)
	victim.Send("{R}You have been KILLED!!{x}\n")
	w.stopFighting(victim, true)
	rules := w.deathRules()
	w.makeCorpse(victim, rules.CorpseRounds)

	if victim.mob != nil {
		m := victim.mob
		w.removeMobFromRoom(m)
		if killer != nil && killer.player != nil {
			w.grantXP(killer, w.xpForKill(killer, victim))
		}
		return
	}

	p := victim.player
	if loss := w.deathXPLoss(victim, rules); loss > 0 {
		victim.Experience -= loss
		victim.Send("You lose {C}" + itoa(loss) + "{x} experience points.\n")
	}
	start, _ := w.content.Rooms.Get(w.cfg.World.StartRoom)
	w.recalc(victim)
	victim.Health = max(1, int(float64(victim.HealthMax)*rules.RespawnHealth))
	victim.Mana = victim.ManaMax
	if start != nil && victim.Room != start {
		victim.Room = start
		w.act("$n appears, looking shaken.", victim, nil, "", toRoom)
	}
	w.look(p)
	w.save(p)
}

// deathXPLoss is the experience a death costs: a fraction of the total,
// capped at a fraction of the current level's cost, never crossing the
// level's threshold.
func (w *World) deathXPLoss(c *Character, rules DeathRules) int {
	if rules.XpFraction <= 0 {
		return 0
	}
	loss := int(float64(c.Experience) * rules.XpFraction)
	floor := w.xpToLevel(c.Level)
	if rules.XpLevelCap > 0 {
		levelCost := w.xpToLevel(c.Level+1) - floor
		if levelCost > 0 && levelCost < 1<<29 {
			loss = min(loss, int(float64(levelCost)*rules.XpLevelCap))
		}
	}
	if c.Experience-loss < floor {
		loss = c.Experience - floor
	}
	return max(loss, 0)
}

// makeCorpse moves everything the character carried and wore into a corpse
// in the room. The corpse decays after rounds; its contents spill out.
func (w *World) makeCorpse(c *Character, rounds int) {
	if c.Room == nil {
		return
	}
	var held []*item.Item
	for _, s := range c.equippedList() {
		held = append(held, c.Equipment[s])
	}
	held = append(held, c.Inventory...)
	c.Inventory = nil
	c.Equipment = map[item.Slot]*item.Item{}
	corpse := item.New(corpseProto(c))
	corpse.Contents = held
	corpse.Decay = max(rounds, 1)
	w.contents(c.Room).items = append(w.contents(c.Room).items, corpse)
}

// corpseProto builds the prototype for a corpse. Corpses have no vnum and
// are never saved; they exist only in room contents.
func corpseProto(c *Character) *item.Proto {
	kws := []string{"corpse"}
	kws = append(kws, c.Keywords...)
	p := &item.Proto{
		Name:        "the corpse of " + c.Name,
		Keywords:    kws,
		Description: "The corpse of " + c.Name + " lies here.",
		Look:        "It is still. Whatever it carried is still with it.",
		Type:        item.Container,
		Flags:       []string{"nopickup", "corpse"},
		Level:       c.Level,
	}
	p.ResolveStated()
	return p
}

// decayItems counts down decaying items in every room. A decayed container
// spills its contents where it lay.
func (w *World) decayItems() {
	for vnum, c := range w.rooms {
		var kept []*item.Item
		for _, it := range c.items {
			if it.Decay <= 0 {
				kept = append(kept, it)
				continue
			}
			it.Decay--
			if it.Decay > 0 {
				kept = append(kept, it)
				continue
			}
			kept = append(kept, it.Contents...)
			if r, ok := w.content.Rooms.Get(vnum); ok {
				for _, p := range w.playersIn(r) {
					p.Send(capitalize(output.Escape(it.Name())) + " crumbles into dust.\n")
				}
			}
		}
		c.items = kept
	}
}

// tickEffects counts down timed effects on everyone and recalculates
// anyone whose list changed.
func (w *World) tickEffects() {
	for _, c := range w.allCharacters() {
		before := len(c.Effects)
		c.Effects = effect.Tick(c.Effects)
		if len(c.Effects) != before {
			w.recalc(c)
		}
	}
}

// grantXP adds experience and applies any levels gained.
func (w *World) grantXP(c *Character, xp int) {
	if xp <= 0 {
		return
	}
	c.Experience += xp
	c.Send("You receive {C}" + itoa(xp) + "{x} experience points.\n")
	for c.Experience >= w.xpToLevel(c.Level+1) {
		c.Level++
		r := w.onLevel(c, c.Level)
		for k, d := range r.StatDeltas {
			if c.Stats == nil {
				c.Stats = map[string]int{}
			}
			c.Stats[k] += d
		}
		c.StatPoints += max(r.StatPoints, 0)
		w.recalc(c)
		c.Health = c.HealthMax
		c.Mana = c.ManaMax
		msg := r.Message
		if msg == "" {
			msg = "You raise a level!"
		}
		c.Send("{Y}" + output.Escape(msg) + "{x} You are now level " + itoa(c.Level) + ".\n")
		if r.StatPoints > 0 {
			c.Send("You have " + itoa(c.StatPoints) + " stat points to train.\n")
		}
		w.act("$n has gained a level.", c, nil, "", toRoom)
	}
	if c.player != nil {
		w.save(c.player)
	}
}

// cmdTrain: train | train <stat>. Spends one banked stat point.
func cmdTrain(w *World, p *Player, args string) {
	if args == "" {
		var b strings.Builder
		b.WriteString("You have " + itoa(p.StatPoints) + " stat points to train.\n")
		if len(p.Stats) > 0 {
			b.WriteString("Stats:")
			for _, k := range sortedKeys(p.Stats) {
				b.WriteString(" " + k + " " + itoa(p.Stats[k]))
			}
			b.WriteString("\n")
		}
		p.Send(b.String())
		return
	}
	if p.StatPoints <= 0 {
		p.Send("You have no stat points to train.\n")
		return
	}
	name := strings.ToLower(args)
	match := ""
	for _, k := range sortedKeys(p.Stats) {
		if strings.HasPrefix(k, name) {
			if match != "" {
				p.Send("Which stat: " + match + " or " + k + "?\n")
				return
			}
			match = k
		}
	}
	if match == "" {
		p.Send("You can't train that.\n")
		return
	}
	p.Stats[match]++
	p.StatPoints--
	w.recalc(p.Character)
	p.Send("You train " + match + " to " + itoa(p.Stats[match]) + ". " + itoa(p.StatPoints) + " points left.\n")
	w.save(p)
}

// cmdFlee tries to leave through a random exit. Whether fleeing costs
// anything is a rule to be written.
func cmdFlee(w *World, p *Player, _ string) {
	if p.Fighting == nil {
		p.Send("You aren't fighting anyone.\n")
		return
	}
	exits := p.Room.ExitList()
	for i := 0; i < maxFleeTries && len(exits) > 0; i++ {
		dir := exits[w.rng.IntN(len(exits))]
		dest, ok := w.content.Rooms.Get(p.Room.Exits[dir])
		if !ok {
			continue
		}
		w.stopFighting(p.Character, true)
		w.act("$n has fled!", p.Character, nil, "", toRoom)
		p.Room = dest
		p.Send("You flee " + dir + "!\n")
		w.act("$n arrives in a hurry.", p.Character, nil, "", toRoom)
		w.look(p)
		return
	}
	p.Send("PANIC! You couldn't escape!\n")
}

// regen applies onTick to everyone once per round.
func (w *World) regen() {
	for _, c := range w.allCharacters() {
		r := w.onTick(c)
		c.Health = clamp(c.Health+r.HealthDelta, 0, c.HealthMax)
		c.Mana = clamp(c.Mana+r.ManaDelta, 0, c.ManaMax)
		if c.Health <= 0 && r.HealthDelta < 0 {
			w.die(c, nil)
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// cmdScore shows the character sheet.
func cmdScore(_ *World, p *Player, _ string) {
	var b strings.Builder
	b.WriteString("You are " + output.Escape(p.Name) + ", level " + itoa(p.Level) + ".\n")
	b.WriteString("Health " + itoa(p.Health) + "/" + itoa(p.HealthMax))
	if p.ManaMax > 0 {
		b.WriteString("  Mana " + itoa(p.Mana) + "/" + itoa(p.ManaMax))
	}
	b.WriteString("\nExperience " + itoa(p.Experience) + "  Speed " + ftoa(p.Speed) + "\n")
	if len(p.Stats) > 0 {
		b.WriteString("Stats:")
		for _, k := range sortedKeys(p.Stats) {
			b.WriteString(" " + k + " " + itoa(p.Stats[k]))
		}
		b.WriteString("\n")
	}
	if p.StatPoints > 0 {
		b.WriteString("You have " + itoa(p.StatPoints) + " stat points to train.\n")
	}
	if len(p.Effects) > 0 {
		b.WriteString("Effects:")
		for _, e := range p.Effects {
			b.WriteString(" " + output.Escape(e.Kind))
			if e.Rounds > 0 {
				b.WriteString("(" + itoa(e.Rounds) + ")")
			}
		}
		b.WriteString("\n")
	}
	if p.Fighting != nil {
		b.WriteString("You are fighting " + output.Escape(p.Fighting.Name) + ".\n")
	}
	p.Send(b.String())
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
