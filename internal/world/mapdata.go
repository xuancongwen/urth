package world

import (
	"strconv"

	"urth/internal/item"
	"urth/internal/output"
	"urth/internal/room"
)

// Structured room data for clients: what is here, named the way a command
// would name it, and the map around the player.

// mapRadius is how many steps of exits the map message reaches.
const mapRadius = 6

// roomEntities lists the mobs and items in r with the references a
// command would resolve to the same thing. Mob references follow
// findCharacter, which counts prefix matches over players then mobs in
// room order, so a second guard is "2.guard".
func (w *World) roomEntities(r *room.Room, self *Player) (mobs, items []output.Entity) {
	chars := w.charactersIn(r)
	for _, m := range w.contents(r).mobs {
		word := ""
		if len(m.Keywords) > 0 {
			word = m.Keywords[0]
		}
		n := 0
		for _, c := range chars {
			if c == self.Character || !c.Matches(word) {
				continue
			}
			n++
			if c == m.Character {
				break
			}
		}
		mobs = append(mobs, output.Entity{Name: m.Name, Ref: ref(word, n)})
	}
	list := w.contents(r).items
	for _, g := range item.Group(list) {
		word := ""
		if len(g.Item.Proto.Keywords) > 0 {
			word = g.Item.Proto.Keywords[0]
		}
		n := 0
		for _, it := range list {
			if !it.Matches(word) {
				continue
			}
			n++
			if it == g.Item {
				break
			}
		}
		items = append(items, output.Entity{Name: g.Item.Name(), Ref: ref(word, n), Count: g.Count, Fixed: g.Item.Proto.HasFlag("nopickup")})
	}
	return mobs, items
}

func ref(word string, n int) string {
	if n <= 1 {
		return word
	}
	return strconv.Itoa(n) + "." + word
}

// mapAround walks exits from r within the same area up to mapRadius steps
// and returns the placed rooms found, with the player's visited marks.
func (w *World) mapAround(r *room.Room, p *Player) *output.MapData {
	if !r.Placed {
		return nil
	}
	data := &output.MapData{Area: r.Area}
	depth := map[int]int{r.Vnum: 0}
	queue := []*room.Room{r}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		exits := map[string]int{}
		for _, dir := range room.Directions {
			to, ok := cur.Exits[dir]
			if !ok {
				continue
			}
			exits[dir] = to
			next, ok := w.content.Rooms.Get(to)
			if !ok || next.Area != r.Area || !next.Placed {
				continue
			}
			if _, seen := depth[to]; seen || depth[cur.Vnum] >= mapRadius {
				continue
			}
			depth[to] = depth[cur.Vnum] + 1
			queue = append(queue, next)
		}
		data.Rooms = append(data.Rooms, output.MapRoom{
			Vnum: cur.Vnum, Name: cur.Name, X: cur.X, Y: cur.Y, Z: cur.Z,
			Exits: exits, Seen: p.visited[cur.Vnum], Safe: cur.Safe(),
		})
	}
	return data
}

// sendCommands tells a client which commands exist, for completion.
func (w *World) sendCommands(p *Player) {
	var names []string
	for _, c := range commands {
		if c.admin && !p.Admin {
			continue
		}
		names = append(names, c.name)
	}
	p.SendMsg(output.Message{Type: output.Commands, Data: names})
}
