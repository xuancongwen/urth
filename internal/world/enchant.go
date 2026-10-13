package world

import (
	"strings"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
)

// Enchanting (docs/RULES.md 5.6): an enchantment is a spell whose target
// is an item in the caster's inventory. The rules' enchantBlocker says,
// before mana is spent, why the item cannot take it; resolveEnchant
// decides the attempt and returns the item's applied effects as they now
// stand, or that it was destroyed. The Enchanting skill's rating sets
// the chance; the first enchantment cast gives it.

// EnchantResult is what resolveEnchant returns.
type EnchantResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	// Outcome is success, nothing, strip, or destroy.
	Outcome string `json:"outcome"`
	// Effects, when present, replace the item's applied effects; Rarity,
	// Color, Prefix, and Suffix are its naming after the change.
	Effects []effect.Spec      `json:"effects"`
	Rarity  string             `json:"rarity"`
	Color   string             `json:"color"`
	Prefix  string             `json:"prefix"`
	Suffix  string             `json:"suffix"`
	Skills  map[string]float64 `json:"skills"`
}

// enchantView is the item as an enchantment sees it: its view, its
// shown name, and its applied effects apart from its intrinsic ones.
func enchantView(it *item.Item) map[string]any {
	v := itemView(it)
	v["name"] = it.Name()
	v["applied"] = effect.Views(nil, it.Effects)
	if it.Prefix != "" {
		v["prefix"] = it.Prefix
	}
	if it.Suffix != "" {
		v["suffix"] = it.Suffix
	}
	return v
}

// enchantTarget finds the item an enchantment is cast on, in c's
// inventory, and asks the rules whether it can take it.
func (w *World) enchantTarget(c *Character, sp Spell, arg string) (*item.Item, string) {
	if arg == "" {
		return nil, "Cast it on what?"
	}
	found := item.Find(c.Inventory, item.ParseTarget(arg))
	if len(found) == 0 {
		return nil, "You aren't carrying that."
	}
	it := found[0]
	var why string
	if w.callOptional("enchantBlocker", &why, w.view(c), enchantView(it), spellView(sp)) && why != "" {
		return nil, output.Escape(why)
	}
	return it, ""
}

// resolveEnchantNow runs resolveEnchant on it and applies the outcome.
func (w *World) resolveEnchantNow(c *Character, sp Spell, it *item.Item) {
	if sk, ok := w.skillByID("enchanting"); ok {
		ensureSkill(c, sk)
	}
	var r EnchantResult
	if !w.call("resolveEnchant", &r, w.view(c), enchantView(it), spellView(sp)) || !r.OK {
		msg := r.Message
		if msg == "" {
			msg = "The spell fizzles."
		}
		c.Send(output.Escape(msg) + "\n")
		return
	}
	if c.cooldowns == nil {
		c.cooldowns = map[string]int{}
	}
	if sp.Cooldown > 0 {
		c.cooldowns[sp.ID] = sp.Cooldown
	}
	applySkillRatings(c, r.Skills)
	name := output.Escape(it.Name())
	w.act("You cast {Y}$t{x} on "+name+".", c, nil, output.Escape(sp.Name), toChar)
	w.act("$n casts $t on "+name+".", c, nil, output.Escape(sp.Name), toRoom)
	switch r.Outcome {
	case "destroy":
		c.Inventory = item.Remove(c.Inventory, it)
		c.Send("{R}" + output.Escape(r.Message) + "{x}\n")
		w.act("$n's "+bareName(name)+" crumbles to dust.", c, nil, "", toRoom)
		return
	case "success", "strip":
		it.Effects = nil
		for _, e := range r.Effects {
			if e.Kind != "" {
				it.Effects = append(it.Effects, effect.Active{Spec: e})
			}
		}
		it.Rarity, it.Color = r.Rarity, ""
		if output.IsColor(r.Color) {
			it.Color = r.Color
		}
		it.Prefix, it.Suffix = strings.TrimSpace(r.Prefix), strings.TrimSpace(r.Suffix)
	}
	color := "{G}"
	if r.Outcome != "success" {
		color = "{y}"
	}
	c.Send(color + output.Escape(r.Message) + "{x}\n")
}

// skillByID finds a skill in the rules' list.
func (w *World) skillByID(id string) (Skill, bool) {
	for _, sk := range w.skillList() {
		if sk.ID == id {
			return sk, true
		}
	}
	return Skill{}, false
}

// bareName is an item's name without its leading article, for "$n's
// sword".
func bareName(name string) string {
	for _, art := range []string{"a ", "an ", "the ", "some "} {
		if rest, ok := strings.CutPrefix(name, art); ok {
			return rest
		}
	}
	return name
}
