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

// A weapon's added damage rides its own swings, scaled and reduced as
// the swing is, each part resisted by its own kind. Here the fire is the
// size of the blade's own swing, so it doubles the hit.
func TestAddedDamageRidesItsWeapon(t *testing.T) {
	w, att, def := resistFixture(t)
	blade := plainBlade(t, w)
	att.Equipment["wield"] = blade
	base, _ := swingWith(t, w, att, def, blade, 1500)
	share := math.Pow(1.07/1.10, 9)
	near := func(got, want float64) bool { return got > 0.88*want && got < 1.12*want }

	blade.Effects = []effect.Active{added("fire", share, 10)}
	if got, _ := swingWith(t, w, att, def, blade, 1500); !near(got, 2*base) {
		t.Fatalf("with fire: %.1f a swing, want about twice the plain %.1f", got, base)
	}
	def.Effects = []effect.Active{resist("fire", 1)}
	if got, _ := swingWith(t, w, att, def, blade, 1500); !near(got, base) {
		t.Errorf("fire-immune: %.1f a swing, want about the plain %.1f", got, base)
	}
	def.Effects = []effect.Active{resist("slash", 1)}
	if got, resisted := swingWith(t, w, att, def, blade, 1500); !near(got, base) || resisted != 0 {
		t.Errorf("immune to the blade: %.1f a swing (%d shrugged off), want about %.1f of fire", got, resisted, base)
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
	if got, _ := swingWith(t, w, att, def, blade, 1500); got > 1.12*base {
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
	if err := w.scripts.Call("describeEffect", &words, map[string]any{"kind": "added", "params": map[string]any{"kind": "fire", "power": 0.5, "level": 10}, "rounds": 0}); err != nil || words != "Every hit adds about 7 fire damage" {
		t.Errorf("describe: %q (%v)", words, err)
	}
}

// A weapon's own damage rides the flatter curve (5.2): a plain level-60
// sword is about a fifth of the power unit.
func TestWeaponsRideTheFlatterCurve(t *testing.T) {
	w := shippedWorld(t)
	for _, L := range []int{1, 20, 60} {
		var r struct {
			Weapon struct {
				Damage float64 `json:"damage"`
			} `json:"weapon"`
		}
		if err := w.scripts.Call("itemBaseline", &r, map[string]any{"type": "weapon", "level": L, "baseline": "standard"}); err != nil {
			t.Fatal(err)
		}
		if want := 6 * math.Pow(1.07, float64(L-1)); math.Abs(r.Weapon.Damage-want) > 0.01*want {
			t.Errorf("level %d sword: %.1f, want %.1f", L, r.Weapon.Damage, want)
		}
	}
}

// An armed mob keeps power-unit damage (8): with a plain weapon of its
// level it hits as hard as the power unit, harder than a player with the
// same weapon by the share the weapon curve lost.
func TestArmedMobKeepsPowerUnitDamage(t *testing.T) {
	w, player, def := resistFixture(t)
	var mobAtt *Character
	for _, p := range w.content.Mobs {
		if p.Level == 10 && !p.HasFlag("peaceful") && len(p.Effects) == 0 && p != def.mob.Proto {
			mobAtt = w.newMob(p).Character
			break
		}
	}
	mobAtt.Effects = nil
	mobAtt.Stats = copyStats(player.Stats)
	blade := plainBlade(t, w)
	mobAtt.Equipment["wield"] = blade
	player.Equipment["wield"] = blade
	w.recalc(mobAtt)
	w.recalc(player)
	mine, _ := swingWith(t, w, player, def, blade, 1500)
	theirs, _ := swingWith(t, w, mobAtt, def, blade, 1500)
	if r, want := theirs/mine, 1/math.Pow(1.07/1.10, 9); r < 0.9*want || r > 1.1*want {
		t.Errorf("armed mob %.1f against a player's %.1f: ratio %.2f, want about %.2f", theirs, mine, r, want)
	}
}

// Every named weapon from item level 5 carries added damage making up
// what the flatter curve took from it (5.3).
func TestNamedWeaponsAreMadeWhole(t *testing.T) {
	w := shippedWorld(t)
	for _, p := range w.content.Items {
		if p.Type != item.Weapon || len(p.Effects) == 0 || p.Level < 5 {
			continue
		}
		total := 0.0
		for _, e := range p.Effects {
			if e.Kind == "added" {
				total += toFloat(e.Params["power"])
			}
		}
		if want := 1 - math.Pow(1.07/1.10, float64(p.Level-1)); total < want-0.015 {
			t.Errorf("item %d (%s), level %d: added damage %.2f, want at least %.2f", p.Vnum, p.Name, p.Level, total, want)
		}
	}
}
