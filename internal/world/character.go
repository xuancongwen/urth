package world

import (
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/output"
	"urth/internal/room"
)

// Character is what players and mobs have in common: a name, a place, and
// things carried or worn. Rules will hang stats off it later.
type Character struct {
	Name     string
	Keywords []string
	Room     *room.Room

	// Sheet. Base stats are a free-form map because the stat set is a
	// rules decision (docs/RULES.md 3.2); maxima come from derivedStats.
	Level      int
	Experience int
	Stats      map[string]int
	// EffStats are the stats after gear and effects, as derivedStats
	// reports them; nil when the rules do not report them.
	EffStats map[string]int
	// StatPoints and FeatPoints are banked, unspent grants from levels;
	// train and feat spend them.
	StatPoints int
	FeatPoints int
	// Silver is the wallet; 100 silver is a gold (money.go).
	Silver    int
	Health    int
	HealthMax int
	Mana      int
	ManaMax   int
	// Speed is swings per round; swing is the meter that turns a fractional
	// speed into whole swings (docs/RULES.md 4.4).
	Speed float64
	swing float64
	// Effects on the character: feats are permanent, buffs and poisons are
	// timed. The engine stores and counts them; the rules interpret them.
	Effects []effect.Active
	// Magic access (docs/RULES.md 6.1).
	Schools []string
	Deity   string
	// casting is the spell in progress, if any; cooldowns are rounds left
	// per spell id.
	casting   *casting
	cooldowns map[string]int
	// lastSkillRound enforces one skill use per round (docs/RULES.md 2.1).
	lastSkillRound uint64

	// Fighting is the current target. Everyone whose Fighting is this
	// character is one of its attackers (enemiesOf).
	Fighting *Character
	// following and group implement parties (docs/RULES.md 4.7).
	following *Character
	group     *Group

	Inventory []*item.Item
	Equipment map[item.Slot]*item.Item

	// Exactly one of these is set.
	player *Player
	mob    *Mob
}

func newCharacter(name string, keywords []string) *Character {
	return &Character{Name: name, Keywords: keywords, Level: 1, Stats: map[string]int{}, HealthMax: 1, Health: 1, Speed: 1, Equipment: map[item.Slot]*item.Item{}}
}

// HasSchool reports whether the character has unlocked the school.
func (c *Character) HasSchool(school string) bool {
	for _, s := range c.Schools {
		if s == school {
			return true
		}
	}
	return false
}

// IsPlayer reports whether this character is a connected player.
func (c *Character) IsPlayer() bool { return c.player != nil }

// Matches reports whether a typed word refers to this character.
func (c *Character) Matches(word string) bool {
	return item.MatchKeywords(c.Keywords, word)
}

// DisplayName is the name as it appears at the start of a sentence.
func (c *Character) DisplayName() string { return capitalize(output.Escape(c.Name)) }

// Send delivers text to the player behind this character, if any.
func (c *Character) Send(text string) {
	if c.player != nil {
		c.player.Send(text)
	}
}

// Mob is a non-player character spawned from a prototype.
type Mob struct {
	*Character
	Proto *mob.Proto
	id    uint64
}

func (w *World) newMob(p *mob.Proto) *Mob {
	w.lastMobID++
	m := &Mob{Character: newCharacter(p.Name, p.Keywords), Proto: p, id: w.lastMobID}
	m.Level = p.Level
	m.Stats = copyStats(p.Resolved.Stats)
	for _, e := range p.Effects {
		m.Effects = append(m.Effects, effect.Active{Spec: e})
	}
	m.mob = m
	return m
}

// roomContents is the dynamic state of a room: what lies there and who
// stands there. Players are tracked on the Player itself; mobs here.
type roomContents struct {
	items []*item.Item
	mobs  []*Mob
}

func (w *World) contents(r *room.Room) *roomContents {
	c, ok := w.rooms[r.Vnum]
	if !ok {
		c = &roomContents{}
		w.rooms[r.Vnum] = c
	}
	return c
}

