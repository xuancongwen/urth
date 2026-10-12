package world

import (
	"strings"
	"testing"

	"urth/internal/item"
)

// carry gives the player n distinct items, so inventory lists n lines.
func carry(p *Player, n int) {
	for i := range n {
		name := "trinket" + itoa(i)
		p.Inventory = append(p.Inventory, item.New(&item.Proto{Vnum: 9500 + i, Name: "a " + name, Keywords: []string{name}, Type: item.Other}))
	}
}

func TestPagerHoldsLongRepliesAPageAtATime(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	carry(w.players[1], 40) // 41 lines with the heading
	send(w, 1, "inventory")
	o := bob.take()
	if !strings.Contains(o, "You are carrying") || !strings.Contains(o, "trinket22") || strings.Contains(o, "trinket23") {
		t.Fatalf("first page should end at trinket22: %q", o)
	}
	if !strings.Contains(o, "17 more lines") {
		t.Fatalf("no more line: %q", o)
	}
	send(w, 1, "")
	o = bob.take()
	if !strings.Contains(o, "trinket23") || !strings.Contains(o, "trinket39") || strings.Contains(o, "more lines") {
		t.Fatalf("second page should be the rest: %q", o)
	}
	send(w, 1, "")
	if o := bob.take(); strings.Contains(o, "trinket") {
		t.Fatalf("Enter after the last page repeated it: %q", o)
	}

	// Any other command drops the rest and runs.
	send(w, 1, "inventory")
	bob.take()
	send(w, 1, "score")
	o = bob.take()
	if strings.Contains(o, "trinket") || !strings.Contains(o, "Bob") {
		t.Fatalf("a command should drop the held page and run: %q", o)
	}
	send(w, 1, "")
	if o := bob.take(); strings.Contains(o, "trinket") {
		t.Fatalf("held page survived another command: %q", o)
	}

	// A short reply is never held.
	send(w, 1, "equipment")
	if o := bob.take(); strings.Contains(o, "more lines") {
		t.Fatalf("short reply paged: %q", o)
	}
}

func TestScrollSetting(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	carry(w.players[1], 40)

	send(w, 1, "scroll 30")
	bob.take()
	send(w, 1, "inventory")
	if o := bob.take(); !strings.Contains(o, "trinket28") || strings.Contains(o, "trinket29") || !strings.Contains(o, "11 more lines") {
		t.Fatalf("a 30-line page: %q", o)
	}
	send(w, 1, "scroll 3")
	if o := bob.take(); !strings.Contains(o, "Syntax") || w.players[1].rec.Scroll != 30 {
		t.Fatalf("too short a page should be refused: %q", o)
	}

	send(w, 1, "scroll off")
	bob.take()
	send(w, 1, "inventory")
	if o := bob.take(); !strings.Contains(o, "trinket39") || strings.Contains(o, "more lines") {
		t.Fatalf("scroll off should show everything: %q", o)
	}
	send(w, 1, "settings")
	if o := bob.take(); !strings.Contains(o, "scroll    off") {
		t.Fatalf("settings should show scroll off: %q", o)
	}
	send(w, 1, "settings scroll")
	bob.take()
	if got := w.players[1].pageLines(); got != defaultPageLines {
		t.Fatalf("settings scroll should turn it back on at the default: %d", got)
	}
}

func TestPagerLeavesTheTapClientAlone(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	carry(w.players[1], 40)
	send(w, 1, "client state")
	bob.take()
	send(w, 1, "inventory")
	if o := bob.take(); !strings.Contains(o, "trinket39") || strings.Contains(o, "more lines") {
		t.Fatalf("the tap client should get the whole list: %q", o)
	}
}
