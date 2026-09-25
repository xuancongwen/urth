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
	target := w.findCharacter(p.Room, p.Character, args)
	if target == nil {
		p.Send("They aren't here.\n")
		return
	}
	if target.mob != nil && target.mob.Proto.HasFlag("peaceful") {
		p.Send("You can't bring yourself to attack " + output.Escape(target.Name) + ".\n")
		return
	}
	if p.Room.Safe() {
		p.Send("You cannot fight here.\n")
		return
	}
	if sameGroup(p.Character, target) {
		p.Send("You can't attack a member of your group.\n")
		return
	}
	if p.Fighting == target {
		p.Send("You are already fighting them!\n")
		return
	}
	if p.Fighting != nil {
		// Switching targets mid-fight (docs/RULES.md 4.7).
		w.act("You turn on $N.", p.Character, target, "", toChar)
		w.act("$n turns on you.", p.Character, target, "", toVict)
		w.act("$n turns on $N.", p.Character, target, "", toNotVict)
		p.Fighting = target
		if target.Fighting == nil {
			target.Fighting = p.Character
			target.swing = 0
		}
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
	w.autoAssist(att, def)
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
			// The target is gone; turn on anyone still fighting us.
			c.Fighting = nil
			for _, e := range w.enemiesOf(c) {
				if e.Health > 0 {
					c.Fighting = e
					def = e
					break
				}
			}
			if c.Fighting == nil {
				continue
			}
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
	if def.casting != nil && def.casting.spell.InterruptOnDamage {
		w.interruptCast(def, "Your "+output.Escape(def.casting.spell.Name)+" is interrupted!")
	}
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

	victim.casting = nil
	if victim.mob != nil {
		m := victim.mob
		room := m.Room
		w.removeMobFromRoom(m)
		if killer != nil && killer.player != nil {
			// Every grouped player here earns the kill as if alone (4.7).
			for _, member := range groupOf(killer) {
				if member.Room == room && member.player != nil {
					w.grantXP(member, w.xpForKill(member, victim))
				}
			}
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
	if c.Silver > 0 {
		held = append(held, coinPile(c.Silver))
		c.Silver = 0
	}
	if c.mob != nil {
		if silver := w.moneyFor(c); silver > 0 {
			held = append(held, coinPile(silver))
		}
	}
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
		c.FeatPoints += max(r.FeatPicks, 0)
		w.ensurePassives(c)
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
		if r.FeatPicks > 0 {
			c.Send("You may choose " + plural(c.FeatPoints, "feat") + ". Type 'feat' to see them.\n")
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
		if len(r.Skills) > 0 {
			applySkillRatings(c, r.Skills)
			w.recalc(c)
		}
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

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }

// takenFeats returns the ids of the feats a character holds. A feat is
// an effect whose params carry its id under "feat".
func takenFeats(c *Character) map[string]bool {
	out := map[string]bool{}
	for _, e := range c.Effects {
		if id, ok := e.Params["feat"].(string); ok {
			out[id] = true
		}
	}
	return out
}

// featNames lists a character's feats by display name.
func (w *World) featNames(c *Character) []string {
	taken := takenFeats(c)
	var names []string
	for _, f := range w.featList() {
		if taken[f.ID] {
			names = append(names, output.Escape(f.Name))
		}
	}
	return names
}

// cmdFeat: feat | feat <name>. Lists feats with their status, or spends a
// banked pick on one (docs/RULES.md 7.2).
func cmdFeat(w *World, p *Player, args string) {
	feats := w.featList()
	if len(feats) == 0 {
		p.Send("There are no feats to learn.\n")
		return
	}
	taken := takenFeats(p.Character)
	available := func(f Feat) (bool, string) {
		if taken[f.ID] {
			return false, "learned"
		}
		if p.Level < f.Level {
			return false, "level " + itoa(f.Level)
		}
		for _, req := range f.Requires {
			if !taken[req] {
				return false, "needs " + featName(feats, req)
			}
		}
		return true, "available"
	}
	if args == "" {
		var b strings.Builder
		b.WriteString("You may choose " + plural(p.FeatPoints, "feat") + ".\n")
		for _, f := range feats {
			ok, why := available(f)
			mark := "  "
			switch {
			case taken[f.ID]:
				mark = "{G}*{x} "
			case ok:
				mark = "{Y}+{x} "
			}
			b.WriteString(mark + padRight(output.Escape(f.Name), 16) + " (" + why + ") " + output.Escape(f.Description) + "\n")
		}
		b.WriteString("{G}*{x} learned  {Y}+{x} available. Type 'feat <name>' to choose one.\n")
		p.Send(b.String())
		return
	}
	want := strings.ToLower(args)
	var match *Feat
	for i := range feats {
		f := &feats[i]
		if strings.ToLower(f.ID) == want || strings.ToLower(f.Name) == want {
			match = f
			break
		}
		if strings.HasPrefix(strings.ToLower(f.Name), want) || strings.HasPrefix(strings.ToLower(f.ID), want) {
			if match != nil {
				p.Send("Which feat: " + output.Escape(match.Name) + " or " + output.Escape(f.Name) + "?\n")
				return
			}
			match = f
		}
	}
	if match == nil {
		p.Send("There is no feat called that.\n")
		return
	}
	if ok, why := available(*match); !ok {
		switch {
		case taken[match.ID]:
			p.Send("You already have " + output.Escape(match.Name) + ".\n")
		case strings.HasPrefix(why, "level"):
			p.Send(output.Escape(match.Name) + " needs " + why + ".\n")
		default:
			p.Send(output.Escape(match.Name) + " " + why + ".\n")
		}
		return
	}
	if p.FeatPoints <= 0 {
		p.Send("You have no feat picks to spend.\n")
		return
	}
	w.grantFeat(p.Character, *match)
	p.FeatPoints--
	p.Send("You learn {Y}" + output.Escape(match.Name) + "{x}. " + output.Escape(match.Description) + "\n")
	w.save(p)
}

// grantFeat attaches a feat's effect permanently, tagged with the feat id.
// Trainers and admin commands use the same path as the pick command.
func (w *World) grantFeat(c *Character, f Feat) {
	params := map[string]any{"feat": f.ID}
	for k, v := range f.Effect.Params {
		params[k] = v
	}
	c.Effects = append(c.Effects, effect.Active{Spec: effect.Spec{Kind: f.Effect.Kind, Params: params}})
	w.recalc(c)
}

func featName(feats []Feat, id string) string {
	for _, f := range feats {
		if f.ID == id {
			return output.Escape(f.Name)
		}
	}
	return id
}

// condition describes how hurt a character looks, in 20 percent bands.
func condition(c *Character) string {
	if c.HealthMax <= 0 || c.Health <= 0 {
		return "is near death"
	}
	pct := 100 * c.Health / c.HealthMax
	switch {
	case pct >= 100:
		return "looks perfectly healthy"
	case pct >= 80:
		return "looks slightly wounded"
	case pct >= 60:
		return "looks wounded"
	case pct >= 40:
		return "looks badly wounded"
	case pct >= 20:
		return "looks gravely wounded"
	default:
		return "is near death"
	}
}

// conditionLine is the sentence shown after each round and on look.
func conditionLine(c *Character) string {
	return c.DisplayName() + " " + condition(c) + ".\n"
}

// showConditions tells every fighting player how their target looks,
// once per round.
func (w *World) showConditions() {
	for _, p := range w.players {
		if p.State == StatePlaying && p.Fighting != nil && p.Fighting.Health > 0 && p.Fighting.Room == p.Room {
			p.Send(conditionLine(p.Fighting))
		}
	}
}

// cmdConsider: consider <target>. The verdict comes from the rules'
// consider hook when present, else from the level gap alone.
func cmdConsider(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Consider whom?\n")
		return
	}
	target := w.findCharacter(p.Room, p.Character, args)
	if target == nil {
		p.Send("They aren't here.\n")
		return
	}
	if target.mob != nil && target.mob.Proto.HasFlag("peaceful") {
		p.Send("You couldn't bring yourself to attack " + output.Escape(target.Name) + ".\n")
		return
	}
	verdict := w.consider(p.Character, target)
	p.Send(output.Escape(verdict) + "\n" + conditionLine(target))
}

// consider asks the rules for a verdict on a fight. Default: by level gap.
func (w *World) consider(me, target *Character) string {
	var verdict string
	if w.callOptional("consider", &verdict, w.view(me), w.view(target)) && verdict != "" {
		return verdict
	}
	switch gap := target.Level - me.Level; {
	case gap <= -5:
		return "You could do it with a needle."
	case gap <= -3:
		return "Easy."
	case gap <= 0:
		return "A fair fight."
	case gap == 1:
		return "You could win, with a little luck."
	case gap <= 3:
		return "Best of luck."
	case gap <= 4:
		return "Death will thank you for your gift."
	default:
		return "You ARE mad!"
	}
}
