package room

import (
	"strings"
	"testing"
)

func grid() *World {
	// 1 -east- 2 -east- 3, with 4 south of 2 and 5 up from 1.
	w := &World{Rooms: map[int]*Room{}, Areas: map[string]Area{}}
	add := func(vnum int, exits map[string]int) {
		w.Rooms[vnum] = &Room{Vnum: vnum, Name: "r", Area: "a", Exits: exits}
	}
	add(1, map[string]int{"east": 2, "up": 5})
	add(2, map[string]int{"west": 1, "east": 3, "south": 4})
	add(3, map[string]int{"west": 2})
	add(4, map[string]int{"north": 2})
	add(5, map[string]int{"down": 1})
	return w
}

func TestLayoutWalksExits(t *testing.T) {
	w := grid()
	if warns := w.Layout(); len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	want := map[int][3]int{1: {0, 0, 0}, 2: {1, 0, 0}, 3: {2, 0, 0}, 4: {1, 1, 0}, 5: {0, 0, 1}}
	for vnum, pos := range want {
		r := w.Rooms[vnum]
		if !r.Placed || [3]int{r.X, r.Y, r.Z} != pos {
			t.Errorf("room %d at %v placed=%v, want %v", vnum, [3]int{r.X, r.Y, r.Z}, r.Placed, pos)
		}
	}
}

func TestLayoutHonoursAnchorAndReportsCollision(t *testing.T) {
	w := grid()
	w.Rooms[2].Position = &Coord{X: 10, Y: 10, Z: 0}
	// 6 is west of 3 by its own exit, which lands on 2's cell.
	w.Rooms[6] = &Room{Vnum: 6, Name: "clash", Area: "a", Exits: map[string]int{"east": 3}}
	w.Rooms[3].Exits["south"] = 6 // reachable, but placed south of 3 first
	warns := w.Layout()
	if r := w.Rooms[2]; r.X != 10 || r.Y != 10 {
		t.Fatalf("anchor moved to %d,%d", r.X, r.Y)
	}
	if r := w.Rooms[1]; r.X != 9 || r.Y != 10 {
		t.Fatalf("room 1 should be west of the anchor, got %d,%d", r.X, r.Y)
	}
	// Room 6 was placed south of 3 by the walk; its own east exit is never
	// walked backwards, so there is no collision here.
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	// Now force one: 7 is east of 2's south neighbour 4 and also north of
	// 3's south neighbour 6, and those cells differ, so the second route
	// collides with where the first put it.
	w.Rooms[7] = &Room{Vnum: 7, Name: "seven", Area: "a", Exits: map[string]int{}}
	w.Rooms[4].Exits["east"] = 7
	w.Rooms[6].Exits["west"] = 7
	warns = w.Layout()
	// Both routes to 7 land on an occupied cell (4's east is 6's cell and
	// 6's west is 4's cell), so each is reported and 7 is placed apart.
	if len(warns) != 2 || !strings.Contains(warns[0], "already there") {
		t.Fatalf("want two collision warnings, got %v", warns)
	}
	for _, r := range w.Rooms {
		if !r.Placed {
			t.Errorf("room %d left unplaced", r.Vnum)
		}
	}
}

func TestLayoutSeparatesComponents(t *testing.T) {
	w := grid()
	w.Rooms[9] = &Room{Vnum: 9, Name: "island", Area: "a", Exits: map[string]int{}}
	w.Layout()
	if r := w.Rooms[9]; !r.Placed || r.X <= 2 {
		t.Fatalf("island should be placed to the right of the grid, got %d,%d placed=%v", r.X, r.Y, r.Placed)
	}
}
