package world

import (
	"math/rand/v2"
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
	"urth/internal/room"
)

// lootRulesJS rolls every item rare with one affix and drops two items.
const lootRulesJS = testRules + `
function rollItem(it, ctx) {
  return { rarity: "rare", color: "B", prefix: "elusive", suffix: "of the bear", effects: [{ kind: "stat", params: { stat: "might", amount: ctx.source === "drop" ? 2 : 1 } }] };
}
function lootRules(v) { return { drops: 2, levelBand: 5 }; }
`

func TestRollItem(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	sword := item.New(w.content.Items[10])
	w.rollItem(sword, "reset", 1)
	if sword.Rarity != "rare" || sword.Color != "B" || sword.Name() != "an elusive rusty sword of the bear" || len(sword.Effects) != 1 {
		t.Fatalf("rolled sword: %+v name %q", sword, sword.Name())
	}
	w.rollItem(sword, "reset", 1)
	if len(sword.Effects) != 1 {
		t.Fatalf("an item rolls once: %+v", sword.Effects)
	}
	bread := item.New(w.content.Items[13])
	w.rollItem(bread, "reset", 1)
	named := *w.content.Items[10]
	named.Effects = []effect.Spec{{Kind: "crit"}}
	unique := item.New(&named)
	w.rollItem(unique, "reset", 1)
	if bread.Rolled() || unique.Rolled() {
		t.Fatal("bread and named items never roll")
	}
}

// A named item takes the color itemBaseline gives its prototype; a rolled
// item keeps its tier's.
func TestNamedItemColor(t *testing.T) {
	w := testWorld(t)
	w.content.Items[10].Effects = []effect.Spec{{Kind: "crit"}}
	setRules(t, w, lootRulesJS+`
function itemBaseline(p) { return p.effects.length > 0 ? { color: "C" } : {}; }
`)
	named := item.New(w.content.Items[10])
	if got := shownName(named); got != "{C}"+named.Name()+"{x}" {
		t.Fatalf("named item shown as %q", got)
	}
	if bread := item.New(w.content.Items[13]); shownName(bread) != bread.Name() {
		t.Fatalf("plain item colored: %q", shownName(bread))
	}
	rolled := item.New(w.content.Items[10])
	rolled.Rarity, rolled.Color = "rare", "B"
	if rolled.ShownColor() != "B" {
		t.Fatalf("rolled item shown in %q", rolled.ShownColor())
	}
}

func TestNoRollWithoutHook(t *testing.T) {
	w := testWorld(t)
	sword := item.New(w.content.Items[10])
	w.rollItem(sword, "reset", 1)
	if sword.Rolled() {
		t.Fatal("rolled without a rollItem hook")
	}
}

func TestResetRollsMobGear(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	room, _ := w.content.Rooms.Get(1)
	w.removeMobFromRoom(w.contents(room).mobs[0])
	w.resetArea(w.areas["a"])
	var guard *Mob
	for _, m := range w.contents(room).mobs {
		if m.Proto.Vnum == 20 {
			guard = m
		}
	}
	if guard == nil {
		t.Fatal("guard did not respawn")
	}
	if !guard.Equipment["wield"].Rolled() {
		t.Fatal("the guard's sword was not rolled")
	}
	if guard.Inventory[0].Rolled() {
		t.Fatal("bread rolled")
	}
}

func TestDeathDrops(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	room, _ := w.content.Rooms.Get(1)
	guard := w.contents(room).mobs[0]
	drops := w.lootDrops(guard.Character, w.rng)
	if len(drops) != 2 {
		t.Fatalf("drops: %d", len(drops))
	}
	for _, it := range drops {
		if it.Proto.Area != "a" || !rollable(it.Proto) || !it.Rolled() || it.Effects[0].Params["amount"] != int64(2) {
			t.Fatalf("drop %q area %q effects %+v", it.Name(), it.Proto.Area, it.Effects)
		}
	}
	corpse := w.makeCorpse(guard.Character, 5)
	if n := len(corpse.Contents); n != 4 { // sword, bread, two drops
		t.Fatalf("corpse holds %d", n)
	}
}

