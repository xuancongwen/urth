package world

import (
	"strings"
	"testing"
	"time"
)

func TestBuilderSnapshotAndArea(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	snap := w.builderSnapshot()
	if len(snap.Areas) != 2 || snap.Areas[0].Name != "a" || snap.Areas[0].Rooms != 2 || snap.Areas[0].Mobs != 2 || snap.Areas[1].Rooms != 4 {
		t.Fatalf("areas: %+v", snap.Areas)
	}
	if snap.StartRoom != 1 {
		t.Fatalf("start room: %d", snap.StartRoom)
	}
	// The test world's a/1 holds a guard with a sword and a sack, and Bob.
	view, err := w.builderArea("a")
	if err != nil {
		t.Fatal(err)
	}
	hub := view.Rooms[0]
	if hub.Vnum != 1 || len(hub.Players) != 1 || hub.Players[0] != "Bob" || len(hub.Mobs) != 1 || !strings.Contains(hub.Mobs[0], "guard") {
		t.Fatalf("hub: %+v", hub)
	}
	if !hub.Placed || hub.Exits["north"] != 2 || hub.File == "" {
		t.Fatalf("hub layout: %+v", hub)
	}
	north := view.Rooms[1]
	if north.Links["east"].Area != "b" || north.Links["east"].Name != "Elsewhere" {
		t.Fatalf("cross-area link: %+v", north.Links)
	}
	var sword, sack, guard, dog int
	for _, p := range view.Items {
		switch p.Vnum {
		case 10:
			sword = p.Live
		case 12:
			sack = p.Live
		}
	}
	for _, p := range view.Mobs {
		switch p.Vnum {
		case 20:
			guard = p.Live
		case 21:
			dog = p.Live
		}
	}
	// The sword is wielded by the guard: carried items count too.
	if sword != 1 || sack != 1 || guard != 1 || dog != 1 {
		t.Fatalf("live counts: sword %d sack %d guard %d dog %d", sword, sack, guard, dog)
	}
	if len(view.Resets) != 6 || view.Resets[0].Name != "a city guard" || view.Resets[0].RoomName != "Hub" || len(view.Resets[0].Equip) != 2 || view.Resets[0].Equip[0].Slot != "wield" {
		t.Fatalf("resets: %+v", view.Resets)
	}
	if _, err := w.builderArea("zz"); err == nil {
		t.Fatal("unknown area accepted")
	}
}

func TestBuilderAPIRunsOnTheWorldGoroutine(t *testing.T) {
	w := testWorld(t)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				w.Tick()
				time.Sleep(time.Millisecond)
			}
		}
	}()
	defer func() { close(stop); <-done }()

	api := w.Builder()
	if api.Generation() != 0 {
		t.Fatalf("generation: %d", api.Generation())
	}
	snap, err := api.Snapshot()
	if err != nil || len(snap.Areas) != 2 {
		t.Fatalf("snapshot: %v %+v", err, snap)
	}
	report, err := api.Reload("a")
	if err != nil || !strings.Contains(report, "Area a reloaded") {
		t.Fatalf("reload: %v %q", err, report)
	}
	if api.Generation() != 1 {
		t.Fatalf("generation after reload: %d", api.Generation())
	}
	if _, err := api.Area("nowhere"); err == nil {
		t.Fatal("unknown area accepted")
	}
}
