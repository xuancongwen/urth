package world

import (
	"strings"
	"testing"
)

// settleRules counts onTick calls per character so a test can see which
// characters regen asked about.
var settleRules = strings.Replace(testRules,
	"function onTick(c) { return { healthDelta: c.fighting ? 0 : 1, manaDelta: 0 }; }",
	`var ticks = {};
function onTick(c) { ticks[c.name] = (ticks[c.name] || 0) + 1; return { healthDelta: c.fighting ? 0 : 1, manaDelta: 0 }; }
function tickCount(name) { return ticks[name] || 0; }`, 1)

// A mob onTick leaves unchanged is not asked again until something it
// sees changes: a wound, a fight, or new rules.
func TestSettledMobSkipsOnTick(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	if !strings.Contains(settleRules, "tickCount") {
		t.Fatal("settleRules did not replace onTick")
	}
	setRules(t, w, settleRules)
	guard := w.contents(p.Room).mobs[0]
	guard.Health = guard.HealthMax
	calls := func() int {
		t.Helper()
		var n int
		if err := w.scripts.Call("tickCount", &n, guard.Name); err != nil {
			t.Fatal(err)
		}
		return n
	}

	w.regen()
	w.regen()
	w.regen()
	if n := calls(); n != 1 || !guard.settled {
		t.Fatalf("at full health: %d calls, settled %v; want 1 call, settled", n, guard.settled)
	}

	// Wounded, it heals a point a round, then settles again.
	guard.Health -= 2
	for range 5 {
		w.regen()
	}
	if guard.Health != guard.HealthMax || !guard.settled {
		t.Fatalf("after a wound: health %d/%d, settled %v", guard.Health, guard.HealthMax, guard.settled)
	}
	if n := calls(); n != 4 {
		t.Fatalf("after a wound: %d calls, want 4 (one idle, two healing, one to settle)", n)
	}

	// A fight keeps it ticking.
	guard.Fighting = p.Character
	w.regen()
	w.regen()
	if n := calls(); n != 6 || guard.settled {
		t.Fatalf("fighting: %d calls, settled %v; want 6, not settled", n, guard.settled)
	}
	guard.Fighting = nil

	// New rules: asked again, once. The counter starts over with them.
	setRules(t, w, settleRules)
	w.regen()
	w.regen()
	if n := calls(); n != 1 {
		t.Fatalf("after a rules reload: %d calls, want 1", n)
	}
}
