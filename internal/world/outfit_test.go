package world

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"urth/internal/config"
	"urth/internal/content"
	"urth/internal/item"
	"urth/internal/script"
)

// shippedWorld builds a world on the real content and the live rules, for
// balance checks. It skips when the data directory is not there.
func shippedWorld(t *testing.T) *World {
	t.Helper()
	if _, err := os.Stat("../../data/world"); err != nil {
		t.Skip("no shipped world")
	}
	rw, err := content.Load("../../data/world")
	if err != nil {
		t.Fatal(err)
	}
	engine := script.New("../../data/scripts", slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	return New(config.Default(), rw, slog.New(slog.NewTextHandler(io.Discard, nil)), Deps{Scripts: engine})
}

func TestOutfitEquipsFreeSlotsAndPacksTheRest(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "outfit")
	if out := bob.take(); !strings.Contains(out, "No items are flagged outfit.") {
		t.Fatalf("empty outfit: %q", out)
	}
	add := func(vnum int, name string, typ item.Type, slot item.Slot) {
		p := &item.Proto{Vnum: vnum, Name: name, Keywords: keywordsOf(name), Type: typ, Slot: slot, Level: 1, Flags: []string{outfitFlag}}
		p.ResolveStated()
		w.content.Items[vnum] = p
	}
	add(90, "a sun blade", item.Weapon, "wield")
	add(91, "a sun ring", item.Armor, "finger")
	add(92, "a moon ring", item.Armor, "finger")
	add(93, "a star ring", item.Armor, "finger")
	send(w, 1, "outfit")
	out := bob.take()
	for _, want := range []string{"<wielded>             a sun blade", "<worn on left finger> a sun ring", "<worn on right finger>a moon ring", "Into your pack, for want of a free slot:\n    a star ring"} {
		if !strings.Contains(out, want) {
			t.Fatalf("outfit missing %q: %q", want, out)
		}
	}
	p := w.players[1]
	if len(p.Equipment) != 3 || len(p.Inventory) != 1 || p.Inventory[0].Proto.Vnum != 93 {
		t.Fatalf("equipment %v inventory %v", p.Equipment, p.Inventory)
	}

	w.players[1].Admin = false
	send(w, 1, "outfit")
	if out := bob.take(); !strings.Contains(out, "Huh?") {
		t.Fatalf("outfit open to players: %q", out)
	}
}

// Procs, a heal, and effects ride a landed swing; thorns come back; an
// aura strikes every round.
func TestOnHitProcsHealThornsAndAura(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	setRules(t, w, strings.Replace(strings.Replace(testRules,
		`stage: "" };`,
		`stage: "", heal: a.isPlayer ? 3 : 0, procs: a.isPlayer ? [{ verb: "searing flames", damage: 2 }] : [{ verb: "spikes", damage: 1, back: true }],
		  effects: a.isPlayer ? [{ on: "target", kind: "dot", params: { damage: 1 }, rounds: 2 }] : [] };`, 1),
		`return { healthDelta: c.fighting ? 0 : 1, manaDelta: 0 };`,
		`return { healthDelta: c.fighting ? 0 : 1, manaDelta: 0, aura: c.isPlayer && c.fighting ? [{ verb: "corona", damage: 4 }] : [] };`, 1))
	// The test rules size health by level, and attaching an effect
	// recalculates it.
	p := w.players[1]
	p.Level = 45
	w.recalc(p.Character)
	p.Health = 50
	guard := w.contents(p.Room).mobs[0]
	guard.Level = 45
	w.recalc(guard.Character)
	guard.Health = 100
	send(w, 1, "kill guard")
	out := bob.take()
	for _, want := range []string{"Your searing flames hit a city guard. [2]", "You draw life from a city guard. [3]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("swing missing %q: %q", want, out)
		}
	}
	// The punch (1), the flames (2); Bob drew 3.
	if guard.Health != 97 || p.Health != 53 {
		t.Fatalf("guard %d bob %d", guard.Health, p.Health)
	}
	if len(guard.Effects) != 1 || guard.Effects[0].Kind != "dot" {
		t.Fatalf("effect not attached: %v", guard.Effects)
	}
	// The guard's swing lands and Bob's spikes answer it; Bob's corona
	// burns at the end of the round.
	out = tickUntil(t, w, bob, "corona")
	if !strings.Contains(out, "A city guard's slash hits you. [4]\nYour spikes hit a city guard. [1]") || !strings.Contains(out, "Your corona hits a city guard. [4]") {
		t.Fatalf("thorns and aura: %q", out)
	}
}

// The outfit set is overpowered, not invincible (docs/RULES.md 4.5 for the
// standard fighter's row): even fights end in a couple of rounds, and a
// mob twenty levels up still usually wins. Runs the live rules, so a
// change to either the set or the curve that breaks the shape shows here.
func TestOutfitBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("simulation")
	}
	w := shippedWorld(t)
	const level = 5
	even := w.simulate(func() *Character { return w.simOutfitter(level) }, w.content.Mobs[900+level], 100, 1)
	if even.AWins != even.Fights || even.MeanRound > 3 {
		t.Errorf("even fight: %d of %d won in %.1f rounds; the set should crush it", even.AWins, even.Fights, even.MeanRound)
	}
	far := w.simulate(func() *Character { return w.simOutfitter(level) }, w.content.Mobs[900+level+20], 100, 1)
	if far.AWins > far.Fights/2 {
		t.Errorf("+20 fight: %d of %d won; the set should not beat everything", far.AWins, far.Fights)
	}
}
