package world

import (
	"strings"

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

// startFight makes att fight def, and def fight back if idle.
func (w *World) startFight(att, def *Character) {
	att.Fighting = def
	if def.Fighting == nil {
		def.Fighting = att
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

// violence runs one combat round for everyone fighting.
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
		for i := 0; i < c.AttacksPerRound; i++ {
			if c.Fighting != def || def.Health <= 0 {
				break
			}
			w.attackRound(c, def)
		}
	}
}

// attackRound is one swing: ask the rules, apply, narrate, handle death.
func (w *World) attackRound(att, def *Character) {
	weapon := att.Equipment["wield"]
	w.rounds++
	r := w.resolveAttack(att, def, weapon, int(w.roundCount))
	with := "your fists"
	withOthers := escapeName(att.Name) + "'s fists"
	if weapon != nil {
		with = output.Escape(weapon.Name())
		withOthers = with
	}
	if !r.Hit {
		w.act("You miss $N with $t.", att, def, with, toChar)
		w.act("$n misses you with $t.", att, def, withOthers, toVict)
		w.act("$n misses $N with $t.", att, def, withOthers, toNotVict)
		return
	}
	crit := ""
	if r.Crit {
		crit = " {Y}Critical!{x}"
	}
	verb := output.Escape(r.Verb)
	dmg := itoa(r.Damage)
	w.act("You "+verb+" $N with $t. {W}["+dmg+"]{x}"+crit, att, def, with, toChar)
	w.act("$n "+thirdPerson(verb)+" you with $t. {R}["+dmg+"]{x}"+crit, att, def, withOthers, toVict)
	w.act("$n "+thirdPerson(verb)+" $N with $t."+crit, att, def, withOthers, toNotVict)
	def.Health -= r.Damage
	if def.Health <= 0 {
		w.die(def, att)
	}
}

// thirdPerson turns "hit" into "hits" for the observer messages.
func thirdPerson(verb string) string {
	switch {
	case strings.HasSuffix(verb, "s") || strings.HasSuffix(verb, "sh") || strings.HasSuffix(verb, "ch"):
		return verb + "es"
	default:
		return verb + "s"
	}
}

// die handles a character reaching zero health. Mobs are removed and drop
// what they carried; players respawn at the start room. What death costs
// is a rule to be written; for now it costs nothing.
func (w *World) die(victim, killer *Character) {
	w.act("$n is DEAD!!", victim, nil, "", toRoom)
	victim.Send("{R}You have been KILLED!!{x}\n")
	w.stopFighting(victim, true)

	if victim.mob != nil {
		m := victim.mob
		c := w.contents(m.Room)
		for _, s := range m.equippedList() {
			c.items = append(c.items, m.Equipment[s])
		}
		c.items = append(c.items, m.Inventory...)
		if len(m.Inventory)+len(m.Equipment) > 0 {
			w.act("$n's belongings fall to the ground.", victim, nil, "", toRoom)
		}
		m.Inventory = nil
		m.Equipment = map[item.Slot]*item.Item{}
		w.removeMobFromRoom(m)
		if killer != nil && killer.player != nil {
			w.grantXP(killer, w.xpForKill(killer, victim))
		}
		return
	}

	p := victim.player
	start, _ := w.content.Rooms.Get(w.cfg.World.StartRoom)
	victim.Health = victim.HealthMax
	victim.Mana = victim.ManaMax
	if start != nil && victim.Room != start {
		victim.Room = start
		w.act("$n appears, looking shaken.", victim, nil, "", toRoom)
	}
	w.look(p)
	w.save(p)
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
		w.recalc(c)
		c.Health = c.HealthMax
		c.Mana = c.ManaMax
		msg := r.Message
		if msg == "" {
			msg = "You raise a level!"
		}
		c.Send("{Y}" + output.Escape(msg) + "{x} You are now level " + itoa(c.Level) + ".\n")
		w.act("$n has gained a level.", c, nil, "", toRoom)
	}
	if c.player != nil {
		w.save(c.player)
	}
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
	b.WriteString("Health " + itoa(p.Health) + "/" + itoa(p.HealthMax) + "  Mana " + itoa(p.Mana) + "/" + itoa(p.ManaMax) + "\n")
	b.WriteString("Experience " + itoa(p.Experience) + "  Attacks per round " + itoa(p.AttacksPerRound) + "\n")
	if len(p.Stats) > 0 {
		b.WriteString("Stats:")
		for _, k := range sortedKeys(p.Stats) {
			b.WriteString(" " + k + " " + itoa(p.Stats[k]))
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
