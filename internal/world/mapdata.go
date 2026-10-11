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
// room order, so a second guard is "2.guard". Mobs self cannot see are
// left out.
func (w *World) roomEntities(r *room.Room, self *Player) (mobs, items []output.Entity) {
	chars := w.visibleCharactersIn(self.Character, r)
	for _, m := range w.contents(r).mobs {
		if !w.canSeeChar(self.Character, m.Character) {
			continue
		}
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
		mobs = append(mobs, output.Entity{Name: m.Name, Ref: ref(word, n), Services: mobServices(m)})
	}
	list := w.contents(r).items
	pool := w.searchPool(self)
	for _, g := range item.Group(list) {
		items = append(items, output.Entity{Name: g.Item.Name(), Ref: itemRef(list, pool, g.Item), Count: g.Count,
			Fixed: g.Item.Proto.HasFlag("nopickup"), Container: g.Item.IsContainer(), Rarity: g.Item.Rarity, Color: g.Item.Color})
	}
	return mobs, items
}

// mobServices lists what m does for players, in the words MapRoom uses.
func mobServices(m *Mob) []string {
	var out []string
	if isMerchant(m) {
		out = append(out, "shop")
	}
	if len(m.Proto.Teaches) > 0 {
		out = append(out, "trainer")
	}
	if isQuestmaster(m) {
		out = append(out, "quest")
	}
	if isVendor(m) {
		out = append(out, "vendor")
	}
	return out
}

// offerWord is the keyword of protos[i] that a buy command, which takes
// the first offer matching its word, resolves to protos[i] itself.
func offerWord(protos []*item.Proto, i int) string {
	for _, k := range protos[i].Keywords {
		for j, p := range protos {
			if p != nil && item.MatchKeywords(p.Keywords, k) {
				if j == i {
					return k
				}
				break
			}
		}
	}
	if len(protos[i].Keywords) > 0 {
		return protos[i].Keywords[0]
	}
	return ""
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
	scanned := w.scannable(p)
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
		mr := output.MapRoom{
			Vnum: cur.Vnum, Name: cur.Name, X: cur.X, Y: cur.Y, Z: cur.Z,
			Exits: exits, Seen: p.visited[cur.Vnum], Safe: cur.Safe(),
		}
		if mr.Seen {
			w.mapDetails(&mr, cur)
		}
		if scanned[cur.Vnum] {
			w.mapOccupants(&mr, cur, p)
		}
		if cur == r {
			mr.Items = len(w.contents(r).items) > 0
		}
		data.Rooms = append(data.Rooms, mr)
	}
	return data
}

// scannable is the player's room and the rooms scan would look into from
// it: through open exits, and lit.
func (w *World) scannable(p *Player) map[int]bool {
	out := map[int]bool{}
	if p.Room == nil || !w.canSee(p.Character) {
		return out
	}
	out[p.Room.Vnum] = true
	for _, dir := range w.openExits(p.Room) {
		if dest := w.passable(p.Room, dir); dest != nil && (!dest.Dark() || w.lightIn(dest)) {
			out[dest.Vnum] = true
		}
	}
	return out
}

// mapDetails fills what a visitor knows of a room.
func (w *World) mapDetails(mr *output.MapRoom, r *room.Room) {
	mr.Dark, mr.Temple = r.Dark(), r.Temple
	have := map[string]bool{}
	for _, m := range w.contents(r).mobs {
		for _, s := range []struct {
			name string
			ok   bool
		}{{"shop", isMerchant(m)}, {"trainer", len(m.Proto.Teaches) > 0}, {"quest", isQuestmaster(m)}, {"vendor", isVendor(m)}} {
			if s.ok && !have[s.name] {
				have[s.name] = true
				mr.Services = append(mr.Services, s.name)
			}
		}
	}
	for dir := range r.Doors {
		if mr.Doors == nil {
			mr.Doors = map[string]string{}
		}
		switch {
		case w.doorLocked(r, dir):
			mr.Doors[dir] = "locked"
		case w.doorClosed(r, dir):
			mr.Doors[dir] = "closed"
		default:
			mr.Doors[dir] = "open"
		}
	}
}

// mapOccupants counts who p could scan in r.
func (w *World) mapOccupants(mr *output.MapRoom, r *room.Room, p *Player) {
	first := true
	for _, c := range w.visibleCharactersIn(p.Character, r) {
		switch {
		case c == p.Character:
		case c.IsPlayer():
			mr.Players++
		case c.mob != nil && !c.mob.Proto.HasFlag("peaceful"):
			gap := c.Level - p.Level
			if first || gap > mr.Danger {
				mr.Danger = gap
			}
			first = false
			mr.Foes++
		}
	}
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
