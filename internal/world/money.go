package world

import (
	"strconv"
	"strings"

	"urth/internal/item"
	"urth/internal/output"
)

// Money (docs/RULES.md 7.5): a wallet in silver, shown as gold and silver
// at 100 to 1, as in ROM. Coins in the world are piles, items with a Coins
// count that go into the wallet on get. Mobs drop coins into their corpse.

const silverPerGold = 100

// moneyString renders an amount of silver as gold and silver.
func moneyString(silver int) string {
	g, s := silver/silverPerGold, silver%silverPerGold
	switch {
	case silver <= 0:
		return "no coins"
	case g > 0 && s > 0:
		return itoa(g) + " gold and " + itoa(s) + " silver"
	case g > 0:
		return itoa(g) + " gold"
	default:
		return itoa(s) + " silver"
	}
}

// coinPile makes a pile of coins as an item to lie in a room or corpse.
func coinPile(silver int) *item.Item {
	name := "a pile of coins"
	if silver == 1 {
		name = "a single silver coin"
	} else if silver < 10 {
		name = "a few coins"
	}
	p := &item.Proto{
		Name: name, Keywords: []string{"coins", "pile", "silver", "gold", "money"},
		Description: capitalize(name) + " lies here.",
		Look:        "Coins.",
		Type:        item.Other, Flags: []string{"coins"},
	}
	p.ResolveStated()
	it := item.New(p)
	it.Coins = silver
	return it
}

// takeCoins moves a coin pile into a wallet. Returns false if it is not one.
func takeCoins(c *Character, it *item.Item) bool {
	if !it.Proto.HasFlag("coins") {
		return false
	}
	c.Silver += it.Coins
	return true
}

// moneyFor asks the rules what a mob carries in silver. Optional; the
// prototype's silver field wins when set.
func (w *World) moneyFor(victim *Character) int {
	if victim.mob == nil {
		return 0
	}
	if victim.mob.Proto.Silver > 0 {
		return victim.mob.Proto.Silver
	}
	var n int
	if w.callOptional("moneyFor", &n, w.view(victim)) {
		return max(n, 0)
	}
	return 0
}

// parseCoins reads "<amount> <silver|gold>" from the front of args and
// returns the silver value and the rest.
func parseCoins(args string) (int, string, bool) {
	fields := strings.Fields(args)
	if len(fields) < 2 {
		return 0, "", false
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n <= 0 {
		return 0, "", false
	}
	unit := strings.ToLower(fields[1])
	switch {
	case strings.HasPrefix("silver", unit) && len(unit) >= 1:
		return n, strings.Join(fields[2:], " "), true
	case strings.HasPrefix("gold", unit) && len(unit) >= 1:
		return n * silverPerGold, strings.Join(fields[2:], " "), true
	}
	return 0, "", false
}

// giveCoins handles "give <amount> <silver|gold> <character>".
func (w *World) giveCoins(p *Player, args string) bool {
	silver, rest, ok := parseCoins(args)
	if !ok {
		return false
	}
	if rest == "" {
		p.Send("Give it to whom?\n")
		return true
	}
	target := w.findCharacter(p.Room, p.Character, rest)
	if target == nil {
		p.Send("They aren't here.\n")
		return true
	}
	if p.Silver < silver {
		p.Send("You don't have that much.\n")
		return true
	}
	p.Silver -= silver
	target.Silver += silver
	w.act("You give $t to $N.", p.Character, target, moneyString(silver), toChar)
	w.act("$n gives you $t.", p.Character, target, moneyString(silver), toVict)
	w.act("$n gives $N some coins.", p.Character, target, "", toNotVict)
	return true
}

// dropCoins handles "drop <amount> <silver|gold>".
func (w *World) dropCoins(p *Player, args string) bool {
	silver, rest, ok := parseCoins(args)
	if !ok || rest != "" {
		return false
	}
	if p.Silver < silver {
		p.Send("You don't have that much.\n")
		return true
	}
	p.Silver -= silver
	c := w.contents(p.Room)
	c.items = append(c.items, coinPile(silver))
	w.act("You drop $t.", p.Character, nil, moneyString(silver), toChar)
	w.act("$n drops some coins.", p.Character, nil, "", toRoom)
	return true
}

func escapeMoney(silver int) string { return output.Escape(moneyString(silver)) }
