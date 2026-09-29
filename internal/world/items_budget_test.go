package world

import (
	"fmt"
	"testing"
)

// TestShippedItemBudget holds every item outside the balance lab to the
// power budget of the item pass (2026-09-28): caps per effect kind, no
// admin-only kinds, and no debuffs worn as gear.
func TestShippedItemBudget(t *testing.T) {
	w := shippedWorld(t)
	forbidden := map[string]bool{"attuned": true, "blind": true, "disarmed": true, "speedMult": true, "dot": true, "skill": true}
	f := func(params map[string]any, key string) float64 {
		switch v := params[key].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
		return 0
	}
	attacks := 0
	for vnum, p := range w.content.Items {
		if p.Area == "balance" {
			continue
		}
		fail := func(format string, args ...any) {
			t.Errorf("item %d (%s, %s): %s", vnum, p.Name, p.Area, fmt.Sprintf(format, args...))
		}
		if p.HasFlag("outfit") {
			fail("outfit is for admin regalia")
		}
		stats := 0.0
		for _, e := range p.Effects {
			pr := e.Params
			switch e.Kind {
			case "stat":
				if a := f(pr, "amount"); a > 0 {
					stats += a
				}
			case "crit":
				if f(pr, "chance") > 0.10 {
					fail("crit chance %v", f(pr, "chance"))
				}
			case "critMult":
				if f(pr, "amount") > 0.5 {
					fail("critMult %v", f(pr, "amount"))
				}
			case "dodge":
				if f(pr, "amount") > 0.05 {
					fail("dodge %v", f(pr, "amount"))
				}
			case "block":
				if f(pr, "chance") > 0.18 {
					fail("block %v", f(pr, "chance"))
				}
			case "shieldBlock":
				if f(pr, "chance") > 0.08 {
					fail("shieldBlock %v", f(pr, "chance"))
				}
			case "attacks":
				attacks++
				if f(pr, "amount") > 0.5 {
					fail("attacks %v", f(pr, "amount"))
				}
			case "protect":
				if f(pr, "mult") < 0.85 {
					fail("protect %v", f(pr, "mult"))
				}
			case "proc":
				chance := 1.0
				if _, ok := pr["chance"]; ok {
					chance = f(pr, "chance")
				}
				if chance*f(pr, "power") > 0.3001 {
					fail("proc chance*power %v", chance*f(pr, "power"))
				}
			case "ignite", "envenom":
				if f(pr, "power") > 0.15 || f(pr, "maxStacks") > 3 {
					fail("%s power %v stacks %v", e.Kind, f(pr, "power"), f(pr, "maxStacks"))
				}
			case "chill":
				if f(pr, "chance") > 0.25 || f(pr, "mult") < 0.5 {
					fail("chill %v/%v", f(pr, "chance"), f(pr, "mult"))
				}
			case "leech":
				if f(pr, "fraction") > 0.15 {
					fail("leech %v", f(pr, "fraction"))
				}
			case "thorns":
				if f(pr, "fraction") > 0.25 {
					fail("thorns %v", f(pr, "fraction"))
				}
			case "soak":
				if f(pr, "power") > 0.15 {
					fail("soak %v", f(pr, "power"))
				}
			case "lastStand":
				if f(pr, "mult") < 0.6 {
					fail("lastStand %v", f(pr, "mult"))
				}
			case "execute":
				if f(pr, "mult") > 1.5 {
					fail("execute %v", f(pr, "mult"))
				}
			case "regen":
				if f(pr, "fraction") > 0.02 {
					fail("regen %v", f(pr, "fraction"))
				}
			case "aura":
				if f(pr, "power") > 0.3 {
					fail("aura %v", f(pr, "power"))
				}
			case "damage":
				if f(pr, "mult") > 1.10 {
					fail("damage %v", f(pr, "mult"))
				}
			case "unarmedMult":
				if f(pr, "mult") > 1.25 {
					fail("unarmedMult %v", f(pr, "mult"))
				}
			case "cooldownCut":
				if f(pr, "rounds") > 1 {
					fail("cooldownCut %v", f(pr, "rounds"))
				}
			default:
				if forbidden[e.Kind] {
					fail("kind %s does not belong on gear", e.Kind)
				}
			}
		}
		for _, m := range p.Mods {
			if m > 0 {
				stats += float64(m)
			}
		}
		if stats > 6 {
			fail("+%v stats", stats)
		}
	}
	if attacks > 2 {
		t.Errorf("%d items grant extra attacks; the budget allows two legends", attacks)
	}
}
