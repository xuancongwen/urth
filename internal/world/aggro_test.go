package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/mob"
)

// The guard in the start room, made aggressive to levels 1 to 5.
func aggressiveGuard(t *testing.T, w *World, p *Player) *Mob {
	t.Helper()
	g := w.contents(p.Room).mobs[0]
	g.Proto.Flags = append(g.Proto.Flags, "aggressive")
	g.Proto.Resolved.Aggro = mob.Range{Min: 1, Max: 5}
	return g
}

func TestAggressiveMobAttacksInRange(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	p.Admin = false
	g := aggressiveGuard(t, w, p)
	bob.take()

	// Too strong for it: left alone.
	p.Level = 6
	w.aggression()
	if g.Fighting != nil {
		t.Fatal("attacked a player above its range")
	}
	// Hiding, in range: unseen, so left alone, until it can see the hidden.
	p.Level = 3
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: effectHidden}})
	w.aggression()
	if g.Fighting != nil {
		t.Fatal("attacked a hidden player it cannot see")
	}
	g.Effects = append(g.Effects, effect.Active{Spec: effect.Spec{Kind: effectDetectHidden}})
	w.aggression()
	if g.Fighting != p.Character || p.Fighting != g.Character {
		t.Fatalf("in range and seen: guard on %v, Bob on %v", g.Fighting, p.Fighting)
	}
	w.flush()
	if o := bob.take(); !strings.Contains(o, "attacks you!") {
		t.Fatalf("no warning: %q", o)
	}
}

func TestAggressiveMobSparesAdminsAndSafeRooms(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	g := aggressiveGuard(t, w, p)
	p.Level = 2

	p.Admin = true
	w.aggression()
	if g.Fighting != nil {
		t.Fatal("attacked an admin")
	}
	p.Admin = false
	p.Room.Flags = append(p.Room.Flags, "safe")
	w.aggression()
	if g.Fighting != nil {
		t.Fatal("attacked in a safe room")
	}
}

// The shipped rules put the range near the mob's level, and a stated
// bound wins at its own end.
func TestShippedAggroRange(t *testing.T) {
	w := shippedWorld(t)
	for _, p := range w.content.Mobs {
		if p.Level != 10 || p.Aggro != nil {
			continue
		}
		if p.Resolved.Aggro != (mob.Range{Min: 6, Max: 15}) {
			t.Fatalf("level 10 default: %+v", p.Resolved.Aggro)
		}
		p.Aggro = &mob.Range{Min: 3}
		w.resolveBaselines()
		if p.Resolved.Aggro != (mob.Range{Min: 3, Max: 15}) {
			t.Fatalf("stated min 3: %+v", p.Resolved.Aggro)
		}
		return
	}
	t.Fatal("no level-10 mob")
}
