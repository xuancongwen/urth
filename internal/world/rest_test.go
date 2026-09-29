package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
)

// Under the live rules, resting heals twice as fast as standing and
// sleeping three times.
func TestRestAndSleepRegen(t *testing.T) {
	w := shippedWorld(t)
	c := newCharacter("a tester", []string{"tester"})
	c.Level = 10
	w.recalc(c)
	delta := func(pos string) int {
		c.Position = pos
		c.Health = 1
		return w.onTick(c).HealthDelta
	}
	stand, rest, sleep := delta(posStanding), delta(posResting), delta(posSleeping)
	if stand <= 0 || rest != 2*stand && rest != 2*stand+1 && rest != 2*stand-1 {
		t.Fatalf("stand %d rest %d", stand, rest)
	}
	if sleep < 3*stand-1 || sleep > 3*stand+1 {
		t.Fatalf("stand %d sleep %d", stand, sleep)
	}
}

func TestPositions(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	w.players[1].Admin = false // admins skip the position checks
	p := w.players[1]

	send(w, 1, "rest")
	if o := bob.take(); !strings.Contains(o, "You sit down and rest") || p.Position != posResting {
		t.Fatalf("rest: %q", o)
	}
	if o := alice.take(); !strings.Contains(o, "Bob sits down and rests") {
		t.Fatalf("room saw: %q", o)
	}
	send(w, 2, "look")
	if o := alice.take(); !strings.Contains(o, "Bob is resting here.") {
		t.Fatalf("listing: %q", o)
	}
	send(w, 1, "north")
	if o := bob.take(); !strings.Contains(o, "too relaxed") || p.Room.Vnum != 1 {
		t.Fatalf("walk while resting: %q", o)
	}
	send(w, 1, "say hello")
	if o := bob.take(); !strings.Contains(o, "You say") {
		t.Fatalf("say while resting: %q", o)
	}

	send(w, 1, "sleep")
	bob.take()
	send(w, 1, "say hello")
	if o := bob.take(); !strings.Contains(o, "In your dreams") {
		t.Fatalf("say while asleep: %q", o)
	}
	send(w, 1, "score")
	if o := bob.take(); strings.Contains(o, "In your dreams") {
		t.Fatalf("score while asleep: %q", o)
	}
	send(w, 2, "wake bob")
	if o := alice.take(); !strings.Contains(o, "You shake Bob awake") || p.Position != posStanding {
		t.Fatalf("wake other: %q", o)
	}
	bob.take()

	// A fight brings a sleeper to its feet.
	send(w, 1, "sleep")
	if o := bob.take(); p.Position != posSleeping {
		t.Fatalf("sleep again: %q", o)
	}
	guard := w.findCharacter(p.Room, p.Character, "guard")
	w.startFight(guard, p.Character)
	w.flush()
	if o := bob.take(); p.Position != posStanding || !strings.Contains(o, "You wake up") {
		t.Fatalf("fight did not wake the sleeper: pos %q out %q", p.Position, o)
	}
	w.stopFighting(p.Character, true)
	send(w, 1, "rest")
	bob.take()
	send(w, 1, "stand")
	if o := bob.take(); !strings.Contains(o, "You stand up") || p.Position != posStanding {
		t.Fatalf("stand: %q", o)
	}
}

func TestTellAndReply(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	w.players[2].Room = w.content.Rooms.Rooms[32] // far away

	send(w, 1, "tell ali meet me at the hub")
	if o := bob.take(); !strings.Contains(o, "You tell Alice 'meet me at the hub'") {
		t.Fatalf("sender: %q", o)
	}
	if o := alice.take(); !strings.Contains(o, "Bob tells you 'meet me at the hub'") {
		t.Fatalf("target: %q", o)
	}
	send(w, 2, "reply on my way")
	if o := bob.take(); !strings.Contains(o, "Alice tells you 'on my way'") {
		t.Fatalf("reply: %q", o)
	}
	send(w, 1, "tell nobody hi")
	if o := bob.take(); !strings.Contains(o, "Nobody by that name") {
		t.Fatalf("unknown: %q", o)
	}
	send(w, 2, "quit")
	alice.take()
	send(w, 1, "reply still there?")
	if o := bob.take(); !strings.Contains(o, "not playing any more") {
		t.Fatalf("reply gone: %q", o)
	}
}

