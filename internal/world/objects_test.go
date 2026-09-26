package world

import (
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
)

// The test world (see world_test.go) resets area "a" at boot: a stationary
// guard wielding a rusty sword and carrying bread in room 1, up to two
// wandering stray dogs in room 2, and in room 1 a sack containing bread, a leather
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
		t.Fatal("reset duplicated the guard")
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
		t.Fatal("unflagged guard moved")
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

func TestPairedSlotsAndNewPositions(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	ring := &item.Proto{Vnum: 90, Name: "a plain ring", Keywords: []string{"plain", "ring"}, Type: item.Armor, Slot: "finger"}
	ring.ResolveStated()
	belt := &item.Proto{Vnum: 91, Name: "a wide belt", Keywords: []string{"wide", "belt"}, Type: item.Armor, Slot: "belt"}
	belt.ResolveStated()
	for i := 0; i < 3; i++ {
		p.Inventory = append(p.Inventory, item.New(ring))
	}
	p.Inventory = append(p.Inventory, item.New(belt))
	send(w, 1, "wear ring")
	bob.take()
	send(w, 1, "wear ring")
	bob.take()
	if p.Equipment["finger1"] == nil || p.Equipment["finger2"] == nil {
		t.Fatalf("two rings should fill both fingers: %v", p.Equipment)
	}
	send(w, 1, "wear ring")
	if o := bob.take(); !strings.Contains(o, "You stop using a plain ring.") || !strings.Contains(o, "You wear a plain ring.") {
		t.Fatalf("third ring should swap: %q", o)
	}
	send(w, 1, "wear belt")
	bob.take()
	send(w, 1, "equipment")
	o := bob.take()
	for _, want := range []string{"<worn on left finger>", "<worn on right finger>", "<worn as belt>"} {
		if !strings.Contains(o, want) {
			t.Fatalf("equipment missing %q: %q", want, o)
		}
	}
	if !item.ValidItemSlot("wrist") || item.ValidSlot("wrist") || !item.ValidSlot("wrist2") || item.Family("wrist2") != "wrist" || item.Family("head") != "head" {
		t.Fatal("slot families wrong")
	}
	send(w, 1, "remove ring")
	bob.take()
	if _, ok := p.Equipment["finger1"]; ok {
		t.Fatal("remove did not free the finger")
	}
}

func TestLightsDarkRoomsAndBurn(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	torch := &item.Proto{Vnum: 92, Name: "a torch", Keywords: []string{"torch"}, Type: item.Light, Burn: 2}
	torch.ResolveStated()
	lantern := &item.Proto{Vnum: 93, Name: "a lantern", Keywords: []string{"lantern"}, Type: item.Light}
	lantern.ResolveStated()
	p.Inventory = append(p.Inventory, item.New(torch), item.New(lantern))
	w.content.Rooms.Rooms[2].Flags = []string{"dark"}
	send(w, 1, "n")
	if o := bob.take(); !strings.Contains(o, "It is pitch black.") || strings.Contains(o, "stray dog") {
		t.Fatalf("dark room: %q", o)
	}
	send(w, 1, "hold torch")
	if o := bob.take(); !strings.Contains(o, "You light a torch.") || !strings.Contains(o, "North") {
		t.Fatalf("lighting shows the room: %q", o)
	}
	nextRound(w)
	nextRound(w)
	if o := bob.take(); !strings.Contains(o, "A torch flickers and goes out.") {
		t.Fatalf("burn out: %q", o)
	}
	send(w, 1, "look")
	if o := bob.take(); !strings.Contains(o, "pitch black") {
		t.Fatalf("dark again after burn out: %q", o)
	}
	send(w, 1, "wear lantern")
	if o := bob.take(); !strings.Contains(o, "You light a lantern.") {
		t.Fatalf("lantern: %q", o)
	}
	for i := 0; i < 3; i++ {
		nextRound(w)
	}
	bob.take()
	send(w, 1, "look")
	if o := bob.take(); strings.Contains(o, "pitch black") {
		t.Fatalf("a lantern never goes out: %q", o)
	}
	send(w, 1, "look lantern")
	if o := bob.take(); !strings.Contains(o, "A light that never goes out.") {
		t.Fatalf("look lantern: %q", o)
	}
	// Another character's light lights the room for everyone.
	send(w, 1, "remove lantern")
	bob.take()
	alice := login(t, w, 2, "Alice")
	w.players[2].Room = w.players[1].Room
	w.players[2].Equipment["light"] = item.New(lantern)
	send(w, 1, "look")
	if o := bob.take(); strings.Contains(o, "pitch black") {
		t.Fatalf("another's light: %q", o)
	}
	_ = alice
}

func TestPromptShowsToNextLevelAndScoreShowsEffectiveStats(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "look")
	if o := bob.take(); !strings.Contains(o, "<12/12hp 5/5m 100tnl> ") {
		t.Fatalf("prompt: %q", o)
	}
	setRules(t, w, testRules+`function derivedStats(c) { var m = c.stats.might || 0; for (var i = 0; i < c.effects.length; i++) if (c.effects[i].kind === "stat") m += c.effects[i].params.amount; return { healthMax: 12, manaMax: 5, speed: 1, stats: { might: m } }; }`)
	p := w.players[1]
	p.Stats["might"] = 3
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "stat", Params: map[string]any{"stat": "might", "amount": 2}}})
	w.recalc(p.Character)
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "Might        3 (5)") {
		t.Fatalf("effective stat: %q", o)
	}
}

// The room listing tells fixtures, portable items, and creatures apart:
// fixtures follow the description uncolored and come first, then items
// in green, then creatures in yellow. The structured data marks fixtures.
func TestLookSeparatesFixturesItemsAndMobs(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	bob.batches = nil
	send(w, 1, "look")
	var msg *output.Message
	for _, b := range bob.batches {
		for i := range b.Messages {
			if b.Messages[i].Type == output.Room {
				msg = &b.Messages[i]
			}
		}
	}
	if msg == nil {
		t.Fatal("no room message")
	}
	text := msg.Text
	altar := strings.Index(text, "A stone altar stands here.\n")
	cap := strings.Index(text, "{G}A leather cap is here.{x}\n")
	guard := strings.Index(text, "{Y}A city guard stands here.{x}\n")
	if altar < 0 || cap < 0 || guard < 0 {
		t.Fatalf("listing lacks the expected colored lines:\n%s", text)
	}
	if strings.Contains(text, "}A stone altar") {
		t.Errorf("fixture should be uncolored:\n%s", text)
	}
	if !(altar < cap && cap < guard) {
		t.Errorf("want fixture, then item, then mob; got:\n%s", text)
	}
	data, ok := msg.Data.(output.RoomData)
	if !ok {
		t.Fatalf("room data is %T", msg.Data)
	}
	fixed := map[string]bool{}
	for _, e := range data.Items {
		fixed[e.Name] = e.Fixed
	}
	if !fixed["a stone altar"] || fixed["a leather cap"] {
		t.Errorf("fixed flags wrong: %v", fixed)
	}
}
