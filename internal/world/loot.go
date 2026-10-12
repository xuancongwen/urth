package world

import (
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/output"
)

// Loot (docs/RULES.md 5.3): plain weapons and armor can come out of a
// reset or a death better than their prototype. The rules' rollItem hook
// decides the tier and the affixes, which are stored as the instance's
// applied effects like an enchantment; lootRules decides how many extra
// items a death drops; itemTier names the tier again for an item loaded
// from a player file. All are optional, and without them nothing rolls.

// ItemRoll is what rollItem returns. An empty Rarity leaves the item plain.
type ItemRoll struct {
	Rarity  string        `json:"rarity"`
	Color   string        `json:"color"`
	Prefix  string        `json:"prefix"`
	Suffix  string        `json:"suffix"`
	Effects []effect.Spec `json:"effects"`
}

// LootRules is what lootRules returns for a death: how many extra items
// the corpse gets, drawn from prototypes within LevelBand of the victim.
type LootRules struct {
	Drops     int `json:"drops"`
	LevelBand int `json:"levelBand"`
}

// rollable reports whether instances of p may be rolled: plain weapons and
// armor from content. A prototype with intrinsic effects is a named item.
func rollable(p *item.Proto) bool {
	if p.Vnum == 0 || len(p.Effects) > 0 || (p.Type != item.Weapon && p.Type != item.Armor) {
		return false
	}
	return !p.HasFlag("quest") && !p.HasFlag("newbie") && !p.HasFlag("norandom")
}

// rollItem asks the rules to roll a fresh instance. Source is "reset" or
// "drop"; level is the level of whatever it came from.
func (w *World) rollItem(it *item.Item, source string, level int) {
	if it.Rolled() || !rollable(it.Proto) {
		return
	}
	var r ItemRoll
	ctx := map[string]any{"source": source, "level": level}
	if !w.callOptional("rollItem", &r, itemView(it), ctx) || r.Rarity == "" {
		return
	}
	it.Rarity = r.Rarity
	if output.IsColor(r.Color) {
		it.Color = r.Color
	}
	it.Prefix = strings.TrimSpace(r.Prefix)
	it.Suffix = strings.TrimSpace(r.Suffix)
	for _, e := range r.Effects {
		if e.Kind != "" {
			it.Effects = append(it.Effects, effect.Active{Spec: e})
		}
	}
}

// retier asks the rules to name a rolled item's tier again from the
// effects it carries, so an item saved under an older table comes back
// under the current one. Without the hook, or without a tier for the
// count, the item keeps what it had.
func (w *World) retier(it *item.Item) {
	var r ItemRoll
	if !it.Rolled() || !w.callOptional("itemTier", &r, itemView(it)) || r.Rarity == "" {
		return
	}
	it.Rarity, it.Color = r.Rarity, ""
	if output.IsColor(r.Color) {
		it.Color = r.Color
	}
}

// newResetItem creates an item for a reset and rolls it.
func (w *World) newResetItem(p *item.Proto, level int) *item.Item {
	it := item.New(p)
	w.rollItem(it, "reset", max(level, p.Level))
	return it
}

// lootDrops makes what a death adds to the corpse besides the victim's own
// gear: its loot tables, then the rules' extra drops.
func (w *World) lootDrops(victim *Character, rng *rand.Rand) []*item.Item {
	if victim.mob == nil {
		return nil
	}
	return append(w.tableDrops(victim, rng), w.extraDrops(victim, rng)...)
}

// lootTable is a mob's loot table followed by its area's.
func (w *World) lootTable(p *mob.Proto) []item.LootEntry {
	return append(slices.Clone(p.Loot), w.content.Rooms.Areas[p.Area].Loot...)
}

// tableDrops draws each line of the victim's loot tables once with rng. A
// rollable item rolls as a drop; a named item comes out as written.
func (w *World) tableDrops(victim *Character, rng *rand.Rand) []*item.Item {
	var out []*item.Item
	for _, e := range w.lootTable(victim.mob.Proto) {
		p := w.content.Items[e.Item]
		if p == nil || rng.Float64() >= e.Chance {
			continue
		}
		it := item.New(p)
		w.rollItem(it, "drop", victim.Level)
		out = append(out, it)
	}
	return out
}

// extraDrops makes the extra items a death drops, drawn with rng from the
// rollable prototypes of the victim's area within the rules' level band.
func (w *World) extraDrops(victim *Character, rng *rand.Rand) []*item.Item {
	var rules LootRules
	if !w.callOptional("lootRules", &rules, w.view(victim)) || rules.Drops <= 0 {
		return nil
	}
	cands := w.dropCandidates(victim.mob.Proto.Area, victim.Level, max(rules.LevelBand, 0))
	if len(cands) == 0 {
		return nil
	}
	var out []*item.Item
	for range min(rules.Drops, 10) {
		it := item.New(cands[rng.IntN(len(cands))])
		w.rollItem(it, "drop", victim.Level)
		out = append(out, it)
	}
	return out
}

