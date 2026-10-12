package world

import (
	"math/rand/v2"
	"strings"
	"testing"

	"urth/internal/item"
)

// handsProto is a resolved weapon prototype taking hands hands.
func handsProto(vnum int, name, keyword string, hands int) *item.Proto {
	p := &item.Proto{Vnum: vnum, Name: name, Keywords: []string{keyword}, Type: item.Weapon, Slot: "wield",
		Weapon: &item.WeaponSpec{Damage: 5, Speed: 1, Hands: hands}}
	p.ResolveStated()
	return p
}

// Hands (RULES 4.4): a two-handed weapon leaves no hand for a shield or a
// second weapon, a second weapon is one-handed and takes the shield's
// hand, and a second weapon left alone moves to the main hand.
func TestHandsAreEnforced(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	great := item.New(handsProto(9101, "a greatsword", "greatsword", 2))
	sword := item.New(handsProto(9102, "a sword", "sword", 1))
	dagger := item.New(handsProto(9103, "a dagger", "dagger", 1))
	shieldProto := &item.Proto{Vnum: 9104, Name: "a buckler", Keywords: []string{"buckler"}, Type: item.Armor, Slot: "shield"}
	shieldProto.ResolveStated()
	buckler := item.New(shieldProto)
	p.Inventory = append(p.Inventory, great, sword, dagger, buckler)

	expect := func(cmd, want string) {
		t.Helper()
		send(w, 1, cmd)
		if o := bob.take(); !strings.Contains(o, want) {
			t.Fatalf("%s: want %q in %q", cmd, want, o)
		}
	}

	expect("second dagger", "You need a weapon in your main hand first.")
	expect("second", "Wield what as a second weapon?")
	expect("wield greatsword", "You wield a greatsword.")
	expect("wear buckler", "Your weapon takes both hands; there is none free for a shield.")
	expect("second dagger", "Your weapon already takes both hands.")
	if p.Equipment["shield"] != nil || p.Equipment["second"] != nil {
		t.Fatalf("refused gear went on: %v", p.Equipment)
	}

	expect("wield sword", "You wield a sword.")
	expect("second greatsword", "That takes both hands; it cannot be a second weapon.")
	expect("second dagger", "You wield a dagger as a second weapon.")
	expect("wear buckler", "Your second weapon is in the hand a shield would take.")
	expect("wield greatsword", "That takes both hands; remove your second weapon first.")
	if p.Equipment["wield"] != sword || p.Equipment["second"] != dagger {
		t.Fatalf("hands: %v", p.Equipment)
	}

	expect("remove sword", "You shift a dagger to your main hand.")
	if p.Equipment["wield"] != dagger || p.Equipment["second"] != nil {
		t.Fatalf("after removing the main weapon: %v", p.Equipment)
	}
	expect("wear buckler", "You wear a buckler.")
	expect("second sword", "Your shield is in the hand a second weapon would take.")
	expect("wield greatsword", "That takes both hands; remove your shield first.")
}

// Gear saved before hands were enforced comes off what cannot stay.
func TestSettleHands(t *testing.T) {
	c := newCharacter("Ada", nil)
	great := item.New(handsProto(9101, "a greatsword", "greatsword", 2))
	dagger := item.New(handsProto(9103, "a dagger", "dagger", 1))
	shieldProto := &item.Proto{Vnum: 9104, Name: "a buckler", Type: item.Armor, Slot: "shield"}
	buckler := item.New(shieldProto)
	c.Equipment["wield"], c.Equipment["second"], c.Equipment["shield"] = great, dagger, buckler
	settleHands(c)
	if c.Equipment["wield"] != great || c.Equipment["second"] != nil || c.Equipment["shield"] != nil || len(c.Inventory) != 2 {
		t.Fatalf("settled: %v, carried %d", c.Equipment, len(c.Inventory))
	}
}

// meanHands is the mean damage of n rounds of att's auto-attack on def
// with both hands, as live combat swings them, with a fixed seed.
func meanHands(w *World, att, def *Character, n int) float64 {
	w.scripts.SetRandom(rand.New(rand.NewPCG(5, 5)))
	defer w.scripts.SetRandom(w.rng)
	att.resetSwing()
	total := 0.0
	for i := 0; i < n; i++ {
		att.swing += att.Speed
		att.offSwing += att.OffSpeed
		for hand := att.nextSwing(); hand != ""; hand = att.nextSwing() {
			total += float64(w.resolveAttack(att, def, hand, 1).Damage)
		}
	}
	return total / float64(n)
}