func TestRolledItemListingAndSave(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	plain, rolled := item.New(w.content.Items[10]), item.New(w.content.Items[10])
	w.rollItem(rolled, "drop", 1)
	p.Inventory = append(p.Inventory, plain, rolled)
	send(w, 1, "inventory")
	o := bob.take()
	if !strings.Contains(o, "an elusive rusty sword of the bear") || strings.Contains(o, "(2)") {
		t.Fatalf("listing: %q", o)
	}
	send(w, 1, "look bear")
	if o := bob.take(); !strings.Contains(o, "Rarity rare") {
		t.Fatalf("look: %q", o)
	}
	saved := saveItems(p.Inventory)
	back := w.loadItems(saved, "Bob")
	if back[1].Rarity != "rare" || back[1].Prefix != "elusive" || back[1].Suffix != "of the bear" || back[1].Color != "B" || len(back[1].Effects) != 1 {
		t.Fatalf("restored: %+v", back[1])
	}
}

func TestLootCommand(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	bob := login(t, w, 1, "Bob")
	w.players[1].Admin = true
	send(w, 1, "loot 20 10 7")
	o := bob.take()
	if !strings.Contains(o, "10 kills of a city guard") || !strings.Contains(o, "20 drops") || !strings.Contains(o, "rare") {
		t.Fatalf("loot: %q", o)
	}
}

// TestShippedMobsCanDrop checks the shipped world: every hostile mob in a
// placed area has plain gear near its level to drop (RULES 5.3).
func TestShippedMobsCanDrop(t *testing.T) {
	w := shippedWorld(t)
	for _, p := range w.content.Mobs {
		if p.HasFlag("peaceful") || w.content.Rooms.Areas[p.Area].Detached {
			continue
		}
		victim := w.simCharacter(p)
		var rules LootRules
		if !w.callOptional("lootRules", &rules, w.view(victim)) {
			t.Fatal("the shipped rules have no lootRules")
		}
		if len(w.dropCandidates(p.Area, victim.Level, rules.LevelBand)) == 0 {
			t.Errorf("%s (%s, level %d) has nothing to drop", p.Name, p.Area, victim.Level)
		}
	}
}

func TestRoomEntitiesCarryRarity(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	login(t, w, 1, "Bob")
	p := w.players[1]
	it := item.New(w.content.Items[10])
	w.rollItem(it, "drop", 1)
	w.contents(p.Room).items = append(w.contents(p.Room).items, it)
	_, items := w.roomEntities(p.Room, p)
	for _, e := range items {
		if e.Name == it.Name() {
			if e.Rarity != "rare" || e.Color != "B" {
				t.Fatalf("entity: %+v", e)
			}
			return
		}
	}
	t.Fatalf("rolled item not listed: %+v", items)
}

// TestKillSendsReward: a kill with autoloot on sends one reward naming the
// victim, with the experience and what came out of the corpse.
func TestKillSendsReward(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.rec.AutoLoot = true
	p.HealthMax, p.Health = 1000, 1000
	send(w, 1, "kill guard")
	var got *output.RewardData
	for i := 0; i < 400 && got == nil; i++ {
		w.Tick()
		for _, b := range bob.batches {
			for _, m := range b.Messages {
				if m.Type == output.Reward {
					r := m.Data.(output.RewardData)
					got = &r
				}
			}
		}
	}
	if got == nil {
		t.Fatal("no reward after the kill")
	}
	if len(got.Kills) != 1 || got.Kills[0] != "a city guard" || got.XP != 60 || len(got.Items) != 2 || got.Items[0].Name != "a rusty sword" {
		t.Fatalf("reward: %+v", *got)
	}
}

// TestLoadRetiers checks that a rolled item saved under an older tier
// comes back named by the rules' itemTier for its count of effects.
func TestLoadRetiers(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, testRules+`
function itemTier(it) { return it.effects.length >= 2 ? { rarity: "relic", color: "R" } : { rarity: "" }; }
`)
	two := []effect.Active{{Spec: effect.Spec{Kind: "stat"}}, {Spec: effect.Spec{Kind: "stat"}}}
	old := item.New(w.content.Items[10])
	old.Rarity, old.Color, old.Effects = "unique", "W", two
	odd := item.New(w.content.Items[10])
	odd.Rarity, odd.Color, odd.Effects = "rare", "B", two[:1]
	back := w.loadItems(saveItems([]*item.Item{old, odd}), "Bob")
	if back[0].Rarity != "relic" || back[0].Color != "R" {
		t.Errorf("old tier: %q %q", back[0].Rarity, back[0].Color)
	}
	if back[1].Rarity != "rare" || back[1].Color != "B" {
		t.Errorf("no tier from the rules should keep the saved one: %q %q", back[1].Rarity, back[1].Color)
	}
}

