package world

import (
	"math"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
)

// The honed and reinforced affixes (docs/RULES.md 5.3) add a tenth of a
// weapon's own damage or a piece's own defense, at least 1, each roll on
// its own, against the shipped rules.

func fraction(kind string) effect.Active {
	return effect.Active{Spec: effect.Spec{Kind: kind, Params: map[string]any{"fraction": 0.1}}}
}

func TestBaseDamageAndDefenseAffixes(t *testing.T) {
	w := shippedWorld(t)
	own := func(fn string, it *item.Item) float64 {
		t.Helper()
		var v float64
		if err := w.scripts.Call(fn, &v, itemView(it)); err != nil {
			t.Fatal(err)
		}
		return v
	}
	near := func(got, want float64) bool { return math.Abs(got-want) < 1e-6 }

	blade := plainBlade(t, w)
	d := blade.Proto.Resolved.Weapon.Damage
	if got := own("weaponDamage", blade); !near(got, d) {
		t.Fatalf("plain blade: %v, want %v", got, d)
	}
	blade.Effects = append(blade.Effects, fraction("baseDamage"))
	if got, want := own("weaponDamage", blade), d+math.Max(1, d*0.1); !near(got, want) {
		t.Errorf("honed blade: %v, want %v", got, want)
	}
	blade.Effects = append(blade.Effects, fraction("baseDamage"))
	if got, want := own("weaponDamage", blade), d+2*math.Max(1, d*0.1); !near(got, want) {
		t.Errorf("twice-honed blade: %v, want %v", got, want)
	}

	var body *item.Item
	for _, p := range w.content.Items {
		if p.Type == item.Armor && p.Slot == "body" && p.Level == 1 && len(p.Effects) == 0 && p.Resolved.Armor.Defense > 0 {
			body = item.New(p)
			break
		}
	}
	if body == nil {
		t.Fatal("no plain level-1 body armor")
	}
	D := body.Proto.Resolved.Armor.Defense
	if D*0.1 >= 1 {
		t.Fatalf("level-1 body armor has defense %v; the floor of 1 is not tested", D)
	}
	body.Effects = append(body.Effects, fraction("baseDefense"))
	if got := own("armorDefense", body); !near(got, D+1) {
		t.Errorf("reinforced level-1 armor: %v, want %v (the floor of 1)", got, D+1)
	}
}
