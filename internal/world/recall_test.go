package world

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRecall(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w, st := testWorldWithStore(t, playerDir)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]

	// The default is the respawn room, which is the start room here.
	send(w, 1, "recall show")
	if o := bob.take(); !strings.Contains(o, "Hub, where everyone starts") {
		t.Fatalf("show default: %q", o)
	}
	send(w, 1, "recall")
	if o := bob.take(); !strings.Contains(o, "already there") {
		t.Fatalf("recall at home: %q", o)
	}

	// Set it two areas away, walk off, and come back in one step.
	p.Room = w.content.Rooms.Rooms[31]
	send(w, 1, "recall set")
	if o := bob.take(); !strings.Contains(o, "Four Out") || p.rec.Recall != 31 {
		t.Fatalf("set: %q", o)
	}
	p.Room = w.content.Rooms.Rooms[1]
	send(w, 1, "recall")
	if o := bob.take(); !strings.Contains(o, "think of Four Out") || p.Room.Vnum != 31 {
		t.Fatalf("recall: %q room %d", o, p.Room.Vnum)
	}
	if rec, err := st.Load("Bob"); err != nil || rec.Recall != 31 {
		t.Fatalf("not saved: %+v %v", rec, err)
	}

	// Not out of a fight.
	p.Room = w.content.Rooms.Rooms[2]
	dog := w.findCharacter(p.Room, p.Character, "dog")
	if dog == nil {
		t.Fatal("no dog to fight")
	}
	p.Fighting = dog
	send(w, 1, "recall")
	if o := bob.take(); !strings.Contains(o, "middle of a fight") || p.Room.Vnum != 2 {
		t.Fatalf("recall in a fight: %q", o)
	}
	p.Fighting = nil

	// A recall point that no longer exists falls back to the start.
	p.rec.Recall = 9999
	send(w, 1, "recall")
	if o := bob.take(); !strings.Contains(o, "think of Hub") || p.Room.Vnum != 1 {
		t.Fatalf("fallback: %q", o)
	}
	p.rec.Recall = 31
	send(w, 1, "recall clear")
	if o := bob.take(); !strings.Contains(o, "Hub again") || p.rec.Recall != 0 {
		t.Fatalf("clear: %q", o)
	}
}

// Every command is in a help section, so 'help' lists all of them, and
// 'help <command>' explains each.
func TestHelpCoversEveryCommand(t *testing.T) {
	listed := map[string]bool{"help": true}
	for _, sec := range helpSections {
		for _, n := range sec.names {
			listed[n] = true
		}
	}
	for _, c := range commands {
		if !listed[c.name] {
			t.Errorf("command %q is in no help section", c.name)
		}
	}
	w := testWorld(t)
	bob := login(t, w, 1, "Bob") // the first character is an admin
	send(w, 1, "help")
	o := bob.take()
	for _, c := range commands {
		if !strings.Contains(o, c.name) {
			t.Errorf("admin help does not show %q", c.name)
		}
	}
	for _, c := range commands {
		send(w, 1, "help "+c.name)
		if o := bob.take(); strings.Contains(o, "no help written yet") {
			t.Errorf("help %s: %q", c.name, o)
		}
	}
}

func TestRecallTutorial(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	w.cfg.World.StartRoom = 32
	p.Room = w.content.Rooms.Rooms[2]
	send(w, 1, "recall tutorial")
	if o := bob.take(); !strings.Contains(o, "think of Five Out") || p.Room.Vnum != 32 {
		t.Fatalf("recall tutorial: %q room %d", o, p.Room.Vnum)
	}
	// The recall point is untouched.
	if p.rec.Recall != 0 {
		t.Fatalf("recall point changed: %d", p.rec.Recall)
	}
}
