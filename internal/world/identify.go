package world

import (
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
	switch p.Type {
	case item.Weapon:
		wp := p.Resolved.Weapon
		line := "Damage " + trimNum(wp.Damage) + " per swing, speed " + trimNum(wp.Speed)
		if wp.Spread > 0 {
			line += ", varies by " + itoa(int(wp.Spread*100+0.5)) + " percent"
		}
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
			line := "Defense " + trimNum(ar.Defense)
			if ar.Spread > 0 {
				line += ", varies by " + itoa(int(ar.Spread*100+0.5)) + " percent"
			}
			lines = append(lines, line)
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
