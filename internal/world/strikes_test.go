package world

import (
	"math/rand/v2"
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/mob"
)

// meanSkill is the mean damage of n uses of the shipped skill id by user
// on target at the given rating, with a fixed seed. Refused uses fail.
func meanSkill(t *testing.T, w *World, user, target *Character, id string, rating float64, n int) float64 {
	t.Helper()
	var sk Skill
	for _, s := range w.skillList() {
		if s.ID == id {
			sk = s
		}
	}
	if sk.ID == "" {
		t.Fatalf("no skill %q", id)
	}
	e := ensureSkill(user, sk)
	e.State["effectiveness"] = rating
	w.scripts.SetRandom(rand.New(rand.NewPCG(3, 3)))
	defer w.scripts.SetRandom(w.rng)
	total := 0.0
	for i := 0; i < n; i++ {
		var r SkillResult
		if err := w.scripts.Call("useSkill", &r, w.view(user), w.view(target), skillView(sk), e.View()); err != nil {
			t.Fatal(err)
		}
		if !r.OK {
			t.Fatalf("%s refused: %s", id, r.Message)
		}
		total += float64(r.Damage)
	}
	return total / float64(n)
}

// meanRound is the mean damage of n rounds of att's auto-attack on def
// (its weapon's swings per round), with a fixed seed.
func meanRound(w *World, att, def *Character, n int) float64 {
	w.scripts.SetRandom(rand.New(rand.NewPCG(3, 3)))
	defer w.scripts.SetRandom(w.rng)
	total := 0.0
	for i := 0; i < n; i++ {
		att.swing += att.Speed
		for att.swing >= 1 {
			att.swing--
			total += float64(w.resolveAttack(att, def, "wield", 1).Damage)
		}
	}
	return total / float64(n)
}

func shippedMob(t *testing.T, w *World, match func(p *mob.Proto) bool) *Character {
	t.Helper()
	for _, p := range w.content.Mobs {
		if match(p) {
			c := w.newMob(p).Character
			c.Health = c.HealthMax
			return c
		}
	}
	t.Fatal("no such mob in the shipped world")
	return nil
}

// Damage skills (RULES 7.4, 2026-10-11): Kick is worth well over a round
// of the user's own weapon, a fresh one still lands at least half of
// that, and both follow a better weapon; Bash's damage stays a small
// flat aside.
func TestStrikesScaleUtilityDoesNot(t *testing.T) {
	w := shippedWorld(t)
	user := w.simFighter(10)
	target := shippedMob(t, w, func(p *mob.Proto) bool { return p.Level == 10 && !p.HasFlag("peaceful") })

	round := meanRound(w, user, target, 2000)
	kick := meanSkill(t, w, user, target, "kick", 100, 2000)
	fresh := meanSkill(t, w, user, target, "kick", 30, 2000)
	bash := meanSkill(t, w, user, target, "bash", 100, 2000)
	if r := kick / round; r < 1.5 || r > 2.2 {
		t.Errorf("kick at 100: %.1f against a round's %.1f, ratio %.2f, want 1.5 to 2.2", kick, round, r)
	}
	if r := fresh / kick; r < 0.6 || r > 0.7 {
		t.Errorf("kick at 30: %.1f, %.2f of full, want about 0.65", fresh, r)
	}
	if r := bash / round; r > 0.6 {
		t.Errorf("bash: %.1f against a round's %.1f, ratio %.2f: utility damage has grown", bash, round, r)
	}

	// Twice the weapon: Kick follows it, Bash does not.
	old := user.Equipment["wield"]
	better := *old.Proto
	better.Resolved.Weapon.Damage *= 2
	user.Equipment["wield"] = item.New(&better)
	kick2 := meanSkill(t, w, user, target, "kick", 100, 2000)
	bash2 := meanSkill(t, w, user, target, "bash", 100, 2000)
	if r := kick2 / kick; r < 1.8 {
		t.Errorf("kick with twice the weapon: %.1f -> %.1f, ratio %.2f", kick, kick2, r)
	}
	if r := bash2 / bash; r > 1.1 {
		t.Errorf("bash with twice the weapon: %.1f -> %.1f, ratio %.2f", bash, bash2, r)
	}
	t.Logf("level 10, standard kit, level-10 mob: a round %.1f, kick %.1f (at 30: %.1f; twice the weapon: %.1f), bash %.1f", round, kick, fresh, kick2, bash)
}

// Disarm works on a mob whose natural attack is a weapon, which then hits
// for half, and is refused against a bite.
func TestDisarmArmedNaturalAttack(t *testing.T) {
	w := shippedWorld(t)
	user := w.simFighter(10)
	user.Stats["dexterity"] = 14
	verb := func(v string) func(p *mob.Proto) bool {
		return func(p *mob.Proto) bool { return p.Resolved.Attack.Verb == v && !p.HasFlag("peaceful") }
	}
	swordsman := shippedMob(t, w, verb("slash"))
	wolf := shippedMob(t, w, verb("bite"))

	var disarm Skill
	for _, s := range w.skillList() {
		if s.ID == "disarm" {
			disarm = s
		}
	}
	e := ensureSkill(user, disarm)
	try := func(target *Character) SkillResult {
		var r SkillResult
		if err := w.scripts.Call("useSkill", &r, w.view(user), w.view(target), skillView(disarm), e.View()); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := try(wolf); r.OK || !strings.Contains(r.Message, "nothing to disarm") {
		t.Fatalf("disarm on a wolf: %+v", r)
	}
	// Dodges and blocks aside, a use on the swordsman goes ahead.
	if r := try(swordsman); !r.OK {
		t.Fatalf("disarm on a swordsman: %+v", r)
	}

	armed := meanRound(w, swordsman, user, 2000)
	swordsman.Effects = append(swordsman.Effects, effect.Active{Spec: effect.Spec{Kind: "disarmed", Params: map[string]any{}}, Rounds: 3})
	bare := meanRound(w, swordsman, user, 2000)
	if r := bare / armed; r < 0.4 || r > 0.6 {
		t.Fatalf("disarmed swordsman: %.1f -> %.1f a round, ratio %.2f, want about 0.5", armed, bare, r)
	}
}

// Rend is taught somewhere a player can walk to, not only in the lab.
func TestRendIsTaughtInTheWorld(t *testing.T) {
	w := shippedWorld(t)
	for _, p := range w.content.Mobs {
		if p.Vnum >= 900 && p.Vnum < 1000 {
			continue // the balance lab
		}
		for _, s := range p.Teaches {
			if s == "rend" {
				return
			}
		}
	}
	t.Fatal("no trainer outside the balance lab teaches rend")
}
