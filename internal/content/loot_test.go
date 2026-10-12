package content

import (
	"strings"
	"testing"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/room"
)

func TestValidateLoot(t *testing.T) {
	for _, c := range []struct {
		loot []item.LootEntry
		want string
	}{
		{[]item.LootEntry{{Item: 1, Chance: 0.05}, {Item: 1, Chance: 1}}, ""},
		{[]item.LootEntry{{Item: 2, Chance: 0.5}}, "loot item 2 does not exist"},
		{[]item.LootEntry{{Item: 1, Chance: 0}}, "chance 0"},
		{[]item.LootEntry{{Item: 1, Chance: 1.5}}, "chance 1.5"},
	} {
		for _, onArea := range []bool{false, true} {
			w := &World{Items: map[int]*item.Proto{1: {Vnum: 1}}, Mobs: map[int]*mob.Proto{}, Rooms: &room.World{Areas: map[string]room.Area{}}}
			if onArea {
				w.Rooms.Areas["a"] = room.Area{Loot: c.loot}
			} else {
				w.Mobs[5] = &mob.Proto{Vnum: 5, Name: "a rat", Loot: c.loot}
			}
			err := w.validate()
			if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
				t.Errorf("loot %+v on area %v: %v, want %q", c.loot, onArea, err, c.want)
			}
		}
	}
}
