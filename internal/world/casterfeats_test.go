package world

import (
	"math"
	"math/rand/v2"
	"testing"

	"urth/internal/effect"
)

// casterFor is a level-10 caster under the shipped rules, with 16 in both
// casting stats and the given feats taken.
func casterFor(t *testing.T, w *World, feats ...string) *Character {
	t.Helper()
	c := w.simFighter(10)
	c.Stats["intelligence"], c.Stats["wisdom"] = 16, 16
	c.Specialty = "evocation"
	c.Deity = "good"
	byID := map[string]Feat{}
	for _, f := range w.featList() {
		byID[f.ID] = f
	}
	for _, id := range feats {
		f, ok := byID[id]
		if !ok {
			t.Fatalf("no feat %q", id)
		}
		w.grantFeat(c, f)
	}
	w.recalc(c)
	c.Mana = c.ManaMax
	return c
}

func shippedSpell(t *testing.T, w *World, id string) Spell {
	t.Helper()
	for _, sp := range w.spellList() {
		if sp.ID == id {
			return sp
		}
	}
	t.Fatalf("no spell %q", id)
	return Spell{}
}

// meanCast is the mean damage and healing a cast does to target over n
// casts, from the shipped resolveCast, with a fixed seed.
func meanCast(w *World, c, target *Character, sp Spell, n int) (dmg, heal, selfHeal float64) {
	w.scripts.SetRandom(rand.New(rand.NewPCG(7, 7)))
	defer w.scripts.SetRandom(w.rng)
	cv := w.view(c)
	tv := w.view(target)
	tv["self"] = target == c
	for i := 0; i < n; i++ {
		var r CastResult
		w.call("resolveCast", &r, cv, []any{tv}, spellView(sp))
		for _, tr := range r.Targets {
			dmg += float64(tr.Damage)
			heal += float64(tr.Heal)
		}
		selfHeal += float64(r.Heal)
	}
	return dmg / float64(n), heal / float64(n), selfHeal / float64(n)
}

// TestCasterFeatsShippedRules measures the caster feats of RULES 6.6
// against the shipped rules: what they do to cost, cast time, the pool,
// damage, and healing.
func TestCasterFeatsShippedRules(t *testing.T) {
	w := shippedWorld(t)
	fireball := shippedSpell(t, w, "fireball")
	plain := casterFor(t, w)
	fast := casterFor(t, w, "combat_casting", "quicken", "arcane_reservoir", "arcane_thrift")
	if sp := w.castCost(plain, fireball); sp.CastRounds != 2 || sp.Mana != 15 || !sp.InterruptOnDamage {
		t.Fatalf("plain fireball cost: %+v", sp)
	}
	if sp := w.castCost(fast, fireball); sp.CastRounds != 1 || sp.Mana != 12 || sp.InterruptOnDamage {
		t.Fatalf("quickened, steady, thrifty fireball: %+v", sp)
	}
	if sanct := w.castCost(fast, shippedSpell(t, w, "sanctuary")); !sanct.InterruptOnDamage || sanct.Mana != 20 {
		t.Fatalf("arcane feats touched a divine spell: %+v", sanct)
	}
	if got, want := fast.ManaMax, int(math.Round(float64(plain.ManaMax)*1.25)); got < want-1 || got > want+1 {
		t.Fatalf("reservoir: %d, want about %d (plain %d)", got, want, plain.ManaMax)
	}

	target := w.simFighter(10)
	firebolt := shippedSpell(t, w, "firebolt")
	base, _, _ := meanCast(w, plain, target, firebolt, 400)
	focused, _, _ := meanCast(w, casterFor(t, w, "spell_focus", "empower"), target, firebolt, 400)
	if r := focused / base; r < 1.30 || r > 1.45 {
		t.Fatalf("focus and empower: %.0f -> %.0f, ratio %.2f, want about 1.38", base, focused, r)
	}
	// Spell Focus reaches every spell, divine ones too (RULES 6.6, 2026-10-12).
	smite := shippedSpell(t, w, "smite")
	pious, _, _ := meanCast(w, plain, target, smite, 400)
	focusedSmite, _, _ := meanCast(w, casterFor(t, w, "spell_focus"), target, smite, 400)
	if r := focusedSmite / pious; r < 1.15 || r > 1.25 {
		t.Fatalf("spell focus on smite: ratio %.2f, want about 1.2", r)
	}
	archmage, _, _ := meanCast(w, casterFor(t, w, "empower", "archmage"), target, firebolt, 400)
	if r := archmage / base; r < 1.75 || r > 1.95 {
		t.Fatalf("empower and archmage: ratio %.2f, want about 1.84", r)
	}

	mend := shippedSpell(t, w, "mend")
	friend := w.simFighter(10)
	_, healed, _ := meanCast(w, plain, friend, mend, 50)
	_, graced, back := meanCast(w, casterFor(t, w, "healers_grace", "martyr"), friend, mend, 50)
	if r := graced / healed; r < 1.24 || r > 1.26 {
		t.Fatalf("healer's grace: %.0f -> %.0f, ratio %.2f", healed, graced, r)
	}
	if r := back / graced; r < 0.24 || r > 0.26 {
		t.Fatalf("martyr: %.1f back from %.1f given", back, graced)
	}
	t.Logf("firebolt at 10, 16 Int: %.0f plain, %.0f with Focus and Empower; mend %.0f, %.0f with Healer's Grace (Martyr returns %.0f)", base, focused, healed, graced, back)
}

