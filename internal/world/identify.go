package world

import (
	"math"
	"sort"
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
)

// Every item shows its numbers on look unless its prototype carries the
// "unidentified" flag. What an effect means in words comes from the
// rules' describeEffect hook when present.

// itemCard renders an item's stats for players.
func (w *World) itemCard(it *item.Item) string {
	p := it.Proto
	if p.HasFlag("unidentified") {
		return "You can't tell anything more about it.\n"
	}
	var lines []string
	head := "Type " + string(p.Type)
	if p.Type == item.Weapon || p.Type == item.Armor {
		head += ", level " + itoa(p.Level)
	}
	if slot := p.WearSlot(); slot != "" && p.Type != item.Weapon {
		head += ", worn on " + string(slot)
	}
	if p.Weight > 0 {
		head += ", weight " + itoa(p.Weight)
	}
	lines = append(lines, head)
	if it.Rolled() {
		lines = append(lines, "Rarity "+output.Escape(it.Rarity)+".")
	}
	switch p.Type {
	case item.Weapon:
		wp := p.Resolved.Weapon
		line := "Damage " + spreadRange(wp.Damage, wp.Spread) + " per swing, speed " + trimNum(wp.Speed)
		if wp.Hands == 2 {
			line += ", two-handed"
		}
		if wp.Verb != "" {
			line += ". Your swings are " + output.Escape(pluralNoun(wp.Verb)) + "."
		}
		lines = append(lines, line)
	case item.Armor:
		ar := p.Resolved.Armor
		if ar.Defense > 0 {
			lines = append(lines, "Defense "+spreadRange(ar.Defense, 0))
		}
	case item.Material:
		lines = append(lines, "A "+output.Escape(p.Rarity)+" material: "+output.Escape(p.Material)+", used in casting.")
	case item.Totem:
		lines = append(lines, "A totem of "+output.Escape(p.School)+". Consume it to learn the school.")
	case item.Container:
		lines = append(lines, "A container.")
	case item.Light:
		switch {
		case it.Burn < 0:
			lines = append(lines, "A light that never goes out.")
		case it.Burn == 0:
			lines = append(lines, "A light, burned out.")
		default:
			lines = append(lines, "A light with about "+plural(it.Burn, "round")+" left.")
		}
	}
	if p.Sacrifice != "" {
		lines = append(lines, "The "+output.Escape(p.Sacrifice)+" god would accept this as a sacrifice.")
	}
	for _, e := range p.Effects {
		lines = append(lines, "  "+w.describeEffect(effect.Active{Spec: e}))
	}
	for _, e := range it.Effects {
		lines = append(lines, "  "+w.describeEffect(e))
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString("{c}" + l + "{x}\n")
	}
	return b.String()
}

// describeEffect asks the rules for words; the default names the kind
// and its parameters.
func (w *World) describeEffect(e effect.Active) string {
	var text string
	if w.callOptional("describeEffect", &text, e.View()) && text != "" {
		return output.Escape(text)
	}
	keys := make([]string, 0, len(e.Params))
	for k := range e.Params {
		if k != "feat" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+" "+anyString(e.Params[k]))
	}
	s := output.Escape(e.Kind)
	if len(parts) > 0 {
		s += ": " + output.Escape(strings.Join(parts, ", "))
	}
	if e.Rounds > 0 {
		s += " (" + plural(e.Rounds, "round") + ")"
	}
	return s
}

func anyString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return itoa(x)
	case int64:
		return itoa(int(x))
	case float64:
		if x == float64(int(x)) {
			return itoa(int(x))
		}
		return ftoa(x)
	case bool:
		if x {
			return "yes"
		}
		return "no"
	default:
		return "?"
	}
}

// spreadRange shows a mean and its spread as the range a roll falls in,
// in whole numbers: 11.8 at 20 percent is "9-14". A range that rounds to
// one number is that number.
func spreadRange(mean, spread float64) string {
	lo, hi := spreadBounds(mean, spread)
	if lo == hi {
		return itoa(lo)
	}
	return itoa(lo) + "-" + itoa(hi)
}

// spreadBounds is the low and high end of a roll, rounded.
func spreadBounds(mean, spread float64) (lo, hi int) {
	return int(math.Round(mean * (1 - spread))), int(math.Round(mean * (1 + spread)))
}

// trimNum prints a number with one decimal, or none when it is whole.
func trimNum(f float64) string {
	if f == float64(int(f)) {
		return itoa(int(f))
	}
	return ftoa(f)
}

// pluralNoun pluralises a swing verb used as a noun: slash to slashes,
// bite to bites, punch to punches.
func pluralNoun(w string) string {
	switch {
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "sh"), strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "x"):
		return w + "es"
	default:
		return w + "s"
	}
}

// gearScore is what carried gear is weighed by, as the tap client weighs
// it: a weapon's damage per round (the middle of its range times its
// swings), armor's defense. Effects are not scored.
func gearScore(p *item.Proto) float64 {
	if p.Type == item.Weapon {
		lo, hi := spreadBounds(p.Resolved.Weapon.Damage, p.Resolved.Weapon.Spread)
		return float64(lo+hi) / 2 * max(p.Resolved.Weapon.Speed, 1)
	}
	return p.Resolved.Armor.Defense
}

// gearCompare weighs a carried weapon or piece of armor against the
// weakest one worn where it would go (either wrist, either finger; the
// main hand for a weapon). Within 2% is the same. It is nil for anything
// else, or when neither side has numbers.
func gearCompare(c *Character, it *item.Item) *output.GearCompare {
	p := it.Proto
	if p.Type != item.Weapon && p.Type != item.Armor {
		return nil
	}
	mine := gearScore(p)
	var weakest *item.Item
	for _, pos := range item.Positions(p.WearSlot()) {
		worn, ok := c.Equipment[pos]
		if !ok {
			return &output.GearCompare{Dir: "up", Delta: math.Round(mine*10) / 10, Empty: true}
		}
		if weakest == nil || gearScore(worn.Proto) < gearScore(weakest.Proto) {
			weakest = worn
		}
	}
	if weakest == nil {
		return nil
	}
	theirs := gearScore(weakest.Proto)
	if mine == 0 && theirs == 0 {
		return nil
	}
	d := mine - theirs
	rel := 0.0
	switch {
	case theirs != 0:
		rel = d / theirs
	case d > 0:
		rel = 1
	}
	cmp := &output.GearCompare{Dir: "same", Delta: math.Round(d*10) / 10, Worn: weakest.ID}
	switch {
	case rel > 0.02:
		cmp.Dir = "up"
	case rel < -0.02:
		cmp.Dir = "down"
	}
	return cmp
}

// compareMark is gearCompare as `inventory` shows it: "(+2)" in green,
// "(-1.5)" in red, "(=)", "(empty slot)"; "" when it does not apply.
func compareMark(c *Character, it *item.Item) string {
	cmp := gearCompare(c, it)
	switch {
	case cmp == nil:
		return ""
	case cmp.Empty:
		return "{G}(empty slot){x}"
	case cmp.Dir == "up":
		return "{G}(+" + trimNum(cmp.Delta) + "){x}"
	case cmp.Dir == "down":
		return "{R}(" + trimNum(cmp.Delta) + "){x}"
	}
	return "{D}(=){x}"
}
