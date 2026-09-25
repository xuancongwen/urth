package world

import (
	"strings"

	"urth/internal/item"
	"urth/internal/output"
)

// Object commands. Nothing here weighs, limits, or evaluates anything;
// those are rules. This is pure bookkeeping in ROM's vocabulary.

func cmdInventory(_ *World, p *Player, _ string) {
	p.Send("You are carrying:\n" + listItems(p.Inventory, "Nothing."))
}

func cmdEquipment(_ *World, p *Player, _ string) {
	var b strings.Builder
	b.WriteString("You are using:\n")
	slots := p.equippedList()
	if len(slots) == 0 {
		b.WriteString("Nothing.\n")
	}
	for _, s := range slots {
		b.WriteString(slotLabel(s) + output.Escape(p.Equipment[s].Name()) + "\n")
	}
	p.Send(b.String())
}

// listItems renders an inventory-style listing with duplicates grouped.
func listItems(list []*item.Item, empty string) string {
	if len(list) == 0 {
		return "     " + empty + "\n"
	}
	var b strings.Builder
	for _, g := range item.Group(list) {
		if g.Count > 1 {
			b.WriteString(padLeft("("+itoa(g.Count)+") ", 5))
		} else {
			b.WriteString("     ")
		}
		b.WriteString(output.Escape(g.Item.Name()) + "\n")
	}
	return b.String()
}

// cmdGet: get <item> | get all | get <item> <container> | get all <container>
func cmdGet(w *World, p *Player, args string) {
	what, from, _ := strings.Cut(args, " ")
	from = strings.TrimSpace(from)
	if what == "" {
		p.Send("Get what?\n")
		return
	}
	t := item.ParseTarget(what)
	if from == "" {
		found := item.Find(w.contents(p.Room).items, t)
		if len(found) == 0 {
			p.Send("You don't see that here.\n")
			return
		}
		for _, it := range found {
			if it.Proto.HasFlag("nopickup") {
				p.Send("You can't take " + output.Escape(it.Name()) + ".\n")
				continue
			}
			w.contents(p.Room).items = item.Remove(w.contents(p.Room).items, it)
			p.Inventory = append(p.Inventory, it)
			w.actItem("You get $p.", p.Character, nil, output.Escape(it.Name()), "", toChar)
			w.actItem("$n gets $p.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
		}
		return
	}
	container := w.findItemAround(p, from)
	if container == nil {
		p.Send("You don't see that here.\n")
		return
	}
	if !container.IsContainer() {
		p.Send(capitalize(output.Escape(container.Name())) + " is not a container.\n")
		return
	}
	found := item.Find(container.Contents, t)
	if len(found) == 0 {
		p.Send("You don't see that in " + output.Escape(container.Name()) + ".\n")
		return
	}
	for _, it := range found {
		container.Contents = item.Remove(container.Contents, it)
		p.Inventory = append(p.Inventory, it)
		w.actItem("You get $p from $t.", p.Character, nil, output.Escape(it.Name()), output.Escape(container.Name()), toChar)
		w.actItem("$n gets $p from $t.", p.Character, nil, output.Escape(it.Name()), output.Escape(container.Name()), toRoom)
	}
}

// cmdDrop: drop <item> | drop all
func cmdDrop(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Drop what?\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(args))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	for _, it := range found {
		p.Inventory = item.Remove(p.Inventory, it)
		c := w.contents(p.Room)
		c.items = append(c.items, it)
		w.actItem("You drop $p.", p.Character, nil, output.Escape(it.Name()), "", toChar)
		w.actItem("$n drops $p.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
	}
}

// cmdPut: put <item> <container>
func cmdPut(w *World, p *Player, args string) {
	what, into, _ := strings.Cut(args, " ")
	into = strings.TrimSpace(into)
	if what == "" || into == "" {
		p.Send("Put what in what?\n")
		return
	}
	container := w.findItemAround(p, into)
	if container == nil {
		p.Send("You don't see that here.\n")
		return
	}
	if !container.IsContainer() {
		p.Send(capitalize(output.Escape(container.Name())) + " is not a container.\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(what))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	for _, it := range found {
		if it == container {
			continue
		}
		p.Inventory = item.Remove(p.Inventory, it)
		container.Contents = append(container.Contents, it)
		w.actItem("You put $p in $t.", p.Character, nil, output.Escape(it.Name()), output.Escape(container.Name()), toChar)
		w.actItem("$n puts $p in $t.", p.Character, nil, output.Escape(it.Name()), output.Escape(container.Name()), toRoom)
	}
}

// cmdGive: give <item> <character>
func cmdGive(w *World, p *Player, args string) {
	what, whom, _ := strings.Cut(args, " ")
	whom = strings.TrimSpace(whom)
	if what == "" || whom == "" {
		p.Send("Give what to whom?\n")
		return
	}
	target := w.findCharacter(p.Room, p.Character, whom)
	if target == nil {
		p.Send("They aren't here.\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(what))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	for _, it := range found {
		p.Inventory = item.Remove(p.Inventory, it)
		target.Inventory = append(target.Inventory, it)
		name := output.Escape(it.Name())
		w.actItem("You give $p to $N.", p.Character, target, name, "", toChar)
		w.actItem("$n gives you $p.", p.Character, target, name, "", toVict)
		w.actItem("$n gives $p to $N.", p.Character, target, name, "", toNotVict)
	}
}

// cmdWear: wear <item> | wear all. Weapons are wielded, not worn.
func cmdWear(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Wear what?\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(args))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	all := item.ParseTarget(args).All
	for _, it := range found {
		switch it.Proto.Type {
		case item.Armor:
			w.wearItem(p, it, it.Proto.WearSlot(), "You wear $p.", "$n wears $p.")
		case item.Weapon:
			if !all {
				p.Send("You must wield that.\n")
			}
		default:
			if it.Proto.WearSlot() != "" {
				w.wearItem(p, it, it.Proto.WearSlot(), "You wear $p.", "$n wears $p.")
			} else if !all {
				p.Send("You can't wear that.\n")
			}
		}
	}
}

// cmdWield: wield <weapon>
func cmdWield(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Wield what?\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(args))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	it := found[0]
	if it.Proto.Type != item.Weapon {
		p.Send("That is not a weapon.\n")
		return
	}
	w.wearItem(p, it, "wield", "You wield $p.", "$n wields $p.")
}

// cmdHold: hold <item>
func cmdHold(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Hold what?\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(args))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	it := found[0]
	if it.Proto.Type == item.Armor {
		p.Send("You can't hold that.\n")
		return
	}
	w.wearItem(p, it, "hold", "You hold $p.", "$n holds $p.")
}

