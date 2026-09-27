package content

// Vnum blocks. By convention each area numbers each kind of thing within
// a block of a hundred: Drevlin's rooms, items, and mobs all sit in 800 to
// 899, while the start area keeps rooms in 1 to 99, items in 100 to 199,
// and mobs in 200 to 299. Rooms, items, and mobs are separate namespaces,
// so an area has up to a hundred of each. The block for a kind is the
// hundred its lowest vnum falls in; a kind the area has none of yet uses
// the rooms' block, the first thing an area gets.

// BlockSize is how many vnums one area has for one kind.
const BlockSize = 100

// Kinds are the numbered kinds, as they appear in directory names.
var Kinds = []string{"rooms", "items", "mobs"}

// Block is one area's range for one kind, and how full it is.
type Block struct {
	First int `json:"first"`
	Last  int `json:"last"`
	// Used counts the area's vnums of this kind, in the block or not.
	Used int `json:"used"`
	// Free counts vnums in the block that nothing of this kind uses.
	Free int `json:"free"`
	// Next is the vnum a new one would get: the first free one after the
	// highest in use, or the first gap, or 0 when the block is full.
	Next int `json:"next"`
}

// vnumsOf returns every vnum of kind in the world, and the area's own.
func (w *World) vnumsOf(kind, area string) (all map[int]bool, mine []int) {
	all = map[int]bool{}
	add := func(v int, a string) {
		all[v] = true
		if a == area {
			mine = append(mine, v)
		}
	}
	switch kind {
	case "rooms":
		for v, r := range w.Rooms.Rooms {
			add(v, r.Area)
		}
	case "items":
		for v, p := range w.Items {
			add(v, p.Area)
		}
	case "mobs":
		for v, p := range w.Mobs {
			add(v, p.Area)
		}
	}
	return all, mine
}

// Blocks returns the area's block for each kind, keyed as in Kinds.
func Blocks(w *World, area string) map[string]Block {
	base := func(vnums []int) (int, bool) {
		if len(vnums) == 0 {
			return 0, false
		}
		low := vnums[0]
		for _, v := range vnums {
			low = min(low, v)
		}
		return low / BlockSize * BlockSize, true
	}
	_, roomVnums := w.vnumsOf("rooms", area)
	roomBase, haveRooms := base(roomVnums)
	out := map[string]Block{}
	for _, kind := range Kinds {
		all, mine := w.vnumsOf(kind, area)
		start, ok := base(mine)
		if !ok {
			start = roomBase
			if !haveRooms {
				// An empty area: no block to speak of.
				out[kind] = Block{}
				continue
			}
		}
		b := Block{First: max(start, 1), Last: start + BlockSize - 1, Used: len(mine)}
		top := 0
		for _, v := range mine {
			if v >= b.First && v <= b.Last {
				top = max(top, v)
			}
		}
		for v := b.First; v <= b.Last; v++ {
			if all[v] {
				continue
			}
			b.Free++
			if b.Next == 0 && v > top {
				b.Next = v
			}
		}
		if b.Next == 0 {
			for v := b.First; v <= b.Last; v++ {
				if !all[v] {
					b.Next = v
					break
				}
			}
		}
		out[kind] = b
	}
	return out
}
