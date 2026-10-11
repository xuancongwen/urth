package world

import (
	"strings"
	"testing"

	"urth/internal/mob"
)

func TestGeneratorSpawnsUntilItsCap(t *testing.T) {
	w := testWorld(t)
	alice := login(t, w, 1, "Alice")
	hub, _ := w.content.Rooms.Get(1)
	var guard *Mob
	for _, m := range w.contents(hub).mobs {
		if m.Proto.Vnum == 20 {
			guard = m
		}
	}
	if guard == nil {
		t.Fatal("no guard in the hub")
	}
	guard.Proto.Spawns = &mob.Spawn{Mob: 21, Every: 2, Max: 2}
	dogs := func() int {
		n := 0
		for _, m := range w.contents(hub).mobs {
			if m.Proto.Vnum == 21 {
				n++
			}
		}
		return n
	}
	alice.take()

	w.tickGenerators() // the first spawn comes at once
	if dogs() != 1 {
		t.Fatalf("after the first round: %d dogs, want 1", dogs())
	}
	w.flush()
	if o := alice.take(); !strings.Contains(o, "lurches out of") {
		t.Fatalf("room not told: %q", o)
	}
	w.tickGenerators()
	if dogs() != 1 {
		t.Fatalf("spawned before its timer: %d dogs", dogs())
	}
	for range 10 {
		w.tickGenerators()
	}
	if dogs() != 2 {
		t.Fatalf("after ten more rounds: %d dogs, want the cap of 2", dogs())
	}

	for _, m := range append([]*Mob(nil), w.contents(hub).mobs...) {
		w.removeMobFromRoom(m)
	}
	for range 10 {
		w.tickGenerators()
	}
	if dogs() != 0 {
		t.Fatalf("a dead generator still spawns: %d dogs", dogs())
	}
}
