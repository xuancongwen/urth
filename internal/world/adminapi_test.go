package world

import (
	"errors"
	"strings"
	"testing"
	"time"

	"urth/internal/admin"
)

func TestAdminAPI(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	send(w, 1, "get cap")
	send(w, 2, "quit")
	w.Tick()

	stop, done := make(chan struct{}), make(chan struct{})
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
	api := w.Admin()

	st, err := api.Status()
	if err != nil || st.Playing != 1 || st.Characters != 2 || st.Rooms != 6 {
		t.Fatalf("status: %+v %v", st, err)
	}
	rows, err := api.Characters()
	if err != nil || len(rows) != 2 || rows[0].Name != "Alice" || rows[0].Online || !rows[1].Online || rows[1].RoomName != "Hub" || !rows[1].Admin {
		t.Fatalf("characters: %+v %v", rows, err)
	}
	// Bob is online: the view is live, cap and all, without a save.
	bob, err := api.Character("bob")
	if err != nil || bob.Name != "Bob" || len(bob.Inventory) != 1 || bob.Inventory[0].Name != "a leather cap" || bob.HealthMax == 0 {
		t.Fatalf("bob: %+v %v", bob, err)
	}
	if _, err := api.Character("Nobody"); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	if msg, err := api.Act("", "promote", "alice"); err != nil || !strings.Contains(msg, "Alice is now an admin") {
		t.Fatalf("promote offline: %q %v", msg, err)
	}
	if rows, _ := api.Characters(); !rows[0].Admin {
		t.Fatal("promote not saved")
	}
	if _, err := api.Act("", "kick", "Alice"); !errors.Is(err, admin.ErrRefused) {
		t.Fatalf("kick offline: %v", err)
	}
	if _, err := api.Act("", "delete", "Bob"); !errors.Is(err, admin.ErrRefused) {
		t.Fatalf("delete online: %v", err)
	}
	if _, err := api.SetPassword("", "Alice", "abc"); !errors.Is(err, admin.ErrRefused) {
		t.Fatalf("short password: %v", err)
	}
	if msg, err := api.SetPassword("", "Alice", "newsecret"); err != nil || !strings.Contains(msg, "changed") {
		t.Fatalf("password: %q %v", msg, err)
	}
	if _, err := api.Act("", "kick", "Bob"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		rows, _ := api.Characters()
		if !rows[1].Online {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("kicked player still online")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := api.Act("", "delete", "Bob"); err != nil {
		t.Fatalf("delete offline: %v", err)
	}
	if rows, _ := api.Characters(); len(rows) != 1 {
		t.Fatalf("after delete: %+v", rows)
	}
	// Signing in takes an admin's game password; Alice was promoted.
	if name, err := api.Authenticate("alice", "newsecret"); err != nil || name != "Alice" || !api.IsAdmin("Alice") {
		t.Fatalf("sign in: %q %v", name, err)
	}
	if _, err := api.Authenticate("alice", "secret5"); !errors.Is(err, admin.ErrBadLogin) {
		t.Fatalf("old password: %v", err)
	}
	if _, err := api.Act("Alice", "demote", "alice"); !errors.Is(err, admin.ErrRefused) {
		t.Fatalf("self demote: %v", err)
	}
	_ = alice
}
