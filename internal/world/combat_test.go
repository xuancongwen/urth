package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test rules: 10 + 2*level health, 1 damage unarmed, weapon damage with a
// weapon, +1 health per round out of combat, 60 xp per kill, 100 xp per
// level.

func TestKillMobAwardsXPAndLevels(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "n") // dog is here (12 health at level 1)
	bob.take()
	dog := w.contents(w.players[1].Room).mobs[0]
	send(w, 1, "kill dog")
	out := bob.take()
	if !strings.Contains(out, "You hit a stray dog with your fists. [1]") {
		t.Fatalf("first swing: %q", out)
	}
	for i := 0; i < 10; i++ { // one round: the dog swings back
		w.Tick()
	}
	if out := bob.take(); !strings.Contains(out, "A stray dog hits you with a stray dog's fists. [1]") {
		t.Fatalf("mob did not retaliate: %q", out)
	}
	send(w, 1, "n")
	if out := bob.take(); !strings.Contains(out, "You are fighting") {
		t.Fatalf("moving while fighting allowed: %q", out)
	}
	// Bob has 12 hp too; both deal 1 per round. Bob struck first, so the
	// dog dies on the 12th swing, before Bob does.
	dead := false
	for i := 0; i < 200 && !dead; i++ {
		w.Tick()
		if o := bob.take(); strings.Contains(o, "A stray dog is DEAD!!") {
			dead = true
			out = o
		}
	}
	if !dead {
		t.Fatal("dog never died")
	}
	if !strings.Contains(out, "You receive 60 experience points.") {
		t.Fatalf("xp not awarded: %q", out)
	}
	if w.players[1].Fighting != nil {
		t.Fatal("still fighting after kill")
	}
	for _, m := range w.contents(w.players[1].Room).mobs {
		if m == dog {
			t.Fatal("dead dog still in room")
		}
	}
	if dog.Room != nil {
		t.Fatal("dead dog still placed")
	}
	// A second dog kill crosses 100 xp: level 2 with the stat delta.
	for _, a := range w.areas {
		w.resetArea(a)
	}
	w.players[1].Health = w.players[1].HealthMax
	send(w, 1, "kill dog")
	leveled := false
	for i := 0; i < 200 && !leveled; i++ {
		w.Tick()
		if o := bob.take(); strings.Contains(o, "Level up.") {
			leveled = true
			out = o
		}
	}
	if !leveled {
		t.Fatal("never leveled")
	}
	p := w.players[1]
	if p.Level != 2 || p.Stats["might"] != 1 || p.HealthMax != 14 || p.Health != 14 {
		t.Fatalf("level-up state wrong: level=%d stats=%v hp=%d/%d", p.Level, p.Stats, p.Health, p.HealthMax)
	}
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "level 2") || !strings.Contains(o, "might 1") {
		t.Fatalf("score: %q", o)
	}
}

func TestWeaponDamageAndDeathDrops(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "kill guard") // guard wields the 4-damage sword and carries bread
	bob.take()
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	out := bob.take()
	if !strings.Contains(out, "A city guard hits you with a rusty sword. [4]") {
		t.Fatalf("weapon damage not used: %q", out)
	}
	// Bob: 12 hp, takes 4/round after his own 1. Guard kills Bob first.
	died := false
	for i := 0; i < 200 && !died; i++ {
		w.Tick()
		if o := bob.take(); strings.Contains(o, "You have been KILLED!!") {
			died = true
			out = o
		}
	}
	if !died {
		t.Fatal("bob never died")
	}
	p := w.players[1]
	if p.Health != p.HealthMax || p.Room.Vnum != 1 || p.Fighting != nil {
		t.Fatalf("respawn state wrong: hp=%d room=%d fighting=%v", p.Health, p.Room.Vnum, p.Fighting)
	}
	guard := w.contents(p.Room).mobs[0]
	if guard.Fighting != nil {
		t.Fatal("guard still fighting a dead player")
	}
	// Now make the guard die: give bob the sword equivalent by editing
	// health directly, then confirm drops.
	guard.Health = 1
	send(w, 1, "kill guard")
	out = bob.take()
	if !strings.Contains(out, "A city guard is DEAD!!") || !strings.Contains(out, "belongings fall") {
		t.Fatalf("guard death: %q", out)
	}
	items := w.contents(p.Room).items
	if !hasVnum(items, 10) || !hasVnum(items, 13) {
		t.Fatalf("guard's sword and bread not dropped: %v", items)
	}
}

