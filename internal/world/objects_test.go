package world

import (
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"urth/internal/item"
)

// The test world (see world_test.go) resets area "a" at boot: a sentinel
// guard wielding a rusty sword and carrying bread in room 1, up to two
// stray dogs in room 2, and in room 1 a sack containing bread, a leather
// cap, and an immovable altar.

func TestBootResetPopulates(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "look")
	out := bob.take()
	for _, want := range []string{"A city guard stands here.", "A small sack is here.", "A leather cap is here.", "A stone altar stands here."} {
		if !strings.Contains(out, want) {
			t.Errorf("room listing missing %q in %q", want, out)
		}
	}
	if w.countMobs(w.content.Mobs[21]) != 1 {
		t.Fatalf("expected one dog after boot reset, got %d", w.countMobs(w.content.Mobs[21]))
	}
	guard := w.contents(w.players[1].Room).mobs[0]
	if guard.Equipment["wield"] == nil || len(guard.Inventory) != 1 {
		t.Fatalf("guard not equipped: %+v", guard.Character)
	}
}

func TestResetRepopulatesOnSchedule(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "get cap")
	send(w, 1, "get bread sack")
	bob.take()
	if len(w.players[1].Inventory) != 2 {
		t.Fatalf("expected 2 items carried, got %d", len(w.players[1].Inventory))
	}
	room1 := w.players[1].Room
	if hasVnum(w.contents(room1).items, 11) {
		t.Fatal("cap still in room after get")
	}
	// interval 5s / round 1s = 5 rounds = 50 ticks
	for i := 0; i < 50; i++ {
		w.Tick()
	}
	if !hasVnum(w.contents(room1).items, 11) {
		t.Fatal("cap not repopulated by reset")
	}
	var sack *item.Item
	for _, it := range w.contents(room1).items {
		if it.Proto.Vnum == 12 {
			sack = it
		}
	}
	if sack == nil || !hasVnum(sack.Contents, 13) {
		t.Fatal("bread not repopulated inside sack")
	}
	if n := len(w.contents(room1).items); n != 3 {
		t.Fatalf("reset duplicated room items: %d", n)
	}
	if w.countMobs(w.content.Mobs[20]) != 1 {
		t.Fatal("reset duplicated the sentinel guard")
	}
	if w.countMobs(w.content.Mobs[21]) != 2 {
		t.Fatalf("expected dogs to reach max 2, got %d", w.countMobs(w.content.Mobs[21]))
	}
}