// wearItem moves an item to a slot, swapping out what was there.
func (w *World) wearItem(p *Player, it *item.Item, slot item.Slot, toSelf, toOthers string) {
	name := output.Escape(it.Name())
	if old, taken := p.Equipment[slot]; taken {
		p.unequip(slot)
		w.actItem("You stop using $p.", p.Character, nil, output.Escape(old.Name()), "", toChar)
		w.actItem("$n stops using $p.", p.Character, nil, output.Escape(old.Name()), "", toRoom)
	}
	p.equip(it, slot)
	w.actItem(toSelf, p.Character, nil, name, "", toChar)
	w.actItem(toOthers, p.Character, nil, name, "", toRoom)
	w.recalc(p.Character)
}

// cmdRemove: remove <item> | remove all
func cmdRemove(w *World, p *Player, args string) {
	if args == "" {
		p.Send("Remove what?\n")
		return
	}
	t := item.ParseTarget(args)
	var worn []*item.Item
	for _, s := range p.equippedList() {
		worn = append(worn, p.Equipment[s])
	}
	found := item.Find(worn, t)
	if len(found) == 0 {
		p.Send("You aren't using that.\n")
		return
	}
	for _, it := range found {
		slot, _ := p.slotOf(it)
		p.unequip(slot)
		w.actItem("You stop using $p.", p.Character, nil, output.Escape(it.Name()), "", toChar)
		w.actItem("$n stops using $p.", p.Character, nil, output.Escape(it.Name()), "", toRoom)
	}
	w.recalc(p.Character)
}

// findItemAround looks for one item by reference in inventory, then
// equipment, then the room. Used for containers.
func (w *World) findItemAround(p *Player, ref string) *item.Item {
	t := item.ParseTarget(ref)
	if t.All {
		return nil
	}
	if found := item.Find(p.Inventory, t); len(found) == 1 {
		return found[0]
	}
	var worn []*item.Item
	for _, s := range p.equippedList() {
		worn = append(worn, p.Equipment[s])
	}
	if found := item.Find(worn, t); len(found) == 1 {
		return found[0]
	}
	if found := item.Find(w.contents(p.Room).items, t); len(found) == 1 {
		return found[0]
	}
	return nil
}

// lookAt handles "look <target>": a character here, an item carried, worn,
// or lying here, or a container's contents with "look in <container>".
func (w *World) lookAt(p *Player, args string) {
	if rest, ok := strings.CutPrefix(args, "in "); ok {
		c := w.findItemAround(p, strings.TrimSpace(rest))
		if c == nil {
			p.Send("You don't see that here.\n")
			return
		}
		if !c.IsContainer() {
			p.Send("That is not a container.\n")
			return
		}
		p.Send(capitalize(output.Escape(c.Name())) + " contains:\n" + listItems(c.Contents, "Nothing."))
		return
	}
	if c := w.findCharacter(p.Room, p.Character, args); c != nil {
		w.act("$n looks at you.", p.Character, c, "", toVict)
		w.act("$n looks at $N.", p.Character, c, "", toNotVict)
		var b strings.Builder
		desc := "You see nothing special about " + output.Escape(c.Name) + "."
		if c.mob != nil && c.mob.Proto.Look != "" {
			desc = output.Escape(strings.TrimRight(c.mob.Proto.Look, "\n"))
		}
		b.WriteString(desc + "\n" + conditionLine(c))
		if slots := c.equippedList(); len(slots) > 0 {
			b.WriteString(c.DisplayName() + " is using:\n")
			for _, s := range slots {
				b.WriteString(slotLabel(s) + output.Escape(c.Equipment[s].Name()) + "\n")
			}
		}
		p.Send(b.String())
		return
	}
	if it := w.findItemAround(p, args); it != nil {
		look := it.Proto.Look
		if look == "" {
			look = "You see nothing special about " + it.Name() + "."
		}
		p.Send(output.Escape(strings.TrimRight(look, "\n")) + "\n" + w.itemCard(it))
		return
	}
	p.Send("You don't see that here.\n")
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = " " + s
	}
	return s
}
