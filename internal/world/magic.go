package world

import (
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
)

// Magic (docs/RULES.md 6): spells are defined by the rules (spellList),
// access comes from consumed totems and a great sacrifice, casting takes
// rounds and can be interrupted, materials are committed when a cast
// begins, and resolveCast decides what happens to each target. The engine
// owns the timing, the target set, the materials, and the messages.

// Spell is one entry of spellList.
type Spell struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Branch            string          `json:"branch"` // arcane or divine
	School            string          `json:"school"`
	Deity             string          `json:"deity"`
	CastRounds        int             `json:"castRounds"`
	InterruptOnDamage bool            `json:"interruptOnDamage"`
	Cooldown          int             `json:"cooldown"`
	Materials         []SpellMaterial `json:"materials"`
	Target            string          `json:"target"` // single, area, self, ally, group
	Save              string          `json:"save"`
	SaveEffect        string          `json:"saveEffect"`
	Description       string          `json:"description"`
}

// SpellMaterial is one material requirement.
type SpellMaterial struct {
	Material string `json:"material"`
	Count    int    `json:"count"`
}

// hostile reports whether the spell is aimed at enemies.
func (s Spell) hostile() bool { return s.Target == "single" || s.Target == "area" }

// casting is a spell in progress.
type casting struct {
	spell   Spell
	targets []*Character
	rounds  int
}

// castEffect is an effect a cast attaches.
type castEffect struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params"`
	Rounds int            `json:"rounds"`
}

// CastTargetResult is what resolveCast returns for one target.
type CastTargetResult struct {
	Index   int          `json:"index"`
	Damage  int          `json:"damage"`
	Heal    int          `json:"heal"`
	Saved   bool         `json:"saved"`
	Negated bool         `json:"negated"`
	Effects []castEffect `json:"effects"`
	Message string       `json:"message"`
}

// CastResult is what resolveCast returns.
type CastResult struct {
	OK            bool               `json:"ok"`
	Message       string             `json:"message"`
	Consume       []uint64           `json:"consume"`
	Targets       []CastTargetResult `json:"targets"`
	CasterEffects []castEffect       `json:"casterEffects"`
}

// spellList asks the rules which spells exist. Optional; default none.
func (w *World) spellList() []Spell {
	var list []Spell
	w.callOptional("spellList", &list)
	return list
}

// knowsSpell reports whether c has access to sp.
func knowsSpell(c *Character, sp Spell) bool {
	if sp.Branch == "divine" {
		return sp.Deity != "" && c.Deity == sp.Deity
	}
	return c.HasSchool(sp.School)
}

// findSpell matches a typed name against the spells c knows.
func (w *World) findSpell(c *Character, name string) (Spell, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	var match *Spell
	list := w.spellList()
	for i := range list {
		sp := &list[i]
		if !knowsSpell(c, *sp) {
			continue
		}
		if strings.ToLower(sp.Name) == name || strings.ToLower(sp.ID) == name {
			return *sp, true
		}
		if strings.HasPrefix(strings.ToLower(sp.Name), name) && match == nil {
			match = sp
		}
	}
	if match != nil {
		return *match, true
	}
	return Spell{}, false
}

// materialsHeld counts the items in inventory that count as material.
func materialsHeld(c *Character, material string) []*item.Item {
	var out []*item.Item
	for _, it := range c.Inventory {
		if it.Proto.Type == item.Material && it.Proto.Material == material {
			out = append(out, it)
		}
	}
	return out
}

// missingMaterials reports the first requirement c cannot meet.
func missingMaterials(c *Character, sp Spell) (SpellMaterial, bool) {
	for _, m := range sp.Materials {
		if len(materialsHeld(c, m.Material)) < max(m.Count, 1) {
			return m, true
		}
	}
	return SpellMaterial{}, false
}

// commitMaterials removes the spell's materials from c's inventory
// (docs/RULES.md 6.4: committed when the cast begins).
func commitMaterials(c *Character, sp Spell) {
	for _, m := range sp.Materials {
		held := materialsHeld(c, m.Material)
		for i := 0; i < max(m.Count, 1) && i < len(held); i++ {
			c.Inventory = item.Remove(c.Inventory, held[i])
		}
	}
}

