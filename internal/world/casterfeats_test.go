package world

import (
	"math"
	"math/rand/v2"
	"testing"
)

// casterFor is a level-10 caster under the shipped rules, with 16 in both
// casting stats and the given feats taken.
func casterFor(t *testing.T, w *World, feats ...string) *Character {
	t.Helper()
	c := w.simFighter(10)
	c.Stats["intelligence"], c.Stats["wisdom"] = 16, 16
	c.Schools = []string{"evocation"}
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
	if mend := w.castCost(fast, shippedSpell(t, w, "mend")); !mend.InterruptOnDamage || mend.Mana != 8 {
		t.Fatalf("arcane feats touched a divine spell: %+v", mend)
	}
	if got, want := fast.ManaMax, int(math.Round(float64(plain.ManaMax)*1.25)); got < want-1 || got > want+1 {
		t.Fatalf("reservoir: %d, want about %d (plain %d)", got, want, plain.ManaMax)
	}

	target := w.simFighter(10)
	firebolt := shippedSpell(t, w, "firebolt")
	base, _, _ := meanCast(w, plain, target, firebolt, 400)
	focused, _, _ := meanCast(w, casterFor(t, w, "focus_evocation", "empower"), target, firebolt, 400)
	if r := focused / base; r < 1.30 || r > 1.45 {
		t.Fatalf("focus and empower: %.0f -> %.0f, ratio %.2f, want about 1.38", base, focused, r)
	}
	otherSchool, _, _ := meanCast(w, casterFor(t, w, "focus_illusion"), target, firebolt, 400)
	if r := otherSchool / base; r < 0.97 || r > 1.03 {
		t.Fatalf("illusion focus changed a firebolt: ratio %.2f", r)
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