// TestSyncFeatsFollowsTheRules: on load a held feat takes its current
// definition, a retired one becomes the feat that replaces it (once),
// and anything else retired is refunded as a pick (RULES 6.6, 2026-10-12).
func TestSyncFeatsFollowsTheRules(t *testing.T) {
	w := shippedWorld(t)
	c := w.simFighter(20)
	old := func(id string, mult float64, filter map[string]any) effect.Active {
		params := map[string]any{"feat": id, "mult": mult}
		for k, v := range filter {
			params[k] = v
		}
		return effect.Active{Spec: effect.Spec{Kind: "spellPower", Params: params}}
	}
	c.Effects = append(c.Effects,
		old("focus_evocation", 1.2, map[string]any{"school": "evocation"}),
		old("focus_illusion", 1.2, map[string]any{"school": "illusion"}),
		old("archmage", 1.15, map[string]any{"branch": "arcane"}),
		old("no_such_feat", 2, nil))
	if n := w.syncFeats(c); n != 2 || c.FeatPoints != 2 {
		t.Fatalf("refunded %d, picks %d; want 2 and 2", n, c.FeatPoints)
	}
	taken := takenFeats(c)
	if !taken["spell_focus"] || !taken["archmage"] || taken["focus_evocation"] || taken["focus_illusion"] || taken["no_such_feat"] {
		t.Fatalf("feats after sync: %v", taken)
	}
	for _, e := range c.Effects {
		switch e.Params["feat"] {
		case "archmage":
			if e.Params["mult"] != 1.6 {
				t.Fatalf("archmage kept its old number: %v", e.Params)
			}
		case "spell_focus":
			if _, ok := e.Params["school"]; ok {
				t.Fatalf("spell focus kept a school: %v", e.Params)
			}
		}
	}
}