// cmdSpells lists what the character can cast.
func cmdSpells(w *World, p *Player, _ string) {
	var b strings.Builder
	n := 0
	for _, sp := range w.spellList() {
		if !knowsSpell(p.Character, sp) {
			continue
		}
		n++
		line := padRight(output.Escape(sp.Name), 14)
		if sp.Branch == "divine" {
			line += padRight(output.Escape(sp.Deity), 14)
		} else {
			line += padRight(output.Escape(sp.School), 14)
		}
		line += "cast " + itoa(sp.CastRounds) + "  " + sp.Target
		if len(sp.Materials) > 0 {
			var mats []string
			for _, m := range sp.Materials {
				mats = append(mats, itoa(max(m.Count, 1))+" "+output.Escape(m.Material))
			}
			line += "  needs " + strings.Join(mats, ", ")
		}
		if cd := p.cooldowns[sp.ID]; cd > 0 {
			line += "  {R}(" + itoa(cd) + " rounds){x}"
		}
		b.WriteString(line + "\n")
	}
	if n == 0 {
		b.WriteString("You know no spells.\n")
	}
	if len(p.Schools) > 0 {
		b.WriteString("Schools: " + output.Escape(strings.Join(p.Schools, ", ")) + "\n")
	}
	if p.Deity != "" {
		b.WriteString("Apostle of the " + output.Escape(p.Deity) + " god.\n")
	}
	p.Send(b.String())
}

// cmdCast: cast <spell> [target] | cast '<two word spell>' [target]
func cmdCast(w *World, p *Player, args string) {
	name, rest := args, ""
	if strings.HasPrefix(args, "'") {
		if end := strings.Index(args[1:], "'"); end >= 0 {
			name, rest = args[1:end+1], strings.TrimSpace(args[end+2:])
		}
	} else {
		name, rest, _ = strings.Cut(args, " ")
		rest = strings.TrimSpace(rest)
	}
	if strings.TrimSpace(name) == "" {
		p.Send("Cast what?\n")
		return
	}
	sp, ok := w.findSpell(p.Character, name)
	if !ok {
		p.Send("You don't know that spell.\n")
		return
	}
	if p.casting != nil {
		w.interruptCast(p.Character, "You abandon "+output.Escape(p.casting.spell.Name)+".")
	}
	if cd := p.cooldowns[sp.ID]; cd > 0 {
		p.Send(output.Escape(sp.Name) + " is not ready for another " + plural(cd, "round") + ".\n")
		return
	}
	if m, missing := missingMaterials(p.Character, sp); missing {
		p.Send("You need " + plural(max(m.Count, 1), output.Escape(m.Material)) + " to cast " + output.Escape(sp.Name) + ".\n")
		return
	}
	targets, err := w.castTargets(p.Character, sp, rest)
	if err != "" {
		p.Send(err + "\n")
		return
	}
	if sp.hostile() && p.Room.Safe() {
		p.Send("You cannot fight here.\n")
		return
	}
	commitMaterials(p.Character, sp)
	if sp.CastRounds <= 0 {
		w.resolveCastNow(p.Character, sp, targets)
		return
	}
	p.casting = &casting{spell: sp, targets: targets, rounds: sp.CastRounds}
	w.act("You begin casting $t.", p.Character, nil, output.Escape(sp.Name), toChar)
	w.act("$n begins casting.", p.Character, nil, "", toRoom)
}

// castTargets builds the target set for a spell (docs/RULES.md 6.4).
func (w *World) castTargets(c *Character, sp Spell, arg string) ([]*Character, string) {
	switch sp.Target {
	case "self":
		return []*Character{c}, ""
	case "ally":
		if arg == "" || strings.EqualFold(arg, "self") {
			return []*Character{c}, ""
		}
		t := w.findCharacter(c.Room, c, arg)
		if t == nil {
			return nil, "They aren't here."
		}
		return []*Character{t}, ""
	case "group":
		return w.groupIn(c, c.Room), ""
	case "area":
		set := w.hostilesIn(c)
		if len(set) == 0 {
			return nil, "There is nobody here to cast that on."
		}
		return set, ""
	default: // single
		var t *Character
		if arg != "" {
			t = w.findCharacter(c.Room, c, arg)
			if t == nil {
				return nil, "They aren't here."
			}
		} else {
			t = c.Fighting
			if t == nil {
				return nil, "Cast it on whom?"
			}
		}
		if t.mob != nil && t.mob.Proto.HasFlag("peaceful") {
			return nil, "You can't bring yourself to attack " + output.Escape(t.Name) + "."
		}
		return []*Character{t}, ""
	}
}

// tickCasting advances every cast in progress and resolves the ones that
// complete. Targets that left or died are dropped.
func (w *World) tickCasting() {
	for _, c := range w.allCharacters() {
		cs := c.casting
		if cs == nil {
			continue
		}
		cs.rounds--
		if cs.rounds > 0 {
			continue
		}
		c.casting = nil
		var live []*Character
		for _, t := range cs.targets {
			if t.Room == c.Room && t.Health > 0 {
				live = append(live, t)
			}
		}
		if len(live) == 0 {
			c.Send("Your " + output.Escape(cs.spell.Name) + " finds no target.\n")
			continue
		}
		w.resolveCastNow(c, cs.spell, live)
	}
}

