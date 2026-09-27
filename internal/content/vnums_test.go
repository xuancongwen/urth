package content

import (
	"testing"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/room"
)

func TestBlocks(t *testing.T) {
	w := &World{Rooms: &room.World{Rooms: map[int]*room.Room{}}, Items: map[int]*item.Proto{}, Mobs: map[int]*mob.Proto{}}
	for _, v := range []int{800, 801, 803} {
		w.Rooms.Rooms[v] = &room.Room{Vnum: v, Area: "d"}
	}
	w.Rooms.Rooms[1] = &room.Room{Vnum: 1, Area: "start"}
	w.Mobs[830] = &mob.Proto{Vnum: 830, Area: "d"}
	// Another area's item inside d's block still takes the number.
	w.Items[801] = &item.Proto{Vnum: 801, Area: "other"}
	b := Blocks(w, "d")
	if r := b["rooms"]; r.First != 800 || r.Last != 899 || r.Used != 3 || r.Free != 97 || r.Next != 804 {
		t.Fatalf("rooms: %+v", r)
	}
	if m := b["mobs"]; m.Used != 1 || m.Free != 99 || m.Next != 831 {
		t.Fatalf("mobs: %+v", m)
	}
	// No items yet: the rooms' block, skipping what is taken.
	if i := b["items"]; i.First != 800 || i.Used != 0 || i.Free != 99 || i.Next != 800 {
		t.Fatalf("items: %+v", i)
	}
	// Vnum 0 is not a vnum.
	if s := Blocks(w, "start")["rooms"]; s.First != 1 || s.Last != 99 || s.Free != 98 || s.Next != 2 {
		t.Fatalf("start: %+v", s)
	}
	if e := Blocks(w, "empty")["rooms"]; e.Next != 0 {
		t.Fatalf("empty area: %+v", e)
	}
}
