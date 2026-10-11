package world

import (
	"encoding/json"
	"math"
	"strings"

	"urth/internal/item"
	"urth/internal/output"
)

// Client state: a snapshot of the character for clients that draw the
// game instead of printing it (the tap client at /play.html). A session
// asks with "client state"; from then on it gets a State message at the
// end of any tick in which the snapshot changed.

// cmdClient: client state [on|off]
func cmdClient(w *World, p *Player, args string) {
	fields := strings.Fields(strings.ToLower(args))
	if len(fields) == 0 || fields[0] != "state" {
		p.Send("Syntax: client state [on|off]\n")
		return
	}
	p.wantState = len(fields) < 2 || fields[1] != "off"
	p.lastState = ""
	if p.wantState {
		p.Send("Client state on.\n")
	} else {
		p.Send("Client state off.\n")
	}
}

// queueState adds a State message to p's output if the snapshot differs
// from the last one sent.
func (w *World) queueState(p *Player) {
	if !p.wantState || p.State != StatePlaying || p.Room == nil {
		return
	}
	s := w.clientState(p)
	b, err := json.Marshal(s)
	if err != nil || string(b) == p.lastState {
		return
	}
	p.lastState = string(b)
	p.SendMsg(output.Message{Type: output.State, Data: s})
}

// clientState builds the snapshot.
func (w *World) clientState(p *Player) output.StateData {
	c := p.Character
	s := output.StateData{
		Name: c.Name, Level: c.Level,
		Health: c.Health, HealthMax: c.HealthMax, Mana: c.Mana, ManaMax: c.ManaMax,
		Stamina: c.Stamina, StaminaMax: c.StaminaMax,
		TNL: -1, Silver: c.Silver, Position: positionName(c),
		Enemies: []output.Combatant{}, Inventory: []output.GearItem{}, Equipment: []output.GearItem{},
		Skills: []output.Action{}, Spells: []output.Action{},
	}
	s.XP, s.LevelXP = c.Experience, w.xpToLevel(c.Level)
	if next := w.xpToLevel(c.Level + 1); next < 1<<29 {
		s.TNL = max(next-c.Experience, 0)
		s.NextXP = next
	}
	if c.Fighting != nil {
		t := w.combatant(p, c.Fighting)
		s.Target = &t
	}
	for _, e := range w.enemiesOf(c) {
		s.Enemies = append(s.Enemies, w.combatant(p, e))
	}
	if c.casting != nil {
		s.Casting = &output.Casting{Spell: c.casting.spell.Name, Rounds: c.casting.rounds}
	}
	s.StatPoints, s.FeatPoints, s.QuestPoints = c.StatPoints, c.FeatPoints, p.questPoints
	s.Stats = []output.Stat{}
	for _, k := range orderedStats(c.Stats) {
		eff := c.Stats[k]
		if v, ok := c.EffStats[k]; ok {
			eff = v
		}
		s.Stats = append(s.Stats, output.Stat{Name: k, Base: c.Stats[k], Effective: eff})
	}
	s.Room = w.stateRoom(p)
	if w.canSee(c) {
		s.Map = w.mapAround(p.Room, p)
	}
	pool := w.searchPool(p)
	for _, g := range item.Group(c.Inventory) {
		gi := gearItem(g.Item, itemRef(c.Inventory, pool, g.Item))
		gi.Slot = string(g.Item.Proto.WearSlot())
		if g.Count > 1 {
			gi.Count = g.Count
		}
		s.Inventory = append(s.Inventory, gi)
	}
	var worn []*item.Item
	for _, slot := range c.equippedList() {
		worn = append(worn, c.Equipment[slot])
	}
	for i, slot := range c.equippedList() {
		gi := gearItem(worn[i], itemRef(worn, pool, worn[i]))
		gi.Slot = string(slot)
		s.Equipment = append(s.Equipment, gi)
	}
	for _, sk := range w.skillList() {
		if sk.Passive || !knowsSkill(c, sk) {
			continue
		}
		s.Skills = append(s.Skills, output.Action{ID: sk.ID, Name: sk.Name, Command: sk.ID,
			Target: sk.Target, Cooldown: c.cooldowns["skill:"+sk.ID], Stamina: sk.Stamina})
	}
	for _, sp := range w.spellList() {
		if !knowsSpell(c, sp) {
			continue
		}
		s.Spells = append(s.Spells, output.Action{ID: sp.ID, Name: sp.Name, Command: "cast '" + sp.Name + "'",
			Target: sp.Target, Cooldown: c.cooldowns[sp.ID], Mana: sp.Mana})
	}
	return s
}