// tickCooldowns counts every cooldown down.
func (w *World) tickCooldowns() {
	for _, c := range w.allCharacters() {
		for id, n := range c.cooldowns {
			if n <= 1 {
				delete(c.cooldowns, id)
			} else {
				c.cooldowns[id] = n - 1
			}
		}
	}
}

// interruptCast cancels a cast in progress. Materials were committed at
// the start and are gone.
func (w *World) interruptCast(c *Character, why string) {
	if c.casting == nil {
		return
	}
	c.casting = nil
	c.Send("{R}" + why + "{x}\n")
	w.act("$n's spell fizzles.", c, nil, "", toRoom)
}

// resolveCastNow runs the hook and applies its result.
func (w *World) resolveCastNow(c *Character, sp Spell, targets []*Character) {
	var views []any
	for _, t := range targets {
		views = append(views, w.view(t))
	}
	var r CastResult
	if !w.call("resolveCast", &r, w.view(c), views, spellView(sp)) || !r.OK {
		msg := r.Message
		if msg == "" {
			msg = "The spell fizzles."
		}
		c.Send(output.Escape(msg) + "\n")
		return
	}
	// D17: consume by id, all or nothing.
	var consume []*item.Item
	for _, id := range r.Consume {
		found := false
		for _, it := range c.Inventory {
			if it.ID == id {
				consume = append(consume, it)
				found = true
				break
			}
		}
		if !found {
			c.Send("The spell fizzles.\n")
			return
		}
	}
	for _, it := range consume {
		c.Inventory = item.Remove(c.Inventory, it)
	}
	if c.cooldowns == nil {
		c.cooldowns = map[string]int{}
	}
	if sp.Cooldown > 0 {
		c.cooldowns[sp.ID] = sp.Cooldown
	}
	name := output.Escape(strings.ToLower(sp.Name))
	w.act("You cast {Y}$t{x}.", c, nil, output.Escape(sp.Name), toChar)
	w.act("$n casts $t.", c, nil, output.Escape(sp.Name), toRoom)
	for _, e := range r.CasterEffects {
		attachEffect(c, e)
	}
	if len(r.CasterEffects) > 0 {
		w.recalc(c)
	}
	for _, tr := range r.Targets {
		if tr.Index < 0 || tr.Index >= len(targets) {
			continue
		}
		t := targets[tr.Index]
		if t.Health <= 0 {
			continue
		}
		w.applyCastTarget(c, t, sp, name, tr)
	}
	if r.Message != "" {
		c.Send(output.Escape(r.Message) + "\n")
	}
}

func attachEffect(c *Character, e castEffect) {
	if e.Kind == "" {
		return
	}
	c.Effects = append(c.Effects, effect.Active{Spec: effect.Spec{Kind: e.Kind, Params: e.Params}, Rounds: e.Rounds})
}

// applyCastTarget applies one target's result and narrates it.
func (w *World) applyCastTarget(c, t *Character, sp Spell, name string, tr CastTargetResult) {
	if sp.hostile() {
		if t.Fighting == nil {
			t.Fighting = c
			t.swing = 0
		}
		if c.Fighting == nil {
			w.startFight(c, t)
		}
	}
	switch {
	case tr.Negated:
		w.act("$N shrugs off your "+name+".", c, t, "", toChar)
		w.act("You shrug off $n's "+name+".", c, t, "", toVict)
		w.act("$N shrugs off $n's "+name+".", c, t, "", toNotVict)
		return
	case tr.Damage > 0:
		dmg := itoa(tr.Damage)
		resist := ""
		if tr.Saved {
			resist = " partly"
		}
		w.act("Your "+name+" hits $N"+resist+". {W}["+dmg+"]{x}", c, t, "", toChar)
		w.act("$n's "+name+" hits you"+resist+". {R}["+dmg+"]{x}", c, t, "", toVict)
		w.act("$n's "+name+" hits $N"+resist+".", c, t, "", toNotVict)
		t.Health -= tr.Damage
	case tr.Heal > 0:
		healed := min(tr.Heal, t.HealthMax-t.Health)
		t.Health += healed
		amt := "{G}[" + itoa(healed) + "]{x}"
		if t == c {
			w.act("Your "+name+" heals you. "+amt, c, t, "", toChar)
			w.act("$n's "+name+" heals $n.", c, t, "", toRoom)
		} else {
			w.act("Your "+name+" heals $N. "+amt, c, t, "", toChar)
			w.act("$n's "+name+" heals you. "+amt, c, t, "", toVict)
			w.act("$n's "+name+" heals $N.", c, t, "", toNotVict)
		}
	case len(tr.Effects) > 0:
		if t == c {
			w.act("Your "+name+" takes hold.", c, t, "", toChar)
		} else {
			w.act("Your "+name+" takes hold of $N.", c, t, "", toChar)
			w.act("$n's "+name+" takes hold of you.", c, t, "", toVict)
			w.act("$n's "+name+" takes hold of $N.", c, t, "", toNotVict)
		}
	}
	if tr.Message != "" {
		c.Send(output.Escape(tr.Message) + "\n")
	}
	for _, e := range tr.Effects {
		attachEffect(t, e)
	}
	if len(tr.Effects) > 0 {
		w.recalc(t)
	}
	if t.Health <= 0 {
		w.die(t, c)
	}
}

