package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGotoAtTransfer(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob") // admin
	alice := login(t, w, 2, "Alice")
	bob.take()
	alice.take()

	send(w, 1, "goto 3")
	if o := bob.take(); !strings.Contains(o, "Elsewhere") {
		t.Fatalf("goto vnum: %q", o)
	}
	if o := alice.take(); !strings.Contains(o, "Bob disappears in a puff of smoke.") {
		t.Fatalf("goto not seen: %q", o)
	}
	send(w, 1, "goto alice")
	if o := bob.take(); !strings.Contains(o, "Alice is here.") {
		t.Fatalf("goto player: %q", o)
	}
	send(w, 1, "goto dog")
	if o := bob.take(); !strings.Contains(o, "A stray dog sniffs about.") {
		t.Fatalf("goto mob: %q", o)
	}
	send(w, 1, "goto 999")
	if o := bob.take(); !strings.Contains(o, "No such location") {
		t.Fatalf("goto bad: %q", o)
	}

	send(w, 1, "at 1 say boo")
	alice.take()
	send(w, 2, "say hi") // alice still in room 1? she never moved; check she heard bob
	if o := bob.take(); !strings.Contains(o, "You say 'boo'") || strings.Contains(o, "Alice says") {
		t.Fatalf("at should return bob to room 2: %q", o)
	}
	if w.players[1].Room.Vnum != 2 {
		t.Fatal("at did not restore room")
	}

	send(w, 1, "transfer alice")
	if o := alice.take(); !strings.Contains(o, "Bob has transferred you.") || !strings.Contains(o, "North") {
		t.Fatalf("transfer: %q", o)
	}
	send(w, 1, "transfer alice 3")
	if w.players[2].Room.Vnum != 3 {
		t.Fatal("transfer to vnum failed")
	}
}

func TestStat(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "stat")
	if o := bob.take(); !strings.Contains(o, "Vnum: 1") || !strings.Contains(o, "north->2") || !strings.Contains(o, "a city guard[20]") || !strings.Contains(o, "a small sack[12]") {
		t.Fatalf("stat room: %q", o)
	}
	send(w, 1, "stat guard")
	if o := bob.take(); !strings.Contains(o, "Mob vnum 20") || !strings.Contains(o, "<wielded>") || strings.Contains(o, "Flags:") {
		t.Fatalf("stat mob: %q", o)
	}
	send(w, 1, "stat sack")
	if o := bob.take(); !strings.Contains(o, "Type: container") || !strings.Contains(o, "Contains: a loaf of bread[13]") {
		t.Fatalf("stat item: %q", o)
	}
	send(w, 1, "stat bob")
	if o := bob.take(); !strings.Contains(o, "Player") || !strings.Contains(o, "Level: 1") {
		t.Fatalf("stat self: %q", o)
	}
	send(w, 1, "stat 2")
	if o := bob.take(); !strings.Contains(o, "Name: North") {
		t.Fatalf("stat remote room: %q", o)
	}
}

func TestLoadPurgeForceRestorePeace(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	bob.take()
	alice.take()

	send(w, 1, "load mob 21")
	if o := bob.take(); !strings.Contains(o, "You create a stray dog.") {
		t.Fatalf("load mob: %q", o)
	}
	if o := alice.take(); !strings.Contains(o, "Bob has created a stray dog!") {
		t.Fatalf("load mob not seen: %q", o)
	}
	send(w, 1, "load obj 10")
	if o := bob.take(); !strings.Contains(o, "You create a rusty sword.") {
		t.Fatalf("load obj: %q", o)
	}
	if len(w.players[1].Inventory) != 1 {
		t.Fatal("loaded item not in inventory")
	}
	send(w, 1, "load obj 14") // nopickup lands on the floor
	bob.take()
	altars := 0
	for _, it := range w.contents(w.players[1].Room).items {
		if it.Proto.Vnum == 14 {
			altars++
		}
	}
	if altars != 2 {
		t.Fatalf("nopickup load should go to the room; altars=%d", altars)
	}
	send(w, 1, "load obj 999")
	if o := bob.take(); !strings.Contains(o, "No item has that vnum") {
		t.Fatalf("load bad: %q", o)
	}

	send(w, 1, "force alice say forced")
	if o := alice.take(); !strings.Contains(o, "Bob forces you to 'say forced'") || !strings.Contains(o, "You say 'forced'") {
		t.Fatalf("force: %q", o)
	}
	send(w, 2, "kill dog")
	alice.take()
	send(w, 1, "peace")
	if w.players[2].Fighting != nil {
		t.Fatal("peace did not stop the fight")
	}
	w.players[2].Health = 1
	send(w, 1, "restore alice")
	if o := alice.take(); !strings.Contains(o, "Bob has restored you.") || w.players[2].Health != w.players[2].HealthMax {
		t.Fatalf("restore: %q hp=%d", o, w.players[2].Health)
	}

	send(w, 1, "purge dog")
	if o := bob.take(); !strings.Contains(o, "You purge a stray dog.") {
		t.Fatalf("purge target: %q", o)
	}
	send(w, 1, "purge alice")
	if o := bob.take(); !strings.Contains(o, "can't purge players") {
		t.Fatalf("purge player: %q", o)
	}
	send(w, 1, "purge")
	if o := bob.take(); !strings.Contains(o, "Ok.") {
		t.Fatalf("purge room: %q", o)
	}
	c := w.contents(w.players[1].Room)
	if len(c.mobs) != 0 || len(c.items) != 0 {
		t.Fatalf("room not purged: mobs=%d items=%d", len(c.mobs), len(c.items))
	}
	if len(w.players[1].Inventory) != 1 {
		t.Fatal("purge room should not touch inventories")
	}
	if _, still := w.players[2]; !still {
		t.Fatal("purge removed a player")
	}
}

