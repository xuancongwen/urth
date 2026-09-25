package world

import (
	"sort"
	"strings"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/output"
	"urth/internal/room"
	"urth/internal/store"
)

// Quests (docs/RULES.md 7.6), after ROM's questmaster: a mob flagged
// "questmaster" hands out one randomly drawn task at a time, scaled to
// the player's level and drawn from the whole live world: kill a mob, or
// recover a quest item planted somewhere in an area. Finishing in time
// pays quest points, which a mob with a "sells" list exchanges for items.
//
// The engine owns the draw, the timer, the item stamp, and the credit;
// the rules set the level band, the timers, and the reward.

// QuestRules is what the questRules hook returns: how far from the
// player's level a target may be, and the timers in rounds.
type QuestRules struct {
	LevelBand    int `json:"levelBand"`
	Rounds       int `json:"rounds"`
	Cooldown     int `json:"cooldown"`
	QuitCooldown int `json:"quitCooldown"`
}

// QuestReward is what the questReward hook returns for a drawn quest.
type QuestReward struct {
	Points  int    `json:"points"`
	Silver  int    `json:"silver"`
	XP      int    `json:"xp"`
	Message string `json:"message"`
}

func (w *World) questRules(c *Character) QuestRules {
	r := QuestRules{LevelBand: 3, Rounds: 450, Cooldown: 150, QuitCooldown: 300}
	var got QuestRules
	if w.callOptional("questRules", &got, w.view(c)) {
		if got.LevelBand > 0 {
			r.LevelBand = got.LevelBand
		}
		if got.Rounds > 0 {
			r.Rounds = got.Rounds
		}
		if got.Cooldown >= 0 {
			r.Cooldown = got.Cooldown
		}
		if got.QuitCooldown >= 0 {
			r.QuitCooldown = got.QuitCooldown
		}
	}
	return r
}

func (w *World) questReward(c *Character, q *store.SavedQuest) QuestReward {
	r := QuestReward{Points: 5 + q.Level}
	var got QuestReward
	if w.callOptional("questReward", &got, w.view(c), w.questView(q)) {
		r = got
	}
	r.Points = max(r.Points, 1)
	return r
}

// questView is the script-facing snapshot of a quest.
func (w *World) questView(q *store.SavedQuest) map[string]any {
	if q == nil {
		return nil
	}
	return map[string]any{
		"kind": q.Kind, "target": q.Target, "name": w.questTargetName(q), "level": q.Level,
		"area": q.Area, "room": q.Room, "rounds": q.Rounds, "points": q.Points, "done": q.Done,
	}
}

func isQuestmaster(m *Mob) bool { return m.Proto.HasFlag("questmaster") }
func isVendor(m *Mob) bool      { return len(m.Proto.Sells) > 0 }

func (w *World) questmasterHere(p *Player) *Character {
	for _, m := range w.contents(p.Room).mobs {
		if isQuestmaster(m) {
			return m.Character
		}
	}
	return nil
}

func (w *World) vendorHere(p *Player) *Mob {
	for _, m := range w.contents(p.Room).mobs {
		if isVendor(m) {
			return m
		}
	}
	return nil
}

// questTargetName is the target as the player will see it named.
func (w *World) questTargetName(q *store.SavedQuest) string {
	switch q.Kind {
	case "kill":
		if m, ok := w.content.Mobs[q.Target]; ok {
			return m.Name
		}
	case "fetch":
		if it, ok := w.content.Items[q.Target]; ok {
			return it.Name
		}
	}
	return "something that no longer exists"
}

func (w *World) areaName(area string) string {
	if a, ok := w.content.Rooms.Areas[area]; ok && a.Name != "" {
		return a.Name
	}
	return area
}

func (w *World) roomName(vnum int) string {
	if r, ok := w.content.Rooms.Get(vnum); ok {
		return r.Name
	}
	return "somewhere"
}

// questMinutes renders a count of rounds as minutes for the player.
func (w *World) questMinutes(rounds int) string {
	ms := rounds * max(w.cfg.Timing.RoundMs, 1)
	mins := (ms + 59999) / 60000
	if mins <= 1 {
		return "less than a minute"
	}
	return plural(mins, "minute")
}

// questTarget is a live mob a quest may be drawn against: fightable, in
// a placed area, and not one of the service mobs.
func questTarget(m *Mob, areas map[string]room.Area) bool {
	p := m.Proto
	if p.HasFlag("peaceful") || isQuestmaster(m) || isVendor(m) || len(p.Teaches) > 0 {
		return false
	}
	if m.Room == nil || m.Room.Safe() || areas[m.Room.Area].Detached {
		return false
	}
	return true
}

