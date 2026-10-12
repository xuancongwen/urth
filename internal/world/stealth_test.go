package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
)

func give(c *Character, kind string) {
	c.Effects = append(c.Effects, effect.Active{Spec: effect.Spec{Kind: kind, Params: map[string]any{}}, Rounds: 50})
}

// Sneaking, Bob's comings and goings are lost on Alice until she can see
// the hidden.
func TestSneakHidesMovementFromThoseWhoCannotSeeTheHidden(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	p, a := w.players[1], w.players[2]
	p.Admin, a.Admin = false, false
	a.Room = w.content.Rooms.Rooms[2]
	bob.take()
	alice.take()

	send(w, 1, "north")
	if o := alice.take(); !strings.Contains(o, "Bob arrives") {
		t.Fatalf("plain arrival: %q", o)
	}
	send(w, 1, "south")
	alice.take()

	give(p.Character, effectSneak)
	send(w, 1, "north")
	if o := alice.take(); strings.Contains(o, "Bob") {
		t.Fatalf("sneaking arrival was seen: %q", o)
	}
	send(w, 1, "south")
	if o := alice.take(); strings.Contains(o, "Bob") {
		t.Fatalf("sneaking departure was seen: %q", o)
	}

	give(a.Character, effectDetectHidden)
	send(w, 1, "north")
	if o := alice.take(); !strings.Contains(o, "Bob arrives") {
		t.Fatalf("detect hidden should see a sneak: %q", o)
	}
}

// Hiding of one's own ends on a move; hiding from gear does not.
func TestMovingEndsHiding(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	give(p.Character, effectHidden)
	bob.take()
	send(w, 1, "north")
	if o := bob.take(); !strings.Contains(o, "You step out of hiding.") || p.Hidden() {
		t.Fatalf("still hidden after moving: %v %q", p.Hidden(), o)
	}
}

// An aggressive mob passes a sneaking player by, unless it sees the
// hidden; and a hidden mob can be seen by whoever it fights.
func TestSneakPastAggressionAndAmbushers(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	p.Admin = false
	p.Level = 2
	g := aggressiveGuard(t, w, p)
	give(p.Character, effectSneak)
	w.aggression()
	if g.Fighting != nil {
		t.Fatal("attacked a sneaking player")
	}
	give(g.Character, effectDetectHidden)
	w.aggression()
	if g.Fighting != p.Character {
		t.Fatal("a mob that sees the hidden let a sneak by")
	}

	// The guard hides, fighting Bob: Bob still sees it; Alice does not.
	give(g.Character, effectHidden)
	if !w.canSeeChar(p.Character, g.Character) {
		t.Fatal("cannot see a hidden mob fighting you")
	}
	alice := login(t, w, 2, "Alice")
	alice.take()
	if w.canSeeChar(w.players[2].Character, g.Character) {
		t.Fatal("a bystander sees a hidden mob")
	}
}
