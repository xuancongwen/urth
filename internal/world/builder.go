package world

import (
	"strconv"
	"strings"
	"time"

	"urth/internal/content"
	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
	"urth/internal/room"
)

// Builder and admin commands in ROM's vocabulary. None of these are game
// rules; they move things around and show what the engine knows.

// resolveRoom turns "<vnum>", a player name, or a mob keyword into a room.
func (w *World) resolveRoom(ref string) *room.Room {
	if v, err := strconv.Atoi(ref); err == nil {
		r, _ := w.content.Rooms.Get(v)
		return r
	}
	if p := w.playingByName(canonicalName(ref), nil); p != nil && p.Room != nil {
		return p.Room
	}
	t := item.ParseTarget(ref)
	n := 0
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			if m.Matches(t.Word) {
				n++
				if n == t.Index {
					return m.Room
				}
			}
		}
	}
	return nil
}

func canonicalName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// cmdGoto: goto <vnum | player | mob>
func cmdGoto(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Goto where?\n")
		return
	}
	dest := w.resolveRoom(args)
	if dest == nil {
		p.Send("No such location.\n")
		return
	}
	w.teleport(p, dest, "$n disappears in a puff of smoke.", "$n appears in a puff of smoke.")
}

// teleport moves a player between rooms with the given messages.
func (w *World) teleport(p *Player, dest *room.Room, leave, arrive string) {
	w.stopFighting(p.Character, true)
	if p.Room != nil {
		w.act(leave, p.Character, nil, "", toRoom)
	}
	p.Room = dest
	w.act(arrive, p.Character, nil, "", toRoom)
	w.look(p)
}

// cmdAt: at <vnum | target> <command>
func cmdAt(w *World, p *Player, args string) {
	where, rest, _ := strings.Cut(args, " ")
	rest = strings.TrimSpace(rest)
	if where == "" || rest == "" {
		p.Send("Syntax: at <location> <command>\n")
		return
	}
	dest := w.resolveRoom(where)
	if dest == nil {
		p.Send("No such location.\n")
		return
	}
	original := p.Room
	p.Room = dest
	w.dispatch(p, rest)
	if w.stillConnected(p) && p.State == StatePlaying {
		p.Room = original
	}
}

// cmdStat: stat | stat room | stat <target>
func cmdStat(w *World, p *Player, args string) {
	args = strings.TrimSpace(args)
	if args == "" || args == "room" {
		p.Send(w.statRoom(p.Room))
		return
	}
	if c := w.findCharacter(p.Room, p.Character, args); c != nil {
		p.Send(w.statCharacter(c))
		return
	}
	if it := w.findItemAround(p, args); it != nil {
		p.Send(statItem(it))
		return
	}
	if o := w.playingByName(canonicalName(args), nil); o != nil {
		p.Send(w.statCharacter(o.Character))
		return
	}
	if r := w.resolveRoom(args); r != nil {
		for _, m := range w.contents(r).mobs {
			if m.Matches(item.ParseTarget(args).Word) {
				p.Send(w.statCharacter(m.Character))
				return
			}
		}
		p.Send(w.statRoom(r))
		return
	}
	p.Send("Nothing by that name.\n")
}

func (w *World) statRoom(r *room.Room) string {
	var b strings.Builder
	b.WriteString("Name: {C}" + output.Escape(r.Name) + "{x}  Vnum: " + itoa(r.Vnum) + "  Area: " + output.Escape(r.Area) + "\n")
	b.WriteString("Exits:")
	for _, d := range r.ExitList() {
		b.WriteString(" " + d + "->" + itoa(r.Exits[d]))
	}
	b.WriteString("\n")
	c := w.contents(r)
	b.WriteString("Items:")
	for _, it := range c.items {
		b.WriteString(" " + output.Escape(it.Name()) + "[" + itoa(it.Proto.Vnum) + "]")
	}
	b.WriteString("\nMobs:")
	for _, m := range c.mobs {
		b.WriteString(" " + output.Escape(m.Name) + "[" + itoa(m.Proto.Vnum) + "]")
	}
	b.WriteString("\nPlayers:")
	for _, o := range w.playersIn(r) {
		b.WriteString(" " + output.Escape(o.Name))
	}
	b.WriteString("\n")
	return b.String()
}

