package world

import (
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
)

// Enchanting (docs/RULES.md 5.6) under the shipped rules: the spells are
// cast on an item in the caster's inventory and resolved by
// resolveEnchant.

// enchanter is a level-25 caster with the enchantment school, a full
// pool, and the Enchanting skill at rating.
func enchanter(t *testing.T, w *World, rating float64) *Character {
	t.Helper()
	c := w.simFighter(25)
	c.Stats["intelligence"] = 22
	c.Schools = []string{"enchantment"}
	w.recalc(c)
	c.Mana = c.ManaMax
	sk, ok := w.skillByID("enchanting")
	if !ok {
		t.Fatal("no enchanting skill")
	}
	ensureSkill(c, sk).State["effectiveness"] = rating
	return c
}

// plainArmor is a fresh plain body armor of level L or above from the
// shipped world.
func plainArmor(t *testing.T, w *World, L int) *item.Item {
	t.Helper()
	for _, vnum := range sortedItemVnums(w.content.Items) {
		p := w.content.Items[vnum]
		if p.Type == item.Armor && p.Slot == "body" && p.Level >= L && rollable(p) && p.Resolved.Armor.Defense > 0 {
			return item.New(p)
		}
	}
	t.Fatalf("no plain body armor from level %d", L)
	return nil
}

func stats(n int) []effect.Active {
	var out []effect.Active
	for range n {
		out = append(out, effect.Active{Spec: effect.Spec{Kind: "stat", Params: map[string]any{"stat": "strength", "amount": 1}}})
	}
	return out
}

func blocker(t *testing.T, w *World, c *Character, it *item.Item, spell string) string {
	t.Helper()
	_, why := w.enchantTarget(c, shippedSpell(t, w, spell), it.Proto.Keywords[0])
	return why
}

func TestEnchantSpellsCostAndGates(t *testing.T) {
	w := shippedWorld(t)
	for _, id := range []string{"enchant_weapon", "enchant_armor", "restore_item", "permanency"} {
		if sp := shippedSpell(t, w, id); sp.Mana != 150 || sp.Target != "item" {
			t.Errorf("%s: mana %d target %q, want 150 and item", id, sp.Mana, sp.Target)
		}
	}
	c := enchanter(t, w, 50)
	blade := plainBlade(t, w)
	c.Inventory = []*item.Item{blade}
	if why := blocker(t, w, c, blade, "enchant_armor"); !strings.Contains(why, "no defense") {
		t.Errorf("enchant armor on a sword: %q", why)
	}
	if why := blocker(t, w, c, blade, "enchant_weapon"); why != "" {
		t.Errorf("enchant weapon on a plain sword refused: %q", why)
	}
	if why := blocker(t, w, c, blade, "restore_item"); !strings.Contains(why, "as it was made") {
		t.Errorf("restore on a plain sword: %q", why)
	}
	// A level-10 sword stops at four effects (5.3's rare).
	blade.Effects = stats(4)
	if why := blocker(t, w, c, blade, "enchant_weapon"); !strings.Contains(why, "another effect") {
		t.Errorf("fifth effect on a level-10 sword: %q", why)
	}
	// Permanency stops everything, Restore included.
	blade.Effects = append(stats(1), effect.Active{Spec: effect.Spec{Kind: "permanent", Params: map[string]any{}}})
	for _, id := range []string{"enchant_weapon", "restore_item", "permanency"} {
		if why := blocker(t, w, c, blade, id); !strings.Contains(why, "permanent") {
			t.Errorf("%s on a permanent sword: %q", id, why)
		}
	}
}