// charactersIn returns everyone in r, players first, in a stable order.
func (w *World) charactersIn(r *room.Room) []*Character {
	var out []*Character
	for _, p := range w.playersIn(r) {
		out = append(out, p.Character)
	}
	for _, m := range w.contents(r).mobs {
		out = append(out, m.Character)
	}
	return out
}

// visibleCharactersIn returns everyone in r whom viewer can see
// (visibility.go), in charactersIn's order.
func (w *World) visibleCharactersIn(viewer *Character, r *room.Room) []*Character {
	var out []*Character
	for _, c := range w.charactersIn(r) {
		if w.canSeeChar(viewer, c) {
			out = append(out, c)
		}
	}
	return out
}

// findCharacter resolves a target word to a character in the room, other
// than self. "2.guard" picks the second match. Only characters self can
// see are counted, so an unseen one cannot be named.
func (w *World) findCharacter(r *room.Room, self *Character, ref string) *Character {
	t := item.ParseTarget(ref)
	if t.All || t.Word == "" {
		return nil
	}
	n := 0
	for _, c := range w.visibleCharactersIn(self, r) {
		if c == self || !c.Matches(t.Word) {
			continue
		}
		n++
		if n == t.Index {
			return c
		}
	}
	return nil
}

// placeMob puts m in r.
func (w *World) placeMob(m *Mob, r *room.Room) {
	m.Room = r
	c := w.contents(r)
	c.mobs = append(c.mobs, m)
}

// removeMobFromRoom takes m out of its room without destroying it.
func (w *World) removeMobFromRoom(m *Mob) {
	if m.Room == nil {
		return
	}
	c := w.contents(m.Room)
	for i, x := range c.mobs {
		if x == m {
			c.mobs = append(c.mobs[:i:i], c.mobs[i+1:]...)
			break
		}
	}
	m.Room = nil
}

// countMobs returns how many live instances of proto exist.
func (w *World) countMobs(proto *mob.Proto) int {
	n := 0
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			if m.Proto == proto {
				n++
			}
		}
	}
	return n
}

// equip puts it in the slot, returning false if the slot is taken.
func (c *Character) equip(it *item.Item, slot item.Slot) bool {
	if _, taken := c.Equipment[slot]; taken {
		return false
	}
	c.Inventory = item.Remove(c.Inventory, it)
	c.Equipment[slot] = it
	return true
}

// unequip moves the item in slot back to inventory.
func (c *Character) unequip(slot item.Slot) *item.Item {
	it, ok := c.Equipment[slot]
	if !ok {
		return nil
	}
	delete(c.Equipment, slot)
	c.Inventory = append(c.Inventory, it)
	return it
}

// equippedList returns worn items in slot display order.
func (c *Character) equippedList() []item.Slot {
	var out []item.Slot
	for _, s := range item.Slots {
		if _, ok := c.Equipment[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

// slotOf finds which slot an equipped item is in.
func (c *Character) slotOf(it *item.Item) (item.Slot, bool) {
	for s, x := range c.Equipment {
		if x == it {
			return s, true
		}
	}
	return "", false
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// slotLabel renders a slot for the equipment listing, ROM style.
func slotLabel(s item.Slot) string {
	labels := map[item.Slot]string{
		"light": "<used as light>", "head": "<worn on head>", "face": "<worn on face>",
		"neck": "<worn around neck>", "shoulders": "<worn on shoulders>", "body": "<worn on body>",
		"arms": "<worn on arms>", "hands": "<worn on hands>",
		"wrist1": "<worn on left wrist>", "wrist2": "<worn on right wrist>", "belt": "<worn as belt>",
		"legs": "<worn on legs>", "feet": "<worn on feet>",
		"finger1": "<worn on left finger>", "finger2": "<worn on right finger>",
		"shield": "<worn as shield>", "wield": "<wielded>", "hold": "<held>",
	}
	if l, ok := labels[s]; ok {
		return padRight(l, 22)
	}
	return padRight("<"+string(s)+">", 22)
}
