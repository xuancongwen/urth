package world

import (
	"strings"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/output"
)

// Merchants: a mob with a "trades" list hands over items for silver,
// for other items, or for both, and buys what players bring it for half
// the item's value (a weapon or armor without one
// is valued by its level). list shows the wares, buy trades for one, sell
// trades one away.

func isMerchant(m *Mob) bool { return len(m.Proto.Trades) > 0 }

func (w *World) merchantHere(p *Player) *Mob {
	for _, m := range w.contents(p.Room).mobs {
		if isMerchant(m) {
			return m
		}
	}
	return nil
}

// tradePrice renders what a trade asks: "120 silver and a wolf pelt".
func (w *World) tradePrice(t mob.Trade) string {
	var parts []string
	if t.Silver > 0 {
		parts = append(parts, moneyString(t.Silver))
	}
	counts := map[int]int{}
	var order []int
	for _, v := range t.Items {
		if counts[v] == 0 {
			order = append(order, v)
		}
		counts[v]++
	}
	for _, v := range order {
		name := "something"
		if proto := w.content.Items[v]; proto != nil {
			name = output.Escape(proto.Name)
		}
		if counts[v] > 1 {
			name = itoa(counts[v]) + " x " + name
		}
		parts = append(parts, name)
	}
	switch len(parts) {
	case 0:
		return "nothing"
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// tradeGoods picks the inventory items that pay a trade's item price,
// one distinct item per vnum asked. ok is false if something is missing.
func tradeGoods(inv []*item.Item, want []int) ([]*item.Item, bool) {
	used := map[*item.Item]bool{}
	var goods []*item.Item
	for _, v := range want {
		var found *item.Item
		for _, it := range inv {
			if !used[it] && it.Proto.Vnum == v && len(it.Contents) == 0 {
				found = it
				break
			}
		}
		if found == nil {
			return nil, false
		}
		used[found] = true
		goods = append(goods, found)
	}
	return goods, true
}

// cmdList: list. What the merchant here offers, and its price.
func cmdList(w *World, p *Player, _ string) {
	m := w.merchantHere(p)
	if m == nil {
		p.Send("Nobody here is selling anything.\n")
		return
	}
	var b strings.Builder
	b.WriteString(m.DisplayName() + " offers:\n")
	for _, t := range m.Proto.Trades {
		proto := w.content.Items[t.Item]
		if proto == nil {
			continue
		}
		line := "  " + padRight(output.Escape(proto.Name), 34) + w.tradePrice(t)
		if proto.Level > 1 {
			line += " (level " + itoa(proto.Level) + ")"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("You have " + moneyString(p.Silver) + ".\n")
	p.Send(b.String())
}

// cmdBuy: buy <item>. Pays the trade's silver and items to the merchant.
func cmdBuy(w *World, p *Player, args string) {
	m := w.merchantHere(p)
	if m == nil {
		p.Send("Nobody here is selling anything.\n")
		return
	}
	word, _, _ := strings.Cut(strings.TrimSpace(args), " ")
	if word == "" {
		p.Send("Buy what? 'list' shows what is offered.\n")
		return
	}
	for _, t := range m.Proto.Trades {
		proto := w.content.Items[t.Item]
		if proto == nil || !item.MatchKeywords(proto.Keywords, word) {
			continue
		}
		name := output.Escape(proto.Name)
		goods, ok := tradeGoods(p.Inventory, t.Items)
		if p.Silver < t.Silver || !ok {
			w.act("$N says '"+capitalize(name)+" is "+w.tradePrice(t)+". Come back when you have it.'", p.Character, m.Character, "", toChar)
			return
		}
		p.Silver -= t.Silver
		for _, it := range goods {
			p.Inventory = item.Remove(p.Inventory, it)
		}
		p.Inventory = append(p.Inventory, item.New(proto))
		w.actItem("You trade "+w.tradePrice(t)+" to $N for $p.", p.Character, m.Character, name, "", toChar)
		w.actItem("$n buys $p from $N.", p.Character, m.Character, name, "", toNotVict)
		w.save(p)
		return
	}
	w.act("$N does not sell that.", p.Character, m.Character, "", toChar)
}

// sellPrice is what a merchant pays for an item: half its value. A weapon
// or armor with no stated value is worth ten silver a level, about what
// two mobs of its level carry.
func sellPrice(it *item.Item) int {
	if it.Proto.HasFlag(newbieFlag) {
		return 0 // sub issue gear is handed out free
	}
	value := it.Proto.Value
	if value <= 0 && (it.Proto.Type == item.Weapon || it.Proto.Type == item.Armor) {
		value = 10 * it.Proto.Level
	}
	if value <= 0 {
		return 0
	}
	return max(value/2, 1)
}

// cmdSell: sell <item>. The merchant pays half the item's value.
func cmdSell(w *World, p *Player, args string) {
	m := w.merchantHere(p)
	if m == nil {
		p.Send("Nobody here is buying anything.\n")
		return
	}
	if strings.TrimSpace(args) == "" {
		p.Send("Sell what?\n")
		return
	}
	found := item.Find(p.Inventory, item.ParseTarget(args))
	if len(found) == 0 {
		p.Send("You don't have that.\n")
		return
	}
	it := found[0]
	name := output.Escape(it.Name())
	price := sellPrice(it)
	switch {
	case it.Quest != "" || it.Proto.HasFlag("quest"):
		w.actItem("$N says 'I'll not touch $p.'", p.Character, m.Character, name, "", toChar)
		return
	case len(it.Contents) > 0:
		w.actItem("$N says 'Empty $p first.'", p.Character, m.Character, name, "", toChar)
		return
	case price == 0:
		w.actItem("$N says '$p is worth nothing to me.'", p.Character, m.Character, name, "", toChar)
		return
	}
	p.Inventory = item.Remove(p.Inventory, it)
	p.Silver += price
	w.actItem("You sell $p to $N for "+moneyString(price)+".", p.Character, m.Character, name, "", toChar)
	w.actItem("$n sells $p to $N.", p.Character, m.Character, name, "", toNotVict)
	w.save(p)
}
