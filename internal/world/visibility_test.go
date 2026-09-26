package world

import (
	"strings"
	"testing"

	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/session"
)

func addEffect(w *World, id int, kind string) {
	p := w.players[session.ID(id)]
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: kind}})
}

func TestWhoObeysInvisibleAndHidden(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	carol := login(t, w, 3, "Carol")
	bob.take()
	alice.take()
	carol.take()
	// The first character in a fresh world is the admin; make Bob plain.
	w.players[1].Admin = false

	addEffect(w, 2, effectInvisible)
	addEffect(w, 3, effectHidden)

	// Bob sees neither, and the count is of what he sees.
	send(w, 1, "who")
	out := bob.take()
	if strings.Contains(out, "Alice") || strings.Contains(out, "Carol") || !strings.Contains(out, "1 player online") {
		t.Fatalf("plain viewer saw unseen players: %q", out)
	}

	// Alice sees herself, marked, but not the hidden Carol.
	send(w, 2, "who")
	out = alice.take()
	if !strings.Contains(out, "[  1] Alice (Invis)\n") || strings.Contains(out, "Carol") || !strings.Contains(out, "2 players online") {
		t.Fatalf("invisible viewer's who wrong: %q", out)
	}

	// Detection sees through the matching one only.
	addEffect(w, 1, effectDetectInvisible)
	send(w, 1, "who")
	out = bob.take()
	if !strings.Contains(out, "[  1] Alice (Invis)\n") || strings.Contains(out, "Carol") {
		t.Fatalf("detect invisible wrong: %q", out)
	}
	addEffect(w, 1, effectDetectHidden)
	send(w, 1, "who")
	out = bob.take()
	if !strings.Contains(out, "[  1] Carol (Hidden)\n") || !strings.Contains(out, "3 players online") {
		t.Fatalf("detect hidden wrong: %q", out)
	}

	// Admins see everyone, and each row carries the level.
	w.players[3].Admin = true
	w.players[2].Level = 12
	send(w, 3, "who")
	out = carol.take()
	if !strings.Contains(out, "[ 12] Alice (Invis)\n") || !strings.Contains(out, "[  1] Bob\n") || !strings.Contains(out, "[  1] Carol (Hidden)\n") {
		t.Fatalf("admin who wrong: %q", out)
	}
}

func TestCanSeeCharThroughWornGear(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	login(t, w, 2, "Alice")
	bob, alice := w.players[1].Character, w.players[2].Character
	w.players[1].Admin = false

	alice.Effects = append(alice.Effects, effect.Active{Spec: effect.Spec{Kind: effectInvisible}})
	if w.canSeeChar(bob, alice) {
		t.Fatal("saw an invisible player without detection")
	}
	// A worn item carrying the detect effect counts.
	it := item.New(&item.Proto{Vnum: 90, Name: "a seeing eye", Keywords: []string{"eye"}, Type: item.Armor, Slot: "light"})
	it.Effects = append(it.Effects, effect.Active{Spec: effect.Spec{Kind: effectDetectInvisible}})
	bob.Equipment["light"] = it
	if !w.canSeeChar(bob, alice) {
		t.Fatal("worn detect invisible did not work")
	}
	if !w.canSeeChar(alice, alice) {
		t.Fatal("cannot see self")
	}
}

func TestLookAndScanObeyVisibility(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	w.players[1].Admin = false
	// Alice stands north with the dogs; Bob is in the hub with the guard.
	send(w, 2, "n")
	bob.take()
	alice.take()

	// Seen: the guard and Alice show up as usual.
	send(w, 1, "look")
	if out := bob.take(); !strings.Contains(out, "A city guard stands here.") {
		t.Fatalf("guard missing from look: %q", out)
	}
	send(w, 1, "scan")
	if out := bob.take(); !strings.Contains(out, "    Alice\n") {
		t.Fatalf("alice missing from scan: %q", out)
	}

	guard := w.contents(w.players[1].Room).mobs[0]
	guard.Effects = append(guard.Effects, effect.Active{Spec: effect.Spec{Kind: effectHidden}})
	addEffect(w, 2, effectInvisible)

	// Unseen: gone from look, look <name>, and scan.
	send(w, 1, "look")
	if out := bob.take(); strings.Contains(out, "city guard") {
		t.Fatalf("hidden guard in look: %q", out)
	}
	send(w, 1, "look guard")
	if out := bob.take(); !strings.Contains(out, "You don't see that here.") {
		t.Fatalf("looked at a hidden guard: %q", out)
	}
	send(w, 1, "scan")
	if out := bob.take(); strings.Contains(out, "Alice") {
		t.Fatalf("invisible alice in scan: %q", out)
	}
	send(w, 2, "look")
	if out := alice.take(); !strings.Contains(out, "A stray dog sniffs about.") {
		t.Fatalf("alice's own look broken: %q", out)
	}
	send(w, 2, "scan")
	if out := alice.take(); strings.Contains(out, "city guard") || !strings.Contains(out, "    Bob\n") {
		t.Fatalf("scan from north wrong: %q", out)
	}

	// Detection restores them.
	addEffect(w, 1, effectDetectHidden)
	addEffect(w, 1, effectDetectInvisible)
	send(w, 1, "look")
	if out := bob.take(); !strings.Contains(out, "A city guard stands here.") {
		t.Fatalf("detect hidden did not restore look: %q", out)
	}
	send(w, 1, "look guard")
	if out := bob.take(); !strings.Contains(out, "Tall and bored.") {
		t.Fatalf("detect hidden did not restore look at: %q", out)
	}
	send(w, 1, "scan")
	if out := bob.take(); !strings.Contains(out, "    Alice\n") {
		t.Fatalf("detect invisible did not restore scan: %q", out)
	}
}