// TestSpellTiers: a common arcane spell is open to anyone, a school spell
// needs its school, a common divine spell any god, and a god's own spell
// that god (RULES 6.1, 2026-10-12).
func TestSpellTiers(t *testing.T) {
	w := shippedWorld(t)
	c := w.simFighter(10)
	for id, want := range map[string]bool{"firebolt": true, "magic_missile": true, "fireball": false, "mend": false, "wrath": false} {
		if got := hasAccess(c, shippedSpell(t, w, id)); got != want {
			t.Errorf("no magic, %s: access %v, want %v", id, got, want)
		}
	}
	c.Deity = "evil"
	for id, want := range map[string]bool{"mend": true, "wrath": true, "bless": true, "blight": true, "sanctuary": false} {
		if got := hasAccess(c, shippedSpell(t, w, id)); got != want {
			t.Errorf("evil apostle, %s: access %v, want %v", id, got, want)
		}
	}
	if !knowsSpell(c, shippedSpell(t, w, "wrath")) || knowsSpell(c, shippedSpell(t, w, "bless")) {
		t.Errorf("an apostle knows Wrath at once and buys Bless")
	}
	if knowsSpell(c, shippedSpell(t, w, "firebolt")) {
		t.Errorf("a common arcane spell is learned, not known")
	}
	if why := w.featBlocker(c, Feat{Branch: "arcane"}); why != "needs an arcane spell" {
		t.Errorf("arcane feat without an arcane spell: %q", why)
	}
	learnSpell(c, shippedSpell(t, w, "firebolt"))
	c.Stats["intelligence"] = 11
	if why := w.featBlocker(c, Feat{Branch: "arcane"}); why != "" {
		t.Errorf("arcane feat with Firebolt learned: %q", why)
	}
}

// TestWrathTakesItsGodsKind: the common divine strike hurts with the
// kind of the caster's god, so a creature immune to holy fire still
// takes an evil apostle's Wrath.
func TestWrathTakesItsGodsKind(t *testing.T) {
	w := shippedWorld(t)
	wrath := shippedSpell(t, w, "wrath")
	target := w.simCharacter(w.content.Mobs[910]) // a player's resistance stops short of immunity
	target.Effects = append(target.Effects, effect.Active{Spec: effect.Spec{Kind: "resist", Params: map[string]any{"kind": "holy", "amount": 1.0}}})
	w.recalc(target)
	good := casterFor(t, w)
	good.Deity = "good"
	evil := casterFor(t, w)
	evil.Deity = "evil"
	if d, _, _ := meanCast(w, good, target, wrath, 50); d != 0 {
		t.Fatalf("holy wrath hurt a holy-immune target: %.1f", d)
	}
	if d, _, _ := meanCast(w, evil, target, wrath, 50); d <= 0 {
		t.Fatalf("unholy wrath did nothing")
	}
}

// TestEarlySpellsHoldAndGrowSlowly: a spell of level 5 or below is not
// broken by blows, and at level 20 it hits about 60 percent as hard as
// it would on the full curve (RULES 6.4, 2026-10-12).
func TestEarlySpellsHoldAndGrowSlowly(t *testing.T) {
	w := shippedWorld(t)
	for _, id := range []string{"firebolt", "chill_touch", "mend", "wrath", "rotting_touch"} {
		if shippedSpell(t, w, id).InterruptOnDamage {
			t.Errorf("%s is early and should hold against blows", id)
		}
	}
	if !shippedSpell(t, w, "lightning_bolt").InterruptOnDamage {
		t.Errorf("lightning bolt is not early and should break")
	}
	// Firebolt against Lightning Bolt (2.0x against 2.5x damage, same save)
	// at level 1 and level 20: the early spell falls behind with level.
	ratio := func(level int) float64 {
		c := w.simFighter(level)
		c.Stats["intelligence"] = 20
		w.recalc(c)
		target := w.simFighter(level)
		fb, _, _ := meanCast(w, c, target, shippedSpell(t, w, "firebolt"), 400)
		lb, _, _ := meanCast(w, c, target, shippedSpell(t, w, "lightning_bolt"), 400)
		return (fb / 2.0) / (lb / 2.5)
	}
	if r := ratio(1); r < 0.95 || r > 1.05 {
		t.Fatalf("at level 1 the curves agree: ratio %.2f", r)
	}
	if r := ratio(20); r < 0.55 || r > 0.66 {
		t.Fatalf("at level 20 an early spell hits about 60%% as hard: ratio %.2f", r)
	}
}