// TestShippedLootGates rolls the shipped tables: a low item level cannot
// reach the high tiers or the deep affixes, a slot takes only its own
// affixes, and a deep drop can reach relic (RULES 5.3).
func TestShippedLootGates(t *testing.T) {
	w := shippedWorld(t)
	protoIn := func(slot item.Slot, typ item.Type) *item.Proto {
		for _, vnum := range sortedItemVnums(w.content.Items) {
			p := w.content.Items[vnum]
			if rollable(p) && p.Type == typ && (typ == item.Weapon || p.Slot == slot) {
				cp := *p
				cp.Level = 1
				return &cp
			}
		}
		t.Fatalf("no rollable %s %s in the shipped world", typ, slot)
		return nil
	}
	roll := func(p *item.Proto, level, n int) (tiers map[string]int, kinds map[string]bool) {
		tiers, kinds = map[string]int{}, map[string]bool{}
		for range n {
			it := item.New(p)
			w.rollItem(it, "drop", level)
			tiers[it.Rarity]++
			for _, e := range it.Effects {
				kinds[e.Spec.Kind] = true
			}
		}
		return tiers, kinds
	}

	tiers, kinds := roll(protoIn("head", item.Armor), 2, 2000)
	for tier := range tiers {
		if tier != "" && tier != "uncommon" {
			t.Errorf("a level-2 helmet rolled %s", tier)
		}
	}
	for kind := range kinds {
		if kind != "stat" && kind != "defense" && kind != "baseDefense" {
			t.Errorf("a level-2 helmet rolled a %s affix", kind)
		}
	}

	_, kinds = roll(protoIn("head", item.Armor), 40, 2000)
	for _, kind := range []string{"shieldBlock", "attacks", "crit", "damage", "baseDamage", "thorns"} {
		if kinds[kind] {
			t.Errorf("a helmet rolled %s, which is not open to the head", kind)
		}
	}
	if !kinds["detectInvisible"] {
		t.Error("a level-40 helmet never rolled true sight in 2000 drops")
	}

	tiers, _ = roll(protoIn("", item.Weapon), 40, 3000)
	if tiers["relic"] == 0 || tiers["legendary"] == 0 {
		t.Errorf("level-40 weapon drops never reached the top tiers: %v", tiers)
	}
	for _, old := range []string{"unique", "artifact"} {
		if tiers[old] > 0 {
			t.Errorf("rolled the retired tier %s", old)
		}
	}
}

// TestLootTables draws a mob's table and its area's: a sure line always
// comes up, a rollable item rolls, a named one does not, and a half chance
// comes up about half the time.
func TestLootTables(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS+"\nfunction lootRules(v) { return { drops: 0, levelBand: 0 }; }\n")
	room, _ := w.content.Rooms.Get(1)
	guard := w.contents(room).mobs[0]
	guard.Proto.Loot = []item.LootEntry{{Item: 13, Chance: 1}, {Item: 10, Chance: 1}, {Item: 10, Chance: 1e-9}}
	w.content.Rooms.Areas["a"] = roomArea(w, "a", []item.LootEntry{{Item: 13, Chance: 1}})
	drops := w.lootDrops(guard.Character, w.rng)
	if len(drops) != 3 || drops[0].Proto.Vnum != 13 || drops[1].Proto.Vnum != 10 || drops[2].Proto.Vnum != 13 {
		t.Fatalf("drops: %v", drops)
	}
	if drops[0].Rolled() || !drops[1].Rolled() {
		t.Fatalf("bread rolled %v, sword rolled %v", drops[0].Rolled(), drops[1].Rolled())
	}

	guard.Proto.Loot = []item.LootEntry{{Item: 13, Chance: 0.5}}
	w.content.Rooms.Areas["a"] = roomArea(w, "a", nil)
	rng := rand.New(rand.NewPCG(1, 2))
	n := 0
	for range 2000 {
		n += len(w.tableDrops(guard.Character, rng))
	}
	if n < 900 || n > 1100 {
		t.Errorf("a half chance came up %d times in 2000", n)
	}

	bob := login(t, w, 1, "Bob")
	w.players[1].Admin = true
	send(w, 1, "loot 20 100 7")
	if o := bob.take(); !strings.Contains(o, "loot table:") || !strings.Contains(o, "50%") || !strings.Contains(o, "[13], found") {
		t.Fatalf("loot: %q", o)
	}
}

// roomArea is area name's metadata with its loot table replaced.
func roomArea(w *World, name string, loot []item.LootEntry) room.Area {
	a := w.content.Rooms.Areas[name]
	a.Loot = loot
	return a
}