func spellView(sp Spell) map[string]any {
	mats := []any{}
	for _, m := range sp.Materials {
		mats = append(mats, map[string]any{"material": m.Material, "count": m.Count})
	}
	return map[string]any{
		"id": sp.ID, "name": sp.Name, "branch": sp.Branch, "school": sp.School, "deity": sp.Deity,
		"castRounds": sp.CastRounds, "interruptOnDamage": sp.InterruptOnDamage, "cooldown": sp.Cooldown,
		"materials": mats, "target": sp.Target, "save": sp.Save, "saveEffect": sp.SaveEffect,
	}
}

// cmdConsume: consume <totem>. Unlocks the totem's school (docs/RULES.md 6.1).
func cmdConsume(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Consume what?\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(args))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	it := found[0]
	if it.Proto.Type != item.Totem {
		p.Send("Nothing happens.\n")
		return
	}
	school := it.Proto.School
	p.Inventory = item.Remove(p.Inventory, it)
	w.actItem("You consume $p.", p.Character, nil, output.Escape(it.Name()), "", toChar)
	w.actItem("$n consumes $p.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
	if p.HasSchool(school) {
		p.Send("You already knew the ways of " + output.Escape(school) + ".\n")
		return
	}
	p.Schools = append(p.Schools, school)
	p.Send("{Y}The ways of " + output.Escape(school) + " open to you.{x}\n")
	w.save(p)
}

// cmdSacrifice: sacrifice <item>. At a god's temple, giving up the item the
// god wants makes the character its apostle (docs/RULES.md 6.1). Anywhere,
// an item lying in the room (a corpse, most often) can be sacrificed to
// the gods, who take it away and leave a coin.
func cmdSacrifice(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Sacrifice what?\n")
		return
	}
	if p.Room.Temple != "" {
		if found := item.Find(p.Inventory, item.ParseTarget(args)); len(found) > 0 && found[0].Proto.Sacrifice == p.Room.Temple {
			w.greatSacrifice(p, found[0])
			return
		}
	}
	// Sacrifice something lying here.
	c := w.contents(p.Room)
	found := item.Find(c.items, item.ParseTarget(args))
	if len(found) == 0 {
		if len(item.Find(p.Inventory, item.ParseTarget(args))) > 0 {
			if p.Room.Temple == "" {
				p.Send("The gods take only what lies on the ground. Drop it first, or find a temple.\n")
			} else {
				p.Send("The god does not want that.\n")
			}
			return
		}
		p.Send("You don't see that here.\n")
		return
	}
	it := found[0]
	if it.Proto.HasFlag("nopickup") && !it.Proto.HasFlag("corpse") {
		p.Send("The gods do not want " + output.Escape(it.Name()) + ".\n")
		return
	}
	if len(it.Contents) > 0 {
		c.items = append(c.items, it.Contents...)
		it.Contents = nil
		w.actItem("The contents of $p spill out.", p.Character, nil, output.Escape(it.Name()), "", toChar)
		w.actItem("The contents of $p spill out.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
	}
	c.items = item.Remove(c.items, it)
	reward := max(1, it.Proto.Value/10)
	p.Silver += reward
	w.actItem("You sacrifice $p to the gods.", p.Character, nil, output.Escape(it.Name()), "", toChar)
	w.actItem("$n sacrifices $p to the gods.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
	p.Send("The gods give you " + escapeMoney(reward) + " for your sacrifice.\n")
}

// greatSacrifice is the divine path: the item the god wants, at its temple.
func (w *World) greatSacrifice(p *Player, it *item.Item) {
	p.Inventory = item.Remove(p.Inventory, it)
	w.actItem("You lay $p on the altar, and it is gone.", p.Character, nil, output.Escape(it.Name()), "", toChar)
	w.actItem("$n lays $p on the altar, and it is gone.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
	if p.Deity == p.Room.Temple {
		p.Send("The god is pleased, but you were already an apostle.\n")
		return
	}
	if p.Deity != "" {
		p.Send("You turn from the " + output.Escape(p.Deity) + " god.\n")
	}
	p.Deity = p.Room.Temple
	p.Send("{Y}You are now an apostle of the " + output.Escape(p.Deity) + " god.{x}\n")
	w.save(p)
}