func TestLocks(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	gate := w.content.Rooms.Rooms[3].Doors["west"]
	gate.Closed, gate.Locked, gate.Key = true, true, 13 // the loaf is the key
	p.Room = w.content.Rooms.Rooms[2]

	send(w, 1, "open east")
	if o := bob.take(); !strings.Contains(o, "It's locked") {
		t.Fatalf("open locked: %q", o)
	}
	send(w, 1, "look east")
	if o := bob.take(); !strings.Contains(o, "closed and locked") {
		t.Fatalf("look: %q", o)
	}
	send(w, 1, "unlock gate")
	if o := bob.take(); !strings.Contains(o, "You lack the key") {
		t.Fatalf("no key: %q", o)
	}
	giveItem(w, p.Character, 13)
	send(w, 1, "unlock gate")
	if o := bob.take(); !strings.Contains(o, "You unlock the iron gate") || w.doorLocked(p.Room, "east") {
		t.Fatalf("unlock: %q", o)
	}
	// Both sides turn together.
	if w.doorLocked(w.content.Rooms.Rooms[3], "west") {
		t.Fatal("far side still locked")
	}
	send(w, 1, "lock east")
	if o := bob.take(); !strings.Contains(o, "You lock the iron gate") || !w.doorLocked(p.Room, "east") {
		t.Fatalf("lock: %q", o)
	}

	// Pick, ignoring the rules' odds: a pickproof lock never gives.
	p.Inventory = nil
	gate.Pickproof = true
	for i := 0; i < 20; i++ {
		w.roundCount++
		send(w, 1, "pick east")
		bob.take()
	}
	if !w.doorLocked(p.Room, "east") {
		t.Fatal("picked a pickproof lock")
	}
	send(w, 1, "pick east")
	send(w, 1, "pick east")
	if o := bob.take(); !strings.Contains(o, "still busy") {
		t.Fatalf("two picks in a round: %q", o)
	}
	gate.Pickproof = false
	for i := 0; i < 200 && w.doorLocked(p.Room, "east"); i++ {
		w.roundCount++
		send(w, 1, "pick east")
	}
	if w.doorLocked(p.Room, "east") || !strings.Contains(bob.take(), "You pick the lock on the iron gate") {
		t.Fatal("never picked the lock")
	}

	// A reset puts the lock back.
	w.resetDoors("b")
	w.resetDoors("a")
	if !w.doorLocked(p.Room, "east") {
		t.Fatal("reset did not relock")
	}
}

func TestAutoLootGoldSac(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.Room = w.content.Rooms.Rooms[2]
	kill := func() {
		dog := w.findCharacter(p.Room, p.Character, "dog")
		if dog == nil {
			m := w.newMob(w.content.Mobs[21])
			w.placeMob(m, p.Room)
			dog = m.Character
		}
		dog.Silver = 7
		giveItem(w, dog, 13)
		w.die(dog, p.Character)
		bob.take()
	}

	send(w, 1, "autogold")
	if o := bob.take(); !strings.Contains(o, "take the coins") || !p.rec.AutoGold {
		t.Fatalf("autogold: %q", o)
	}
	kill()
	if p.Silver < 7 || len(p.Inventory) != 0 {
		t.Fatalf("autogold took: silver %d inventory %v", p.Silver, p.Inventory)
	}

	send(w, 1, "autoloot")
	send(w, 1, "autosac")
	bob.take()
	before := len(w.contents(p.Room).items)
	kill()
	if len(p.Inventory) != 1 || len(w.contents(p.Room).items) != before {
		t.Fatalf("autoloot+autosac: inventory %v, room items %d -> %d", p.Inventory, before, len(w.contents(p.Room).items))
	}
}

func TestAffects(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "affects")
	if o := bob.take(); !strings.Contains(o, "not affected by anything") {
		t.Fatalf("none: %q", o)
	}
	p := w.players[1]
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "dot", Params: map[string]any{"damage": 2}}, Rounds: 30})
	send(w, 1, "affects")
	if o := bob.take(); !strings.Contains(o, "You are affected by") || !strings.Contains(o, "30 seconds") {
		t.Fatalf("timed: %q", o)
	}
}
