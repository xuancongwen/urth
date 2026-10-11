package world

import (
	"strings"
	"testing"

	"urth/internal/mob"
	"urth/internal/output"
)

func TestExpeditionRun(t *testing.T) {
	w := testWorld(t)
	setRules(t, w, lootRulesJS)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.Level = 4 // 18 health against the dog's 12, and within reach of level 1
	w.recalc(p.Character)
	p.Health = p.HealthMax
	rules := w.expeditionRules(p.Character)

	send(w, 1, "expedition a")
	if o := bob.take(); !strings.Contains(o, "somewhere safe") {
		t.Fatalf("outside a safe room: %q", o)
	}
	send(w, 1, "expedition")
	if o := bob.take(); !strings.Contains(o, "a stray dog") && !strings.Contains(o, "  a ") {
		t.Fatalf("list: %q", o)
	}
	var listed []output.Expedition
	for _, m := range bob.batches[len(bob.batches)-1].Messages {
		if m.Type == output.Expeditions {
			listed = m.Data.([]output.Expedition)
		}
	}
	if len(listed) != 1 || listed[0].Area != "a" || listed[0].Safe {
		t.Fatalf("expeditions data: %+v", listed)
	}

	// Against the unarmed dog Bob wins every fight; same seed, same run.
	dog := []*mob.Proto{w.content.Mobs[21]}
	a := w.runExpedition(p.Character, dog, rules, 3, 42)
	b := w.runExpedition(p.Character, dog, rules, 3, 42)
	if a.Died || a.Kills != 3 || a.XP != 180 || len(a.Items) != 6 {
		t.Fatalf("run: %+v", a)
	}
	if strings.Join(a.Log, "\n") != strings.Join(b.Log, "\n") {
		t.Fatalf("same seed, different runs:\n%s\n---\n%s", strings.Join(a.Log, "\n"), strings.Join(b.Log, "\n"))
	}
	if !strings.Contains(strings.Join(a.Log, "\n"), "You sleep for") {
		t.Fatalf("never rested: %s", strings.Join(a.Log, "\n"))
	}
	// The guard's sword out-trades Bob's fists.
	guard := []*mob.Proto{w.content.Mobs[20]}
	if d := w.runExpedition(p.Character, guard, rules, 3, 1); !d.Died || d.Kills != 0 {
		t.Fatalf("against the guard: %+v", d)
	}

	// From the safe room, a run pays out and then owes rest.
	room, _ := w.content.Rooms.Get(3)
	p.Room = room
	w.content.Resets["a"].Resets = w.content.Resets["a"].Resets[1:] // the dog, not the guard
	xp := p.Experience
	send(w, 1, "expedition a 2 7")
	o := bob.take()
	if !strings.Contains(o, "You set out for") || !strings.Contains(o, "seed 7") || !strings.Contains(o, "2 kills") || p.Experience != xp+120 || len(p.Inventory) < 4 {
		t.Fatalf("expedition: %q xp %d inv %d", o, p.Experience-xp, len(p.Inventory))
	}
	send(w, 1, "expedition a")
	if o := bob.take(); !strings.Contains(o, "still recovering") {
		t.Fatalf("no rest owed: %q", o)
	}
}