func (w *World) statCharacter(c *Character) string {
	var b strings.Builder
	kind := "Player"
	if c.mob != nil {
		kind = "Mob vnum " + itoa(c.mob.Proto.Vnum) + " (" + output.Escape(c.mob.Proto.Area) + ")"
	}
	b.WriteString("Name: {C}" + output.Escape(c.Name) + "{x}  " + kind + "\n")
	if c.Room != nil {
		b.WriteString("Room: " + itoa(c.Room.Vnum) + "  ")
	}
	b.WriteString("Level: " + itoa(c.Level) + "  XP: " + itoa(c.Experience) + "\n")
	b.WriteString("Health: " + itoa(c.Health) + "/" + itoa(c.HealthMax) + "  Mana: " + itoa(c.Mana) + "/" + itoa(c.ManaMax) + "  Speed: " + ftoa(c.Speed) + "  Stat points: " + itoa(c.StatPoints) + "\n")
	if len(c.Stats) > 0 {
		b.WriteString("Stats:")
		for _, k := range sortedKeys(c.Stats) {
			b.WriteString(" " + k + "=" + itoa(c.Stats[k]))
		}
		b.WriteString("\n")
	}
	if len(c.Effects) > 0 {
		b.WriteString("Effects: " + effectList(c.Effects) + "\n")
	}
	if c.mob != nil {
		pr := c.mob.Proto.Resolved
		if len(c.mob.Proto.Flags) > 0 {
			b.WriteString("Flags: " + strings.Join(c.mob.Proto.Flags, " ") + "\n")
		}
		b.WriteString("Natural (" + pr.Source + "): attack " + weaponLine(pr.Attack) + "  armor defense " + ftoa(pr.Armor.Defense) + " spread " + ftoa(pr.Armor.Spread))
		if pr.Health > 0 {
			b.WriteString("  health override " + itoa(pr.Health))
		}
		if pr.XP > 0 {
			b.WriteString("  xp override " + itoa(pr.XP))
		}
		b.WriteString("\n")
	}
	if c.Fighting != nil {
		b.WriteString("Fighting: " + output.Escape(c.Fighting.Name) + "\n")
	}
	if len(c.Inventory) > 0 {
		b.WriteString("Carrying:")
		for _, it := range c.Inventory {
			b.WriteString(" " + output.Escape(it.Name()) + "[" + itoa(it.Proto.Vnum) + "]")
		}
		b.WriteString("\n")
	}
	for _, s := range c.equippedList() {
		b.WriteString(slotLabel(s) + output.Escape(c.Equipment[s].Name()) + "[" + itoa(c.Equipment[s].Proto.Vnum) + "]\n")
	}
	return b.String()
}