// dropCandidates lists the rollable prototypes of an area within band
// levels of level, in vnum order so a seeded draw is reproducible.
func (w *World) dropCandidates(area string, level, band int) []*item.Proto {
	var out []*item.Proto
	for _, vnum := range sortedItemVnums(w.content.Items) {
		p := w.content.Items[vnum]
		if p.Area != area || !rollable(p) {
			continue
		}
		if p.Level < level-band || p.Level > level+band {
			continue
		}
		out = append(out, p)
	}
	return out
}

func sortedItemVnums(m map[int]*item.Proto) []int {
	out := make([]int, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// shownName is an item's name for a listing: escaped, in its tier's color.
func shownName(it *item.Item) string {
	name := output.Escape(it.Name())
	if it.Color != "" {
		return "{" + it.Color + "}" + name + "{x}"
	}
	return name
}

// rarityTag is "(rare)" in the item's color, for lines that show a
// description rather than a name.
func rarityTag(it *item.Item) string {
	tag := "(" + output.Escape(it.Rarity) + ")"
	if it.Color != "" {
		return "{" + it.Color + "}" + tag + "{x}"
	}
	return tag
}

// cmdLoot samples what a mob's deaths drop: loot <mob vnum> [kills] [seed].
func cmdLoot(w *World, p *Player, args string) {
	fields := strings.Fields(args)
	if len(fields) < 1 {
		p.Send("Syntax: loot <mob vnum> [kills] [seed]\n")
		return
	}
	v, err := strconv.Atoi(fields[0])
	proto := w.content.Mobs[v]
	if err != nil || proto == nil {
		p.Send("No such mob: " + output.Escape(fields[0]) + "\n")
		return
	}
	n := 1000
	if len(fields) > 1 {
		if n, err = strconv.Atoi(fields[1]); err != nil || n < 1 || n > 100000 {
			p.Send("Kills must be 1 to 100000.\n")
			return
		}
	}
	rng := w.rng
	if len(fields) > 2 {
		seed, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			p.Send("Seed must be a number.\n")
			return
		}
		rng = rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	}
	if w.scripts != nil {
		w.scripts.SetRandom(rng)
		defer w.scripts.SetRandom(w.rng)
	}
	victim := w.simCharacter(proto)
	counts := map[string]int{}
	var order []string
	examples := map[string]*item.Item{}
	drops := 0
	table := w.lootTable(proto)
	found := map[int]int{}
	for range n {
		for _, it := range w.tableDrops(victim, rng) {
			found[it.Proto.Vnum]++
		}
		for _, it := range w.extraDrops(victim, rng) {
			drops++
			tier := it.Rarity
			if tier == "" {
				tier = "common"
			}
			if counts[tier] == 0 {
				order = append(order, tier)
				examples[tier] = it
			}
			counts[tier]++
		}
	}
	// Tiers in order of their example's affix count, plain first.
	sort.SliceStable(order, func(i, j int) bool { return len(examples[order[i]].Effects) < len(examples[order[j]].Effects) })
	cands := 0
	var rules LootRules
	if w.callOptional("lootRules", &rules, w.view(victim)) {
		cands = len(w.dropCandidates(proto.Area, victim.Level, max(rules.LevelBand, 0)))
	}
	var b strings.Builder
	b.WriteString(itoa(n) + " kills of " + output.Escape(proto.Name) + " (level " + itoa(victim.Level) + ")")
	if len(table) > 0 {
		b.WriteString(", loot table:\n")
		seen := map[int]bool{}
		for _, e := range table {
			b.WriteString("  " + padLeft(strconv.FormatFloat(100*e.Chance, 'g', 4, 64)+"%", 7) + "  " +
				output.Escape(w.content.Items[e.Item].Name) + " [" + itoa(e.Item) + "]")
			if !seen[e.Item] { // a vnum listed twice is counted once, on its first line
				b.WriteString(", found " + itoa(found[e.Item]))
				seen[e.Item] = true
			}
			b.WriteString("\n")
		}
		b.WriteString("Extra drops")
	}
	b.WriteString(": " + plural(drops, "drop") + " from " + plural(cands, "candidate prototype") + ".\n")
	for _, tier := range order {
		it := examples[tier]
		b.WriteString("  " + padRight(output.Escape(tier), 10) + padLeft(itoa(counts[tier]), 6) + "  e.g. " + shownName(it) + "\n")
		for _, e := range it.Effects {
			b.WriteString("              " + w.describeEffect(e) + "\n")
		}
	}
	p.Send(b.String())
}