func (w *World) stateRoom(p *Player) output.StateRoom {
	r := p.Room
	sr := output.StateRoom{Vnum: r.Vnum, Name: r.Name, Exits: w.openExits(r), Safe: r.Safe(),
		Players: []string{}, Mobs: []output.Entity{}, Items: []output.Entity{}}
	if sr.Exits == nil {
		sr.Exits = []string{}
	}
	if !w.canSee(p.Character) {
		sr.Name = "Darkness"
		return sr
	}
	for _, o := range w.playersIn(r) {
		if o != p && w.canSeeChar(p.Character, o.Character) {
			sr.Players = append(sr.Players, o.Name)
		}
	}
	mobs, items := w.roomEntities(r, p)
	sr.Mobs = append(sr.Mobs, mobs...)
	sr.Items = append(sr.Items, items...)
	return sr
}

// combatant describes another character in the room, with the reference
// findCharacter would resolve to it.
func (w *World) combatant(p *Player, o *Character) output.Combatant {
	return output.Combatant{Name: o.Name, Ref: w.charRef(p, o), Health: o.Health, HealthMax: o.HealthMax}
}

func (w *World) charRef(p *Player, o *Character) string {
	word := ""
	if len(o.Keywords) > 0 {
		word = o.Keywords[0]
	}
	n := 0
	for _, c := range w.visibleCharactersIn(p.Character, p.Room) {
		if c == p.Character || !c.Matches(word) {
			continue
		}
		n++
		if c == o {
			break
		}
	}
	return ref(word, n)
}

// itemRef is the reference item.Find would resolve to it within list,
// built on the keyword of it that the fewest items in pool answer to.
// Commands like look search the bag, then worn gear, then the room, so a
// keyword shared across those ("sub" on every sub issue item) would find
// the wrong one; pool is everything such a search could reach.
func itemRef(list, pool []*item.Item, it *item.Item) string {
	word := distinctKeyword(it, pool)
	n := 0
	for _, x := range list {
		if !x.Matches(word) {
			continue
		}
		n++
		if x == it {
			break
		}
	}
	return ref(word, n)
}

// distinctKeyword picks the keyword of it matching the fewest items in pool,
// the later keyword on a tie, since names end in their noun.
func distinctKeyword(it *item.Item, pool []*item.Item) string {
	word, best := "", -1
	for _, k := range it.Proto.Keywords {
		n := 0
		for _, x := range pool {
			if x.Matches(k) {
				n++
			}
		}
		if best < 0 || n <= best {
			word, best = k, n
		}
	}
	return word
}

// searchPool is every item a look or get by reference could reach: the
// bag, worn gear, and the room, in that order.
func (w *World) searchPool(p *Player) []*item.Item {
	pool := append([]*item.Item{}, p.Inventory...)
	for _, slot := range p.equippedList() {
		pool = append(pool, p.Equipment[slot])
	}
	if p.Room != nil {
		pool = append(pool, w.contents(p.Room).items...)
	}
	return pool
}

func gearItem(it *item.Item, ref string) output.GearItem {
	p := it.Proto
	g := output.GearItem{Name: it.Name(), Ref: ref, Type: string(p.Type), Rarity: it.Rarity, Color: it.Color,
		Level: p.Level, Effects: len(p.Effects) + len(it.Effects)}
	switch p.Type {
	case item.Weapon:
		g.DamageLow, g.DamageHigh = spreadBounds(p.Resolved.Weapon.Damage, p.Resolved.Weapon.Spread)
		g.Speed = p.Resolved.Weapon.Speed
	case item.Armor:
		g.Defense = math.Round(p.Resolved.Armor.Defense*10) / 10
	}
	return g
}