func statItem(it *item.Item) string {
	p := it.Proto
	var b strings.Builder
	b.WriteString("Name: {C}" + output.Escape(p.Name) + "{x}  Vnum: " + itoa(p.Vnum) + "  Area: " + output.Escape(p.Area) + "  Instance: " + strconv.FormatUint(it.ID, 10) + "\n")
	b.WriteString("Type: " + string(p.Type) + "  Slot: " + string(p.WearSlot()) + "  Weight: " + itoa(p.Weight) + "  Value: " + itoa(p.Value) + "\n")
	b.WriteString("Keywords: " + strings.Join(p.Keywords, " ") + "\n")
	b.WriteString("Level: " + itoa(p.Level) + "  Baseline: " + output.Escape(p.Baseline) + "  Numbers from: " + p.Resolved.Source + "\n")
	if p.Type == item.Weapon {
		b.WriteString("Weapon: " + weaponLine(p.Resolved.Weapon))
		if p.Weapon != nil {
			b.WriteString("  (stated: " + weaponLine(*p.Weapon) + ")")
		}
		b.WriteString("\n")
	}
	if p.Type == item.Armor {
		b.WriteString("Armor: defense " + ftoa(p.Resolved.Armor.Defense) + " spread " + ftoa(p.Resolved.Armor.Spread))
		if p.Armor != nil {
			b.WriteString("  (stated: defense " + ftoa(p.Armor.Defense) + " spread " + ftoa(p.Armor.Spread) + ")")
		}
		b.WriteString("\n")
	}
	if len(p.Effects) > 0 || len(it.Effects) > 0 {
		b.WriteString("Effects: ")
		for _, e := range p.Effects {
			b.WriteString(output.Escape(e.Kind) + " ")
		}
		b.WriteString(effectList(it.Effects) + "\n")
	}
	if len(p.Mods) > 0 {
		b.WriteString("Mods:")
		for _, k := range sortedKeys(p.Mods) {
			b.WriteString(" " + k + "=" + itoa(p.Mods[k]))
		}
		b.WriteString("\n")
	}
	if len(p.Flags) > 0 {
		b.WriteString("Flags: " + strings.Join(p.Flags, " ") + "\n")
	}
	if it.IsContainer() {
		b.WriteString("Contains:")
		for _, c := range it.Contents {
			b.WriteString(" " + output.Escape(c.Name()) + "[" + itoa(c.Proto.Vnum) + "]")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// cmdLoad: load mob <vnum> | load obj <vnum>
func cmdLoad(w *World, p *Player, args string) {
	kind, rest, _ := strings.Cut(args, " ")
	v, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil {
		p.Send("Syntax: load mob <vnum> | load obj <vnum>\n")
		return
	}
	switch kind {
	case "mob", "m":
		proto := w.content.Mobs[v]
		if proto == nil {
			p.Send("No mob has that vnum.\n")
			return
		}
		m := w.newMob(proto)
		w.recalc(m.Character)
		m.Health, m.Mana = m.HealthMax, m.ManaMax
		w.placeMob(m, p.Room)
		w.act("$n has created $N!", p.Character, m.Character, "", toRoom)
		w.act("You create $N.", p.Character, m.Character, "", toChar)
	case "obj", "o", "item", "i":
		proto := w.content.Items[v]
		if proto == nil {
			p.Send("No item has that vnum.\n")
			return
		}
		it := item.New(proto)
		if proto.HasFlag("nopickup") {
			c := w.contents(p.Room)
			c.items = append(c.items, it)
		} else {
			p.Inventory = append(p.Inventory, it)
		}
		w.actItem("$n has created $p!", p.Character, nil, output.Escape(it.Name()), "", toRoom)
		w.actItem("You create $p.", p.Character, nil, output.Escape(it.Name()), "", toChar)
	default:
		p.Send("Syntax: load mob <vnum> | load obj <vnum>\n")
	}
}

// cmdPurge: purge | purge <target>. Without a target, every mob and item
// in the room goes; players and what they carry stay.
func cmdPurge(w *World, p *Player, args string) {
	args = strings.TrimSpace(args)
	c := w.contents(p.Room)
	if args == "" {
		for _, m := range append([]*Mob(nil), c.mobs...) {
			w.stopFighting(m.Character, true)
			w.removeMobFromRoom(m)
		}
		c.items = nil
		w.act("$n purges the room!", p.Character, nil, "", toRoom)
		p.Send("Ok.\n")
		return
	}
	if target := w.findCharacter(p.Room, p.Character, args); target != nil {
		if target.mob == nil {
			p.Send("You can't purge players.\n")
			return
		}
		w.stopFighting(target, true)
		w.removeMobFromRoom(target.mob)
		w.act("$n purges $N.", p.Character, target, "", toRoom)
		w.act("You purge $N.", p.Character, target, "", toChar)
		return
	}
	if found := item.Find(c.items, item.ParseTarget(args)); len(found) > 0 {
		for _, it := range found {
			c.items = item.Remove(c.items, it)
		}
		w.actItem("$n purges $p.", p.Character, nil, output.Escape(found[0].Name()), "", toRoom)
		w.actItem("You purge $p.", p.Character, nil, output.Escape(found[0].Name()), "", toChar)
		return
	}
	if found := item.Find(p.Inventory, item.ParseTarget(args)); len(found) > 0 {
		for _, it := range found {
			p.Inventory = item.Remove(p.Inventory, it)
		}
		w.actItem("You purge $p.", p.Character, nil, output.Escape(found[0].Name()), "", toChar)
		return
	}
	p.Send("Nothing by that name here.\n")
}

// cmdForce: force <player | all> <command>
func cmdForce(w *World, p *Player, args string) {
	whom, cmd, _ := strings.Cut(args, " ")
	cmd = strings.TrimSpace(cmd)
	if whom == "" || cmd == "" {
		p.Send("Syntax: force <player | all> <command>\n")
		return
	}
	var targets []*Player
	if strings.EqualFold(whom, "all") {
		for _, o := range w.players {
			if o.State == StatePlaying && o != p {
				targets = append(targets, o)
			}
		}
	} else if o := w.playingByName(canonicalName(whom), nil); o != nil {
		targets = append(targets, o)
	}
	if len(targets) == 0 {
		p.Send("They aren't playing.\n")
		return
	}
	for _, o := range targets {
		o.Send(output.Escape(p.Name) + " forces you to '" + output.Escape(cmd) + "'.\n")
		w.dispatch(o, cmd)
	}
	p.Send("Ok.\n")
}

// cmdRestore: restore <target | all>
func cmdRestore(w *World, p *Player, args string) {
	args = strings.TrimSpace(args)
	var targets []*Character
	switch {
	case args == "":
		p.Send("Restore whom?\n")
		return
	case strings.EqualFold(args, "all"):
		for _, o := range w.players {
			if o.State == StatePlaying {
				targets = append(targets, o.Character)
			}
		}
	default:
		if c := w.findCharacter(p.Room, p.Character, args); c != nil {
			targets = append(targets, c)
		} else if strings.EqualFold(args, "self") || strings.EqualFold(args, p.Name) {
			targets = append(targets, p.Character)
		} else if o := w.playingByName(canonicalName(args), nil); o != nil {
			targets = append(targets, o.Character)
		}
	}
	if len(targets) == 0 {
		p.Send("They aren't here.\n")
		return
	}
	for _, c := range targets {
		w.recalc(c)
		c.Health, c.Mana = c.HealthMax, c.ManaMax
		if c != p.Character {
			w.act("$n has restored you.", p.Character, c, "", toVict)
		}
	}
	p.Send("Ok.\n")
}

// cmdTransfer: transfer <player> [vnum]: bring a player here or to a room.
func cmdTransfer(w *World, p *Player, args string) {
	whom, where, _ := strings.Cut(args, " ")
	if whom == "" {
		p.Send("Syntax: transfer <player> [room vnum]\n")
		return
	}
	o := w.playingByName(canonicalName(whom), nil)
	if o == nil {
		p.Send("They aren't playing.\n")
		return
	}
	dest := p.Room
	if where = strings.TrimSpace(where); where != "" {
		if dest = w.resolveRoom(where); dest == nil {
			p.Send("No such location.\n")
			return
		}
	}
	w.teleport(o, dest, "$n disappears in a puff of smoke.", "$n arrives from a puff of smoke.")
	o.Send(output.Escape(p.Name) + " has transferred you.\n")
	p.Send("Ok.\n")
}

// cmdPeace stops every fight in the room.
func cmdPeace(w *World, p *Player, _ string) {
	for _, c := range w.charactersIn(p.Room) {
		w.stopFighting(c, true)
	}
	w.act("$n calls for peace. All fighting stops.", p.Character, nil, "", toRoom)
	p.Send("Ok.\n")
}

// cmdReload: reload [scripts] | reload area <name> | reload world
func cmdReload(w *World, p *Player, args string) {
	kind, rest, _ := strings.Cut(strings.TrimSpace(args), " ")
	rest = strings.TrimSpace(rest)
	switch kind {
	case "", "scripts":
		if w.scripts == nil {
			p.Send("Scripting is not enabled.\n")
			return
		}
		if err := w.scripts.Load(); err != nil {
			p.Send("{R}Reload failed, keeping the old scripts: " + output.Escape(err.Error()) + "{x}\n")
			return
		}
		w.hookErrors = map[string]time.Time{}
		w.resolveBaselines()
		p.Send("{G}Scripts reloaded.{x}\n")
	case "area", "world":
		if w.deps.LoadContent == nil {
			p.Send("Content reload is not available.\n")
			return
		}
		if kind == "area" && rest == "" {
			p.Send("Syntax: reload area <name>\n")
			return
		}
		if kind == "area" {
			if _, ok := w.content.Rooms.Areas[rest]; !ok {
				p.Send("No area named " + output.Escape(rest) + " is loaded. Use 'reload world' to pick up new areas.\n")
				return
			}
		}
		report, err := w.reloadContent(rest)
		if err != nil {
			p.Send("{R}Reload failed, keeping the old world: " + output.Escape(err.Error()) + "{x}\n")
			return
		}
		p.Send("{G}" + report + "{x}\n")
	default:
		p.Send("Syntax: reload [scripts | area <name> | world]\n")
	}
}

// reloadContent re-reads every area from disk, validates the whole set,
// and re-points live state at the new rooms and prototypes by vnum. If
// area is set, that area's resets run immediately. Anything standing in a
// room that no longer exists is moved to the start room; mobs whose
// prototype vanished are removed; items whose prototype vanished are
// destroyed.
func (w *World) reloadContent(area string) (string, error) {
	nc, err := w.deps.LoadContent()
	if err != nil {
		return "", err
	}
	start, ok := nc.Rooms.Get(w.cfg.World.StartRoom)
	if !ok {
		return "", errStartRoomMissing
	}
	moved, mobsGone, itemsGone := 0, 0, 0

	// Rooms: re-point players and mobs, drop contents of vanished rooms.
	for _, p := range w.players {
		if p.State != StatePlaying || p.Room == nil {
			continue
		}
		if r, ok := nc.Rooms.Get(p.Room.Vnum); ok {
			p.Room = r
		} else {
			w.stopFighting(p.Character, true)
			p.Room = start
			p.Send("{Y}The ground shifts beneath you.{x}\n")
			moved++
		}
	}
	for vnum, c := range w.rooms {
		r, ok := nc.Rooms.Get(vnum)
		if !ok {
			mobsGone += len(c.mobs)
			itemsGone += len(c.items)
			delete(w.rooms, vnum)
			continue
		}
		kept := c.mobs[:0]
		for _, m := range c.mobs {
			proto, ok := nc.Mobs[m.Proto.Vnum]
			if !ok {
				w.stopFighting(m.Character, true)
				mobsGone++
				continue
			}
			m.Proto = proto
			m.Name, m.Keywords = proto.Name, proto.Keywords
			m.Room = r
			kept = append(kept, m)
		}
		c.mobs = kept
	}

	// Items everywhere: rebind prototypes by vnum.
	rebind := func(list []*item.Item) []*item.Item {
		var out []*item.Item
		var walk func(list []*item.Item) []*item.Item
		walk = func(list []*item.Item) []*item.Item {
			var keep []*item.Item
			for _, it := range list {
				proto, ok := nc.Items[it.Proto.Vnum]
				if !ok {
					itemsGone++
					continue
				}
				it.Proto = proto
				it.Contents = walk(it.Contents)
				keep = append(keep, it)
			}
			return keep
		}
		out = walk(list)
		return out
	}
	for _, c := range w.rooms {
		c.items = rebind(c.items)
		for _, m := range c.mobs {
			m.Inventory = rebind(m.Inventory)
			for slot, it := range m.Equipment {
				if kept := rebind([]*item.Item{it}); len(kept) == 0 {
					delete(m.Equipment, slot)
				}
			}
		}
	}
	for _, p := range w.players {
		p.Inventory = rebind(p.Inventory)
		for slot, it := range p.Equipment {
			if kept := rebind([]*item.Item{it}); len(kept) == 0 {
				delete(p.Equipment, slot)
			}
		}
	}

	w.content = nc
	w.resolveBaselines()

	// Areas: keep countdowns for areas that still exist, add new ones.
	fresh := map[string]*areaState{}
	for name, r := range nc.Resets {
		if old, ok := w.areas[name]; ok {
			old.resets = r
			fresh[name] = old
		} else {
			a := &areaState{dir: name, resets: r, roundsLeft: w.roundsFor(r.IntervalSeconds)}
			fresh[name] = a
			w.resetArea(a)
		}
	}
	w.areas = fresh
	if a, ok := w.areas[area]; ok {
		w.resetArea(a)
	}
	for _, c := range w.allCharacters() {
		w.recalc(c)
	}
	w.log.Warn("content reloaded", "area", area, "rooms", len(nc.Rooms.Rooms), "items", len(nc.Items), "mobs", len(nc.Mobs),
		"players_moved", moved, "mobs_removed", mobsGone, "items_removed", itemsGone)
	what := "World"
	if area != "" {
		what = "Area " + area
	}
	return what + " reloaded: " + itoa(len(nc.Rooms.Rooms)) + " rooms, " + itoa(len(nc.Items)) + " items, " + itoa(len(nc.Mobs)) + " mobs; " +
		itoa(moved) + " players moved, " + itoa(mobsGone) + " mobs and " + itoa(itemsGone) + " items removed.", nil
}

var errStartRoomMissing = contentError("the start room no longer exists")

type contentError string

func (e contentError) Error() string { return string(e) }

// contentLoader is what Deps.LoadContent must satisfy.
var _ = content.Load

func weaponLine(s item.WeaponSpec) string {
	line := "damage " + ftoa(s.Damage) + " spread " + ftoa(s.Spread) + " speed " + ftoa(s.Speed)
	if s.Verb != "" {
		line += " verb " + output.Escape(s.Verb)
	}
	if s.Kind != "" {
		line += " kind " + output.Escape(s.Kind)
	}
	if s.Hands > 0 {
		line += " hands " + itoa(s.Hands)
	}
	return line
}

func effectList(list []effect.Active) string {
	var parts []string
	for _, e := range list {
		s := output.Escape(e.Kind)
		if e.Rounds > 0 {
			s += "(" + itoa(e.Rounds) + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}