// Dual wielding under the shipped rules: untrained, the pair deals about
// what one weapon does; the Dual Wield feat makes the second weapon a
// whole extra weapon's worth; Two-Weapon Fighting adds off-hand swings.
// Great Weapon feats add damage to a two-hander and nothing to a sword.
func TestFightingStylesShippedRules(t *testing.T) {
	w := shippedWorld(t)
	foe := w.simFighter(10)
	feats := map[string]Feat{}
	for _, f := range w.featList() {
		feats[f.ID] = f
	}
	fighter := func(ids ...string) *Character {
		c := w.simFighter(10)
		for _, id := range ids {
			f, ok := feats[id]
			if !ok {
				t.Fatalf("no feat %q", id)
			}
			w.grantFeat(c, f)
		}
		w.recalc(c)
		return c
	}
	dual := func(c *Character) *Character {
		c.Equipment["second"] = item.New(c.Equipment["wield"].Proto)
		w.recalc(c)
		return c
	}
	const rounds = 300

	single := meanHands(w, fighter(), foe, rounds)
	untrained := dual(fighter())
	if untrained.OffSpeed <= 0 {
		t.Fatalf("a second weapon gives no off-hand swings: %v", untrained.OffSpeed)
	}
	if r := meanHands(w, untrained, foe, rounds) / single; r < 0.85 || r > 1.15 {
		t.Fatalf("untrained dual wield deals %.2fx one weapon, want about 1x", r)
	}
	trained := meanHands(w, dual(fighter("dual_wield")), foe, rounds)
	if r := trained / single; r < 1.8 || r > 2.2 {
		t.Fatalf("Dual Wield deals %.2fx one weapon, want about 2x", r)
	}
	twf := dual(fighter("dual_wield", "two_weapon", "greater_two_weapon"))
	if twf.OffSpeed < dual(fighter("dual_wield")).OffSpeed+1.9 {
		t.Fatalf("two off-hand feats add %v swings", twf.OffSpeed)
	}
	if meanHands(w, twf, foe, rounds) <= trained*1.5 {
		t.Fatal("off-hand feats add little damage")
	}

	if fighter().OffSpeed != 0 {
		t.Fatal("off-hand swings with no second weapon")
	}

	maul := &item.Proto{Vnum: 9200, Name: "a maul", Keywords: []string{"maul"}, Type: item.Weapon, Slot: "wield",
		Level: 10, Baseline: "heavy", Weapon: &item.WeaponSpec{Hands: 2}}
	w.resolveItem(maul)
	twoHander := func(c *Character) *Character {
		c.Equipment["wield"] = item.New(maul)
		w.recalc(c)
		return c
	}
	plain2h := meanHands(w, twoHander(fighter()), foe, rounds)
	for _, step := range []struct {
		feats []string
		want  float64
	}{
		{[]string{"great_weapon"}, 1.2},
		{[]string{"great_weapon", "great_weapon_mastery", "devastating_blows"}, 1.6},
	} {
		if r := meanHands(w, twoHander(fighter(step.feats...)), foe, rounds) / plain2h; r < step.want-0.05 || r > step.want+0.05 {
			t.Fatalf("%v: two-hander deals %.2fx, want %.2fx", step.feats, r, step.want)
		}
	}
	if r := meanHands(w, fighter("great_weapon"), foe, rounds) / single; r < 0.99 || r > 1.01 {
		t.Fatalf("Great Weapon Fighting changes a one-handed sword by %.2fx", r)
	}

	sheet := map[string]string{}
	for _, r := range w.combatSheet(dual(fighter())) {
		sheet[r.Label] = r.Value
	}
	if !strings.Contains(sheet["Off hand"], " a round, -60%") || sheet["Damage"] != "-40%" {
		t.Fatalf("untrained dual wield on the sheet: %v", sheet)
	}
}

// An extra attack is a weapon's worth of swings, not one swing (RULES
// 4.4), so Second, Third, and Fourth Attack add the same damage a round
// to a dagger, a sword, and a maul alike.
func TestExtraAttacksFairAcrossWeapons(t *testing.T) {
	w := shippedWorld(t)
	foe := w.simFighter(20)
	feats := map[string]Feat{}
	for _, f := range w.featList() {
		feats[f.ID] = f
	}
	dpr := func(baseline string, hands int) float64 {
		c := w.simFighter(20)
		for _, id := range []string{"second_attack", "third_attack", "fourth_attack"} {
			w.grantFeat(c, feats[id])
		}
		p := &item.Proto{Vnum: 9300, Name: "a " + baseline, Type: item.Weapon, Slot: "wield", Level: 20, Baseline: baseline,
			Weapon: &item.WeaponSpec{Hands: hands}}
		w.resolveItem(p)
		c.Equipment["wield"] = item.New(p)
		w.recalc(c)
		return meanHands(w, c, foe, 300)
	}
	sword := dpr("standard", 1)
	for _, b := range []struct {
		baseline string
		hands    int
	}{{"dagger", 1}, {"heavy", 2}} {
		if r := dpr(b.baseline, b.hands) / sword; r < 0.95 || r > 1.05 {
			t.Fatalf("with three extra attacks a %s deals %.2fx a sword", b.baseline, r)
		}
	}
}