// questCandidates lists live mobs within band levels of level, in a
// stable order so a seeded draw repeats.
func (w *World) questCandidates(level, band int) []*Mob {
	var out []*Mob
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			if !questTarget(m, w.content.Rooms.Areas) {
				continue
			}
			if m.Level < level-band || m.Level > level+band {
				continue
			}
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// questItems lists the item prototypes fetch quests may plant.
func (w *World) questItems() []*item.Proto {
	var out []*item.Proto
	for _, p := range w.content.Items {
		if p.HasFlag("quest") {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Vnum < out[j].Vnum })
	return out
}

// questRooms lists the rooms of an area a fetch item may be planted in.
func (w *World) questRooms(area string) []*room.Room {
	var out []*room.Room
	for _, r := range w.content.Rooms.Rooms {
		if r.Area == area && !r.Safe() {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Vnum < out[j].Vnum })
	return out
}

// drawQuest picks a quest for p: a live mob near their level sets the
// area and the difficulty; half the time the task is to recover an item
// planted in that area instead of the kill. The band widens when nothing
// is near the player's level. nil means nothing could be drawn.
func (w *World) drawQuest(p *Player, rules QuestRules) *store.SavedQuest {
	var cands []*Mob
	for band := rules.LevelBand; band <= rules.LevelBand*4 && len(cands) == 0; band += rules.LevelBand {
		cands = w.questCandidates(p.Level, band)
	}
	if len(cands) == 0 {
		return nil
	}
	target := cands[w.rng.IntN(len(cands))]
	q := &store.SavedQuest{Kind: "kill", Target: target.Proto.Vnum, Level: target.Level, Room: target.Room.Vnum, Area: target.Room.Area, Rounds: rules.Rounds}
	items := w.questItems()
	rooms := w.questRooms(target.Room.Area)
	if len(items) > 0 && len(rooms) > 0 && w.rng.IntN(2) == 0 {
		proto := items[w.rng.IntN(len(items))]
		r := rooms[w.rng.IntN(len(rooms))]
		q.Kind, q.Target, q.Room = "fetch", proto.Vnum, r.Vnum
	}
	reward := w.questReward(p.Character, q)
	q.Points, q.Silver, q.XP, q.Message = reward.Points, reward.Silver, reward.XP, reward.Message
	return q
}

// plantQuestItem puts a fresh copy of the fetch target in the quest's
// room, stamped for p so nobody else can take it. It crumbles when the
// time runs out.
func (w *World) plantQuestItem(p *Player, q *store.SavedQuest) {
	proto, ok := w.content.Items[q.Target]
	if !ok {
		return
	}
	r, ok := w.content.Rooms.Get(q.Room)
	if !ok {
		return
	}
	it := item.New(proto)
	it.Quest = p.Name
	it.Decay = q.Rounds + 1
	c := w.contents(r)
	c.items = append(c.items, it)
}

// questItemHeld returns the stamped fetch target in p's inventory, or nil.
func (p *Player) questItemHeld() *item.Item {
	if p.quest == nil || p.quest.Kind != "fetch" {
		return nil
	}
	for _, it := range p.Inventory {
		if it.Quest == p.Name && it.Proto.Vnum == p.quest.Target {
			return it
		}
	}
	return nil
}

// questItemPlanted reports whether a stamped item for p lies in any room.
func (w *World) questItemPlanted(p *Player) bool {
	for _, c := range w.rooms {
		for _, it := range c.items {
			if it.Quest == p.Name {
				return true
			}
		}
	}
	return false
}

// endQuest clears p's quest and sweeps every item stamped for it out of
// the world, held or lying about, so nothing useless lingers.
func (w *World) endQuest(p *Player) {
	p.quest = nil
	var kept []*item.Item
	for _, it := range p.Inventory {
		if it.Quest != p.Name {
			kept = append(kept, it)
		}
	}
	p.Inventory = kept
	for _, c := range w.rooms {
		kept = kept[:0:0]
		for _, it := range c.items {
			if it.Quest != p.Name {
				kept = append(kept, it)
			}
		}
		c.items = kept
	}
}

// resumeQuest runs at login. Rooms are not saved, so a fetch item left
// lying about is planted again where it was.
func (w *World) resumeQuest(p *Player) {
	q := p.quest
	if q == nil {
		return
	}
	if q.Kind == "fetch" && p.questItemHeld() == nil && !w.questItemPlanted(p) {
		w.plantQuestItem(p, q)
	}
}

// tickQuests counts down quest timers and cooldowns once per round.
func (w *World) tickQuests() {
	for _, p := range w.players {
		if p.State != StatePlaying {
			continue
		}
		if p.questWait > 0 {
			p.questWait--
		}
		q := p.quest
		if q == nil {
			continue
		}
		q.Rounds--
		perMinute := 60000 / max(w.cfg.Timing.RoundMs, 1)
		switch {
		case q.Rounds <= 0:
			p.Send("{R}Your time is up. The quest is failed.{x}\n")
			w.endQuest(p)
			p.questWait = w.questRules(p.Character).Cooldown
			w.save(p)
		case q.Rounds == perMinute*5 || q.Rounds == perMinute:
			p.Send("{Y}You have " + w.questMinutes(q.Rounds) + " left on your quest.{x}\n")
		}
	}
}

// questKill credits a kill against p's quest; die calls it for every
// grouped player in the room. Any mob of the target's kind counts, so a
// target that died to someone else, or was reset away, is not a dead end.
func (w *World) questKill(p *Player, victim *Character) {
	q := p.quest
	if victim.mob == nil || q == nil || q.Done || q.Kind != "kill" || q.Target != victim.mob.Proto.Vnum {
		return
	}
	q.Done = true
	p.Send("{Y}Your quest is almost done! Return to the questmaster before your time runs out.{x}\n")
}

// canTake reports whether p may pick up it; a quest item belongs to one
// player. The message says why not.
func canTake(p *Player, it *item.Item) (bool, string) {
	if it.Quest != "" && it.Quest != p.Name {
		return false, "That belongs to " + output.Escape(it.Quest) + "'s quest."
	}
	return true, ""
}

// cmdQuest: quest | quest request | quest info | quest complete |
// quest quit | quest points | quest list | quest buy <item>
func cmdQuest(w *World, p *Player, args string) {
	sub, rest, _ := strings.Cut(strings.TrimSpace(args), " ")
	sub = strings.ToLower(sub)
	rest = strings.TrimSpace(rest)
	var name string
	for _, n := range []string{"request", "info", "complete", "quit", "points", "list", "buy"} {
		if sub != "" && strings.HasPrefix(n, sub) {
			name = n
			break
		}
	}
	switch name {
	case "request":
		w.questRequest(p)
	case "", "info":
		w.questInfo(p)
	case "complete":
		w.questComplete(p)
	case "quit":
		w.questQuit(p)
	case "points":
		p.Send("You have " + plural(p.questPoints, "quest point") + ".\n")
	case "list":
		w.questList(p)
	case "buy":
		w.questBuy(p, rest)
	default:
		p.Send("quest request | info | complete | quit | points | list | buy <item>\n")
	}
}

func (w *World) questRequest(p *Player) {
	qm := w.questmasterHere(p)
	if qm == nil {
		p.Send("There is no questmaster here.\n")
		return
	}
	if p.quest != nil {
		w.act("$N says 'Finish the quest you have first.'", p.Character, qm, "", toChar)
		return
	}
	if p.questWait > 0 {
		w.act("$N says 'Come back in "+w.questMinutes(p.questWait)+".'", p.Character, qm, "", toChar)
		return
	}
	rules := w.questRules(p.Character)
	q := w.drawQuest(p, rules)
	if q == nil {
		w.act("$N says 'I have nothing for you just now. Try again later.'", p.Character, qm, "", toChar)
		return
	}
	p.quest = q
	name := output.Escape(w.questTargetName(q))
	where := "Look in " + output.Escape(w.roomName(q.Room)) + ", in " + output.Escape(w.areaName(q.Area)) + "."
	switch q.Kind {
	case "kill":
		w.act("$N says 'Slay {Y}"+name+"{x}, who has been troubling the folk of "+output.Escape(w.areaName(q.Area))+".'", p.Character, qm, "", toChar)
		where = "It was last seen near " + output.Escape(w.roomName(q.Room)) + ", in " + output.Escape(w.areaName(q.Area)) + "."
	case "fetch":
		w.plantQuestItem(p, q)
		w.act("$N says 'Bring me {Y}"+name+"{x}. It was lost somewhere in "+output.Escape(w.areaName(q.Area))+".'", p.Character, qm, "", toChar)
	}
	p.Send(where + "\n")
	p.Send("The reward is " + plural(q.Points, "quest point") + questExtras(q) + ". You have " + w.questMinutes(q.Rounds) + ".\n")
	w.act("$n takes a quest from $N.", p.Character, qm, "", toNotVict)
	w.save(p)
}

func questExtras(q *store.SavedQuest) string {
	s := ""
	if q.Silver > 0 {
		s += " and " + escapeMoney(q.Silver)
	}
	if q.XP > 0 {
		s += " and " + itoa(q.XP) + " experience"
	}
	return s
}

func (w *World) questInfo(p *Player) {
	q := p.quest
	if q == nil {
		if p.questWait > 0 {
			p.Send("You are not on a quest. A questmaster will give you one in " + w.questMinutes(p.questWait) + ".\n")
		} else {
			p.Send("You are not on a quest. Ask a questmaster for one with 'quest request'.\n")
		}
		return
	}
	name := output.Escape(w.questTargetName(q))
	var b strings.Builder
	switch {
	case q.Done, p.questItemHeld() != nil:
		b.WriteString("You have done what was asked. Return to the questmaster and 'quest complete'.\n")
	case q.Kind == "kill":
		b.WriteString("You are on a quest to slay {Y}" + name + "{x}, last seen near " + output.Escape(w.roomName(q.Room)) + " in " + output.Escape(w.areaName(q.Area)) + ".\n")
	default:
		b.WriteString("You are on a quest to recover {Y}" + name + "{x}, lost somewhere in " + output.Escape(w.areaName(q.Area)) + ".\n")
	}
	b.WriteString("The reward is " + plural(q.Points, "quest point") + questExtras(q) + ". You have " + w.questMinutes(q.Rounds) + " left.\n")
	p.Send(b.String())
}

func (w *World) questComplete(p *Player) {
	qm := w.questmasterHere(p)
	if qm == nil {
		p.Send("There is no questmaster here.\n")
		return
	}
	q := p.quest
	if q == nil {
		w.act("$N says 'You are not on a quest.'", p.Character, qm, "", toChar)
		return
	}
	held := p.questItemHeld()
	if !q.Done && held == nil {
		w.act("$N says 'You have not finished. Get on with it.'", p.Character, qm, "", toChar)
		return
	}
	if held != nil {
		w.actItem("You hand $p to $N.", p.Character, qm, output.Escape(held.Name()), "", toChar)
		w.actItem("$n hands $p to $N.", p.Character, qm, output.Escape(held.Name()), "", toNotVict)
	}
	p.questPoints += q.Points
	p.Silver += max(q.Silver, 0)
	w.act("$N says 'Well done.' You gain {Y}"+plural(q.Points, "quest point")+"{x}"+questExtras(q)+".", p.Character, qm, "", toChar)
	if q.Message != "" {
		p.Send(output.Escape(q.Message) + "\n")
	}
	xp := q.XP
	w.endQuest(p)
	p.questWait = w.questRules(p.Character).Cooldown
	if xp > 0 {
		w.grantXP(p.Character, xp)
	}
	w.save(p)
}

func (w *World) questQuit(p *Player) {
	if p.quest == nil {
		p.Send("You are not on a quest.\n")
		return
	}
	w.endQuest(p)
	p.questWait = w.questRules(p.Character).QuitCooldown
	p.Send("You give up on your quest. A questmaster will give you another in " + w.questMinutes(p.questWait) + ".\n")
	w.save(p)
}

// saleName is the name of the item a sale offers.
func (w *World) saleProto(s mob.Sale) *item.Proto { return w.content.Items[s.Item] }

func (w *World) questList(p *Player) {
	v := w.vendorHere(p)
	if v == nil {
		p.Send("Nobody here sells anything for quest points.\n")
		return
	}
	var b strings.Builder
	b.WriteString(v.DisplayName() + " offers, for quest points:\n")
	for _, s := range v.Proto.Sells {
		proto := w.saleProto(s)
		if proto == nil {
			continue
		}
		line := "  " + padRight(output.Escape(proto.Name), 34) + padRight(itoa(s.Points)+" qp", 10)
		if proto.Level > 1 {
			line += "(level " + itoa(proto.Level) + ")"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("You have " + plural(p.questPoints, "quest point") + ".\n")
	p.Send(b.String())
}

func (w *World) questBuy(p *Player, args string) {
	v := w.vendorHere(p)
	if v == nil {
		p.Send("Nobody here sells anything for quest points.\n")
		return
	}
	if args == "" {
		p.Send("Buy what? 'quest list' shows what is offered.\n")
		return
	}
	word, _, _ := strings.Cut(args, " ")
	for _, s := range v.Proto.Sells {
		proto := w.saleProto(s)
		if proto == nil || !item.MatchKeywords(proto.Keywords, word) {
			continue
		}
		if p.questPoints < s.Points {
			w.act("$N says '"+capitalize(output.Escape(proto.Name))+" is "+plural(s.Points, "quest point")+". You have "+itoa(p.questPoints)+".'", p.Character, v.Character, "", toChar)
			return
		}
		p.questPoints -= s.Points
		it := item.New(proto)
		p.Inventory = append(p.Inventory, it)
		w.actItem("You trade "+plural(s.Points, "quest point")+" to $N for $p.", p.Character, v.Character, output.Escape(proto.Name), "", toChar)
		w.actItem("$n trades with $N for $p.", p.Character, v.Character, output.Escape(proto.Name), "", toNotVict)
		w.save(p)
		return
	}
	w.act("$N does not offer that.", p.Character, v.Character, "", toChar)
}