func TestEnchantOutcomes(t *testing.T) {
	w := shippedWorld(t)
	w.scripts.SetRandom(rand.New(rand.NewPCG(3, 3)))
	defer w.scripts.SetRandom(w.rng)
	ew := shippedSpell(t, w, "enchant_weapon")

	// A master on a plain sword: 100/(100+20) take, the rest do nothing
	// or (one in twenty) strip, and nothing is ever destroyed.
	took, n := 0, 2000
	for range n {
		c := enchanter(t, w, 100)
		blade := plainBlade(t, w)
		c.Inventory = []*item.Item{blade}
		w.resolveEnchantNow(c, ew, blade)
		if len(c.Inventory) != 1 {
			t.Fatal("a plain sword was destroyed")
		}
		if len(blade.Effects) == 1 {
			took++
			p := blade.Effects[0].Params
			f := toFloat(p["fraction"])
			if blade.Effects[0].Kind != "baseDamage" || f < 0.05 || f > 0.10 || p["enchanted"] != true {
				t.Fatalf("enchantment: %+v", blade.Effects[0])
			}
			if blade.Rarity != "uncommon" {
				t.Fatalf("one effect should be uncommon: %q", blade.Rarity)
			}
		}
	}
	if share := float64(took) / float64(n); share < 0.78 || share > 0.88 {
		t.Errorf("master's take on a plain sword: %.2f, want about 0.83", share)
	}

	// Rating 0 always fails. Past twelve a failure may destroy (3/28 at
	// fifteen) or strip (one in ten).
	destroyed, stripped := 0, 0
	for range n {
		c := enchanter(t, w, 0)
		armor := plainArmor(t, w, 40)
		armor.Effects = stats(15)
		armor.Rarity = "relic"
		c.Inventory = []*item.Item{armor}
		w.resolveEnchantNow(c, shippedSpell(t, w, "enchant_armor"), armor)
		switch {
		case len(c.Inventory) == 0:
			destroyed++
		case len(armor.Effects) == 0:
			stripped++
			if armor.Rarity != "" {
				t.Fatalf("stripped armor kept its tier %q", armor.Rarity)
			}
		case len(armor.Effects) != 15:
			t.Fatalf("a failure changed the effects: %d", len(armor.Effects))
		}
	}
	if d := float64(destroyed) / float64(n); d < 0.08 || d > 0.14 {
		t.Errorf("destroyed at fifteen effects: %.3f, want about 0.107", d)
	}
	if s := float64(stripped) / float64(n); s < 0.07 || s > 0.13 {
		t.Errorf("stripped at fifteen effects: %.3f, want about 0.10", s)
	}
}

func TestRestoreAndPermanency(t *testing.T) {
	w := shippedWorld(t)
	w.scripts.SetRandom(rand.New(rand.NewPCG(5, 5)))
	defer w.scripts.SetRandom(w.rng)

	// Restore takes a rolled, enchanted sword back to its prototype.
	restored := false
	for range 50 {
		c := enchanter(t, w, 100)
		blade := plainBlade(t, w)
		blade.Effects = stats(3)
		blade.Rarity, blade.Prefix, blade.Suffix = "rare", "mighty", "of the bear"
		c.Inventory = []*item.Item{blade}
		w.resolveEnchantNow(c, shippedSpell(t, w, "restore_item"), blade)
		if len(blade.Effects) == 0 && blade.Rarity == "" && blade.Prefix == "" && blade.Suffix == "" {
			restored = true
			break
		}
	}
	if !restored {
		t.Error("restore never returned a sword to its prototype in 50 tries")
	}

	// Permanency marks the item without changing its tier.
	for range 50 {
		c := enchanter(t, w, 100)
		blade := plainBlade(t, w)
		blade.Effects = stats(1)
		blade.Rarity = "uncommon"
		c.Inventory = []*item.Item{blade}
		w.resolveEnchantNow(c, shippedSpell(t, w, "permanency"), blade)
		if len(blade.Effects) == 2 {
			if blade.Effects[1].Kind != "permanent" || blade.Rarity != "uncommon" {
				t.Fatalf("permanency: %+v, tier %q", blade.Effects, blade.Rarity)
			}
			return
		}
	}
	t.Error("permanency never took in 50 tries")
}

// The cast command spends the mana at once and resolves the enchantment
// when the cast completes.
func TestCastEnchantOnAnItem(t *testing.T) {
	w := testWorld(t)
	src, err := os.ReadFile("../../data/scripts/rules.js")
	if err != nil {
		t.Fatal(err)
	}
	setRules(t, w, string(src))
	bob := login(t, w, 1, "Bob")
	c := w.players[1].Character
	c.Level = 25
	c.Stats["intelligence"] = 22
	c.Schools = []string{"enchantment"}
	w.recalc(c)
	c.Mana = c.ManaMax
	blade := giveItem(w, c, 10) // the rusty sword
	before := c.Mana
	send(w, 1, "cast 'enchant weapon' "+blade.Proto.Keywords[0])
	if o := bob.take(); !strings.Contains(o, "You begin casting Enchant Weapon") {
		t.Fatalf("cast: %q", o)
	}
	if c.Mana != before-150 {
		t.Fatalf("mana %d, want %d", c.Mana, before-150)
	}
	for range 20 { // two rounds: the cast completes
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "You cast Enchant Weapon on") {
		t.Fatalf("resolve: %q", o)
	}
	if skillEffect(c, "enchanting") == nil {
		t.Error("the first enchantment did not give the Enchanting skill")
	}
}
