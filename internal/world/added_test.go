package world

import (
	"math"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
)

// Added damage and effects sized at the item's level (docs/RULES.md 5.3),
// against the shipped rules.

// plainBlade is a fresh level-10 standard slashing weapon from the shipped
// world, with no effects of its own.
func plainBlade(t *testing.T, w *World) *item.Item {
	t.Helper()
	for _, p := range w.content.Items {
		if p.Type == item.Weapon && p.Level == 10 && p.Baseline == "standard" && p.Resolved.Weapon.Kind == "slash" && len(p.Effects) == 0 {
			return item.New(p)
		}
	}
	t.Fatal("no plain level-10 standard blade")
	return nil
}

func added(kind string, power float64, level int) effect.Active {
	p := map[string]any{"kind": kind, "power": power}
	if level > 0 {
		p["level"] = level
	}
	return effect.Active{Spec: effect.Spec{Kind: "added", Params: p}}
}

// swingWith is the mean of n landed swings by att with weapon, and how
// many were wholly resisted.
func swingWith(t *testing.T, w *World, att, def *Character, weapon *item.Item, n int) (mean float64, resisted int) {
	t.Helper()
	total, hits := 0, 0
	for i := 0; i < n; i++ {
		var r AttackResult
		if err := w.scripts.Call("resolveAttack", &r, w.view(att), w.view(def), itemView(weapon), 1, "wield"); err != nil {
			t.Fatal(err)
		}
		if !r.Hit {
			continue
		}
		hits++
		total += r.Damage
		if r.Resisted {
			resisted++
		}
	}
	return float64(total) / float64(max(hits, 1)), resisted
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

func unitAtLevel(L int) float64 { return 6 * math.Pow(1.10, float64(L-1)) }

// A weapon's added damage rides its own swings, past armor, each part
// resisted by its own kind.
func TestAddedDamageRidesItsWeapon(t *testing.T) {
	w, att, def := resistFixture(t)
	blade := plainBlade(t, w)
	att.Equipment["wield"] = blade
	base, _ := swingWith(t, w, att, def, blade, 800)
	fire := unitAtLevel(10)

	blade.Effects = []effect.Active{added("fire", 1, 10)}
	got, _ := swingWith(t, w, att, def, blade, 800)
	if d := got - base; d < 0.85*fire || d > 1.15*fire {
		t.Fatalf("fire adds %.1f a swing, want about %.1f", d, fire)
	}

	def.Effects = []effect.Active{resist("fire", 1)}
	if got, _ := swingWith(t, w, att, def, blade, 800); got < 0.85*base || got > 1.15*base {
		t.Errorf("fire-immune: %.1f a swing, want about the plain %.1f", got, base)
	}
	def.Effects = []effect.Active{resist("slash", 1)}
	got, resisted := swingWith(t, w, att, def, blade, 800)
	if got < 0.85*fire || got > 1.15*fire || resisted != 0 {
		t.Errorf("immune to the blade: %.1f a swing (%d shrugged off), want about %.1f of fire", got, resisted, fire)
	}
	def.Effects = []effect.Active{resist("slash", 1), resist("fire", 1)}
	if got, resisted := swingWith(t, w, att, def, blade, 100); got != 0 || resisted == 0 {
		t.Errorf("immune to both: %.1f a swing, %d shrugged off", got, resisted)
	}
	def.Effects = nil

	// On the other hand's weapon, it stays with that weapon.
	blade.Effects = nil
	dagger := plainBlade(t, w)
	dagger.Effects = []effect.Active{added("fire", 1, 10)}
	att.Equipment["second"] = dagger
	if got, _ := swingWith(t, w, att, def, blade, 800); got > 1.15*base {
		t.Errorf("the off hand's fire rode the main hand: %.1f a swing, plain %.1f", got, base)
	}
}

// A proc on an item is sized at the item's level, or the level its roll
// names; an attuned item's and the wielder's own at the wielder's.
func TestEffectsSizedAtItemLevel(t *testing.T) {
	w, att, def := resistFixture(t)
	att.Level = 30
	w.recalc(att)
	blade := plainBlade(t, w)
	att.Equipment["wield"] = blade
	proc := func(level int) effect.Active {
		p := map[string]any{"kind": "fire", "chance": 1.0, "power": 1.0, "verb": "flames"}
		if level > 0 {
			p["level"] = level
		}
		return effect.Active{Spec: effect.Spec{Kind: "proc", Params: p}}
	}
	procMean := func() float64 {
		total, n := 0, 0
		for i := 0; i < 400; i++ {
			var r AttackResult
			if err := w.scripts.Call("resolveAttack", &r, w.view(att), w.view(def), itemView(blade), 1, "wield"); err != nil {
				t.Fatal(err)
			}
			for _, pr := range r.Procs {
				total += pr.Damage
				n++
			}
		}
		return float64(total) / float64(max(n, 1))
	}
	for _, c := range []struct {
		name  string
		item  []effect.Active
		own   []effect.Active
		level int
	}{
		{"the item's level", []effect.Active{proc(0)}, nil, 10},
		{"the rolled level", []effect.Active{proc(20)}, nil, 20},
		{"attuned", []effect.Active{proc(0), {Spec: effect.Spec{Kind: "attuned"}}}, nil, 30},
		{"the wielder's own", nil, []effect.Active{proc(0)}, 30},
	} {
		blade.Effects, att.Effects = c.item, c.own
		want := unitAtLevel(c.level)
		if got := procMean(); got < 0.9*want || got > 1.1*want {
			t.Errorf("%s: proc %.1f, want about %.1f", c.name, got, want)
		}
	}
}

// The added-damage affixes roll on weapons, carry their roll level, and
// a full-quality roll makes up what the flatter weapon curve leaves out.
func TestAddedAffixRolls(t *testing.T) {
	w := shippedWorld(t)
	var share float64
	if err := w.scripts.Call("weaponShare", &share, 60); err != nil || share < 0.18 || share > 0.21 {
		t.Fatalf("weapon share at 60: %v (%v), want about a fifth", share, err)
	}
	blade := plainBlade(t, w)
	kinds := map[string]bool{}
	for i := 0; i < 2000; i++ {
		var r ItemRoll
		if err := w.scripts.Call("rollItem", &r, itemView(blade), map[string]any{"source": "drop", "level": 60}); err != nil {
			t.Fatal(err)
		}
		for _, e := range r.Effects {
			if e.Kind != "added" {
				continue
			}
			kind, _ := e.Params["kind"].(string)
			kinds[kind] = true
			power, level := toFloat(e.Params["power"]), toFloat(e.Params["level"])
			if power <= 0 || power > 1.4*(1-share)+0.01 || level != 60 {
				t.Fatalf("added affix %v", e.Params)
			}
		}
	}
	for _, k := range []string{"fire", "cold", "lightning", "poison"} {
		if !kinds[k] {
			t.Errorf("no %s affix in 2000 rolls", k)
		}
	}
	var words string
	if err := w.scripts.Call("describeEffect", &words, map[string]any{"kind": "added", "params": map[string]any{"kind": "fire", "power": 0.5, "level": 10}, "rounds": 0}); err != nil || words != "Every hit adds about 7 fire damage, past armor" {
		t.Errorf("describe: %q (%v)", words, err)
	}
}