func TestReloadAreaFromDisk(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	send(w, 2, "n")
	send(w, 1, "get cap")
	bob.take()
	alice.take()
	guard := w.contents(w.players[1].Room).mobs[0]

	// Edit on disk: rename the hub, add a room west of it, change the
	// cap's name, and remove the dog prototype (and its reset).
	dir := w.contentDir
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a/rooms/1.yaml", "vnum: 1\nname: Renamed Hub\ndescription: Freshly painted.\nexits:\n  north: 2\n  west: 4\n")
	write("a/rooms/4.yaml", "vnum: 4\nname: New Wing\ndescription: Smells of sawdust.\nexits:\n  east: 1\n")
	write("a/items/11.yaml", "vnum: 11\nname: a leather helm\nkeywords: [leather, helm, cap]\ntype: armor\nslot: head\narmor:\n  defense: 1\n")
	os.Remove(filepath.Join(dir, "a/mobs/21.yaml"))
	write("a/resets.yaml", "interval_seconds: 5\nresets:\n  - mob: 20\n    room: 1\n    equip:\n      - item: 10\n        slot: wield\n  - item: 12\n    room: 1\n  - item: 11\n    room: 4\n")

	send(w, 1, "reload area a")
	o := bob.take()
	if !strings.Contains(o, "Area a reloaded") {
		t.Fatalf("reload: %q", o)
	}
	send(w, 1, "look")
	if o := bob.take(); !strings.Contains(o, "Renamed Hub") || !strings.Contains(o, "[Exits: north west]") {
		t.Fatalf("room edit not live: %q", o)
	}
	send(w, 1, "i")
	if o := bob.take(); !strings.Contains(o, "a leather helm") {
		t.Fatalf("carried item not rebound to new prototype: %q", o)
	}
	send(w, 1, "w")
	if o := bob.take(); !strings.Contains(o, "New Wing") || !strings.Contains(o, "leather helm is here") {
		t.Fatalf("new room and its reset item missing: %q", o)
	}
	if w.countMobs(w.content.Mobs[20]) != 1 || guard.Proto != w.content.Mobs[20] {
		t.Fatal("guard not rebound or duplicated")
	}
	if len(w.contents(w.content.Rooms.Rooms[2]).mobs) != 0 {
		t.Fatal("dog with removed prototype still exists")
	}
	if w.players[2].Room != w.content.Rooms.Rooms[2] {
		t.Fatal("alice not re-pointed to the new room object")
	}

	// A broken file keeps the old world.
	write("a/rooms/1.yaml", "vnum: 1\nname: Broken\nexits:\n  south: 999\n")
	send(w, 1, "reload world")
	if o := bob.take(); !strings.Contains(o, "Reload failed") {
		t.Fatalf("bad reload accepted: %q", o)
	}
	send(w, 1, "e")
	if o := bob.take(); !strings.Contains(o, "Renamed Hub") {
		t.Fatalf("old world not kept: %q", o)
	}

	// Removing the room a player stands in moves them to the start room.
	write("a/rooms/1.yaml", "vnum: 1\nname: Renamed Hub\ndescription: Freshly painted.\nexits:\n  north: 2\n")
	os.Remove(filepath.Join(dir, "a/rooms/4.yaml"))
	write("a/resets.yaml", "interval_seconds: 5\nresets:\n  - mob: 20\n    room: 1\n")
	send(w, 1, "goto 4")
	bob.take()
	send(w, 1, "reload world")
	if o := bob.take(); !strings.Contains(o, "The ground shifts") || !strings.Contains(o, "1 players moved") {
		t.Fatalf("player in removed room: %q", o)
	}
	if w.players[1].Room.Vnum != 1 {
		t.Fatal("not moved to start room")
	}
	send(w, 1, "reload area nowhere")
	if o := bob.take(); !strings.Contains(o, "No area named") {
		t.Fatalf("unknown area: %q", o)
	}
}

func TestBuilderCommandsHiddenFromPlayers(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	alice.take()
	for _, cmd := range []string{"goto 2", "at 2 look", "stat", "load mob 21", "purge", "force bob say x", "restore all", "transfer bob", "peace", "reload"} {
		send(w, 2, cmd)
		if o := alice.take(); !strings.Contains(o, "Huh?") {
			t.Errorf("%q ran for a non-admin: %q", cmd, o)
		}
	}
}
