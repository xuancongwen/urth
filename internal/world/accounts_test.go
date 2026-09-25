package world

import (
	"strings"
	"testing"

	"urth/internal/session"
)

// Bob is the first character, so an admin; Alice is a player.

func TestPromoteAndDemote(t *testing.T) {
	w, st := testWorldWithStore(t, "")
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	alice.take()

	send(w, 1, "promote alice")
	if out := bob.take(); !strings.Contains(out, "Alice is now an admin") {
		t.Fatalf("promote: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "made you an admin") {
		t.Fatalf("alice not told: %q", out)
	}
	send(w, 2, "users")
	if out := alice.take(); !strings.Contains(out, "admin") || strings.Contains(out, "Huh?") {
		t.Fatalf("alice cannot use admin commands: %q", out)
	}
	rec, err := st.Load("Alice")
	if err != nil || !rec.Admin {
		t.Fatalf("promotion not saved: %v %+v", err, rec)
	}

	send(w, 2, "demote alice")
	if out := alice.take(); !strings.Contains(out, "cannot demote yourself") {
		t.Fatalf("self-demote allowed: %q", out)
	}
	send(w, 1, "demote alice")
	bob.take()
	send(w, 2, "users")
	if out := alice.take(); !strings.Contains(out, "Huh?") {
		t.Fatalf("alice still admin: %q", out)
	}

	// Offline characters are edited on disk.
	send(w, 2, "quit")
	send(w, 1, "promote alice")
	if out := bob.take(); !strings.Contains(out, "Alice is now an admin") {
		t.Fatalf("offline promote: %q", out)
	}
	if rec, _ := st.Load("Alice"); !rec.Admin {
		t.Fatal("offline promotion not saved")
	}
	send(w, 1, "promote nobody")
	if out := bob.take(); !strings.Contains(out, "no character") {
		t.Fatalf("unknown name: %q", out)
	}
}

func TestPasswdResetsAnotherPassword(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	login(t, w, 2, "Alice")
	send(w, 1, "passwd alice")
	if out := bob.take(); !strings.Contains(out, "Syntax") {
		t.Fatalf("no syntax: %q", out)
	}
	send(w, 1, "passwd alice abc")
	if out := bob.take(); !strings.Contains(out, "5 to 64") {
		t.Fatalf("short password accepted: %q", out)
	}
	w.Events() <- session.Input{ID: 1, Line: "passwd alice fresh12"}
	tickUntil(t, w, bob, "Password for Alice changed")
	send(w, 2, "quit")
	signin(t, w, 3, "Alice", "fresh12")

	// Offline too.
	w.Events() <- session.Input{ID: 1, Line: "passwd alice other34"}
	tickUntil(t, w, bob, "Password for Alice changed")
	send(w, 3, "quit")
	signin(t, w, 4, "Alice", "other34")
}

func TestDenyDropsAndRefusesLogin(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	alice.take()
	send(w, 1, "deny bob")
	if out := bob.take(); !strings.Contains(out, "cannot deny yourself") {
		t.Fatalf("self-deny allowed: %q", out)
	}
	send(w, 1, "deny alice")
	if out := bob.take(); !strings.Contains(out, "Alice is denied") {
		t.Fatalf("deny: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "has been denied") || !alice.closed {
		t.Fatalf("alice not dropped: %q closed=%v", out, alice.closed)
	}

	c := connect(t, w, 3)
	send(w, 3, "Alice")
	c.take()
	w.Events() <- session.Input{ID: 3, Line: "secret5"}
	tickUntil(t, w, c, "has been denied")
	if !c.closed {
		t.Fatal("denied login not closed")
	}

	send(w, 1, "allow alice")
	if out := bob.take(); !strings.Contains(out, "may log in again") {
		t.Fatalf("allow: %q", out)
	}
	signin(t, w, 4, "Alice", "secret5")
}

func TestUsersListsConnections(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	connect(t, w, 2)
	send(w, 2, "Zed")
	send(w, 1, "users")
	out := bob.take()
	for _, want := range []string{"Bob", "admin", "(Zed)", "login", "2 connections"} {
		if !strings.Contains(out, want) {
			t.Fatalf("users missing %q: %q", want, out)
		}
	}
}

func TestAccountCommandsHiddenFromPlayers(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	alice.take()
	for _, cmd := range []string{"promote alice", "demote bob", "passwd bob x", "deny bob", "allow bob", "users"} {
		send(w, 2, cmd)
		if out := alice.take(); !strings.Contains(out, "Huh?") {
			t.Fatalf("%s ran for a player: %q", cmd, out)
		}
	}
}