func TestFleeEndsFight(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "flee")
	if o := bob.take(); !strings.Contains(o, "aren't fighting") {
		t.Fatalf("flee when idle: %q", o)
	}
	send(w, 1, "kill guard")
	bob.take()
	send(w, 1, "flee")
	o := bob.take()
	if !strings.Contains(o, "You flee north!") {
		t.Fatalf("flee: %q", o)
	}
	p := w.players[1]
	if p.Fighting != nil || p.Room.Vnum != 2 {
		t.Fatalf("flee state: fighting=%v room=%d", p.Fighting, p.Room.Vnum)
	}
	guard := w.contents(w.content.Rooms.Rooms[1]).mobs[0]
	if guard.Fighting != nil {
		t.Fatal("guard still fighting after target fled")
	}
}

func TestRegenOutOfCombat(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	p.Health = 5
	for i := 0; i < 30; i++ { // 3 rounds
		w.Tick()
	}
	if p.Health != 8 {
		t.Fatalf("expected 8 health after 3 rounds of regen, got %d", p.Health)
	}
}

func TestEditingRulesChangesOutcomeWithoutRestart(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "[1]") {
		t.Fatalf("baseline damage: %q", o)
	}
	send(w, 1, "flee")
	bob.take()

	edited := strings.Replace(testRules, "damage: w && w.weapon ? w.weapon.damage : 1", "damage: 7", 1)
	path := filepath.Join(w.scriptDir, "rules.js")
	future := time.Now().Add(3 * time.Second)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, future, future)
	// The reload check runs on the round; wait one round.
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	bob.take()
	send(w, 1, "s")
	bob.take()
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "You hit a city guard with your fists. [7]") {
		t.Fatalf("edited damage not live: %q", o)
	}
	send(w, 1, "flee")
	bob.take()

	// A broken edit keeps the old rules and warns the admin once.
	later := future.Add(3 * time.Second)
	os.WriteFile(path, []byte("function resolveAttack( {"), 0o644)
	os.Chtimes(path, later, later)
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); !strings.Contains(o, "Script reload failed") {
		t.Fatalf("admin not warned: %q", o)
	}
	for i := 0; i < 10; i++ {
		w.Tick()
	}
	if o := bob.take(); strings.Contains(o, "Script reload failed") {
		t.Fatalf("warning repeated every round: %q", o)
	}
	for _, a := range w.areas {
		w.resetArea(a) // make sure a guard is there to hit
	}
	send(w, 1, "s")
	bob.take()
	send(w, 1, "kill guard")
	if o := bob.take(); !strings.Contains(o, "[7]") {
		t.Fatalf("old rules not kept: %q", o)
	}
}

func TestSimulateCommand(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "simulate 21 20 50 7") // dog vs guard with sword
	o := bob.take()
	if !strings.Contains(o, "Simulated 50 fights") || !strings.Contains(o, "B wins 100.0%") {
		t.Fatalf("simulate: %q", o)
	}
	send(w, 1, "simulate me 21 20 1")
	o = bob.take()
	if !strings.Contains(o, "you vs a stray dog") || !strings.Contains(o, "A wins 100.0%") {
		t.Fatalf("simulate me: %q", o)
	}
	// Seeded runs are identical.
	send(w, 1, "simulate 21 21 20 42")
	a := bob.take()
	send(w, 1, "simulate 21 21 20 42")
	b := bob.take()
	if a != b {
		t.Fatalf("seeded simulations differ:\n%s\n%s", a, b)
	}
	// The live world was not touched.
	if w.countMobs(w.content.Mobs[21]) != 1 || w.countMobs(w.content.Mobs[20]) != 1 {
		t.Fatal("simulation leaked mobs into the world")
	}
	if w.players[1].Health != w.players[1].HealthMax {
		t.Fatal("simulation damaged the real player")
	}
}

func TestSheetPersists(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w1, _ := testWorldWithStore(t, playerDir)
	bob := login(t, w1, 1, "Bob")
	p := w1.players[1]
	p.Level, p.Experience, p.Health = 3, 250, 7
	p.Stats["might"] = 2
	send(w1, 1, "quit")
	bob.take()
	w2, _ := testWorldWithStore(t, playerDir)
	signin(t, w2, 1, "Bob", "secret5")
	q := w2.players[1]
	if q.Level != 3 || q.Experience != 250 || q.Health != 7 || q.HealthMax != 16 || q.Stats["might"] != 2 {
		t.Fatalf("sheet not restored: level=%d xp=%d hp=%d/%d stats=%v", q.Level, q.Experience, q.Health, q.HealthMax, q.Stats)
	}
}