func TestGetDropWearWieldRemove(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	bob.take()
	alice.take()

	send(w, 1, "get altar")
	if out := bob.take(); !strings.Contains(out, "can't take") {
		t.Fatalf("nopickup ignored: %q", out)
	}
	send(w, 1, "get cap")
	if out := bob.take(); !strings.Contains(out, "You get a leather cap.") {
		t.Fatalf("get: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "Bob gets a leather cap.") {
		t.Fatalf("get not seen: %q", out)
	}
	send(w, 1, "wear cap")
	if out := bob.take(); !strings.Contains(out, "You wear a leather cap.") {
		t.Fatalf("wear: %q", out)
	}
	send(w, 1, "eq")
	if out := bob.take(); !strings.Contains(out, "<worn on head>") || !strings.Contains(out, "a leather cap") {
		t.Fatalf("equipment: %q", out)
	}
	send(w, 1, "i")
	if out := bob.take(); !strings.Contains(out, "Nothing.") {
		t.Fatalf("inventory should be empty after wearing: %q", out)
	}
	send(w, 1, "rem cap")
	send(w, 1, "drop cap")
	if out := bob.take(); !strings.Contains(out, "You stop using a leather cap.") || !strings.Contains(out, "You drop a leather cap.") {
		t.Fatalf("remove/drop: %q", out)
	}

	// Wield: the guard has the only sword, so give a second player none.
	send(w, 1, "wield sword")
	if out := bob.take(); !strings.Contains(out, "You don't have that.") {
		t.Fatalf("wield missing item: %q", out)
	}
	send(w, 1, "wear sack")
	if out := bob.take(); !strings.Contains(out, "You don't have that.") {
		t.Fatalf("wear from floor: %q", out)
	}
	send(w, 1, "get sack")
	send(w, 1, "wear sack")
	if out := bob.take(); !strings.Contains(out, "You can't wear that.") {
		t.Fatalf("wear non-armor: %q", out)
	}
	send(w, 1, "hold sack")
	if out := bob.take(); !strings.Contains(out, "You hold a small sack.") {
		t.Fatalf("hold: %q", out)
	}
	send(w, 1, "look in sack")
	if out := bob.take(); !strings.Contains(out, "a loaf of bread") {
		t.Fatalf("look in held container: %q", out)
	}
	send(w, 1, "get bread sack")
	send(w, 1, "give bread alice")
	if out := bob.take(); !strings.Contains(out, "You give a loaf of bread to Alice.") {
		t.Fatalf("give: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "Bob gives you a loaf of bread.") {
		t.Fatalf("give not received: %q", out)
	}
	send(w, 2, "put bread sack")
	if out := alice.take(); !strings.Contains(out, "You don't see that here.") {
		t.Fatalf("put into someone else's held sack should fail: %q", out)
	}
	send(w, 2, "give bread guard")
	if out := alice.take(); !strings.Contains(out, "You give a loaf of bread to a city guard.") {
		t.Fatalf("give to mob: %q", out)
	}
	guard := w.contents(w.players[1].Room).mobs[0]
	if len(guard.Inventory) != 2 {
		t.Fatalf("guard should hold 2 items, has %d", len(guard.Inventory))
	}
}

func TestTargetingAndLookAt(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "look guard")
	if out := bob.take(); !strings.Contains(out, "Tall and bored.") || !strings.Contains(out, "<wielded>") {
		t.Fatalf("look at mob: %q", out)
	}
	send(w, 1, "look cap")
	if out := bob.take(); !strings.Contains(out, "nothing special about a leather cap") {
		t.Fatalf("look at item without look text: %q", out)
	}
	send(w, 1, "look 2.guard")
	if out := bob.take(); !strings.Contains(out, "You don't see that here.") {
		t.Fatalf("2.guard should not exist: %q", out)
	}
	send(w, 1, "n")
	bob.take()
	// The first dog may have wandered off during login; make sure at
	// least two are here (Bob is admin, so load works).
	for _, a := range w.areas {
		w.resetArea(a)
	}
	send(w, 1, "load mob 21")
	bob.take()
	send(w, 1, "look 2.dog")
	if out := bob.take(); !strings.Contains(out, "nothing special about a stray dog") {
		t.Fatalf("2.dog: %q", out)
	}
	send(w, 1, "look")
	if out := bob.take(); strings.Count(out, "A stray dog sniffs about.") < 2 {
		t.Fatalf("expected at least two dogs listed: %q", out)
	}
	login(t, w, 2, "Alice")
	send(w, 1, "s")
	bob.take()
	send(w, 1, "look alice")
	if out := bob.take(); !strings.Contains(out, "nothing special about Alice") {
		t.Fatalf("look at player: %q", out)
	}
}

func TestMobsWanderWithinArea(t *testing.T) {
	w := testWorld(t)
	w.rng = rand.New(rand.NewPCG(1, 2))
	bob := login(t, w, 1, "Bob")
	send(w, 1, "n")
	bob.take()
	moved := false
	for i := 0; i < 400 && !moved; i++ {
		w.Tick()
		if out := bob.take(); strings.Contains(out, "A stray dog leaves") || strings.Contains(out, "A stray dog arrives") {
			moved = true
		}
	}
	if !moved {
		t.Fatal("dog never wandered")
	}
	guardRoom := w.content.Rooms.Rooms[1]
	if len(w.contents(guardRoom).mobs) == 0 || w.contents(guardRoom).mobs[0].Proto.Vnum != 20 {
		t.Fatal("sentinel guard moved")
	}
	// Room 3 is in area b; a stay-in-area dog must never be there.
	for i := 0; i < 400; i++ {
		w.Tick()
		if len(w.contents(w.content.Rooms.Rooms[3]).mobs) != 0 {
			t.Fatal("dog left its area")
		}
	}
}

func TestInventoryPersists(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w1, st := testWorldWithStore(t, playerDir)
	bob := login(t, w1, 1, "Bob")
	send(w1, 1, "get sack")
	send(w1, 1, "get cap")
	send(w1, 1, "wear cap")
	send(w1, 1, "quit")
	bob.take()
	rec, err := st.Load("Bob")
	if err != nil || len(rec.Inventory) != 1 || rec.Inventory[0].Vnum != 12 || len(rec.Inventory[0].Contents) != 1 || rec.Equipment["head"].Vnum != 11 {
		t.Fatalf("saved items wrong: %+v err=%v", rec, err)
	}

	w2, _ := testWorldWithStore(t, playerDir)
	c := signin(t, w2, 1, "Bob", "secret5")
	send(w2, 1, "i")
	if out := c.take(); !strings.Contains(out, "a small sack") {
		t.Fatalf("inventory not restored: %q", out)
	}
	send(w2, 1, "eq")
	if out := c.take(); !strings.Contains(out, "a leather cap") {
		t.Fatalf("equipment not restored: %q", out)
	}
	send(w2, 1, "look in sack")
	if out := c.take(); !strings.Contains(out, "a loaf of bread") {
		t.Fatalf("container contents not restored: %q", out)
	}
}
