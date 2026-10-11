package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
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
