package room

import (
	"fmt"
	"sort"
)

// Coord is a builder-supplied map position, in room units. A room with a
// Position is an anchor: Layout places every room reachable from it by
// walking exits, one unit per step, and reports collisions. Areas are laid
// out independently; an exit into another area never moves the walk.
type Coord struct {
	X int `yaml:"x"`
	Y int `yaml:"y"`
	Z int `yaml:"z"`
}

// deltas is the map step per direction. North is up the screen, so it
// decreases Y; up and down change Z.
var deltas = map[string][3]int{
	"north": {0, -1, 0}, "south": {0, 1, 0},
	"east": {1, 0, 0}, "west": {-1, 0, 0},
	"up": {0, 0, 1}, "down": {0, 0, -1},
}

// Layout assigns X, Y, Z to every room. Anchored rooms keep their
// position; the rest are placed by walking exits from the anchors, and any
// component with no anchor is anchored at its lowest vnum, offset to the
// right of what is already placed. A room whose derived position is taken
// is reported and placed as a new component instead, so the map never
// draws two rooms on one cell. Warnings name every such conflict.
func (w *World) Layout() []string {
	var warnings []string
	byArea := map[string][]*Room{}
	for _, r := range w.Rooms {
		byArea[r.Area] = append(byArea[r.Area], r)
		r.Placed = false
	}
	areas := make([]string, 0, len(byArea))
	for a := range byArea {
		areas = append(areas, a)
	}
	sort.Strings(areas)
	for _, area := range areas {
		rooms := byArea[area]
		sort.Slice(rooms, func(i, j int) bool { return rooms[i].Vnum < rooms[j].Vnum })
		occupied := map[[3]int]*Room{}
		maxX := -1
		place := func(r *Room, x, y, z int) {
			r.X, r.Y, r.Z, r.Placed = x, y, z, true
			occupied[[3]int{x, y, z}] = r
			if x > maxX {
				maxX = x
			}
		}
		var queue []*Room
		for _, r := range rooms {
			if r.Position == nil {
				continue
			}
			if other, taken := occupied[[3]int{r.Position.X, r.Position.Y, r.Position.Z}]; taken {
				warnings = append(warnings, fmt.Sprintf("area %s: room %d (%s) has the same position as room %d (%s)", area, r.Vnum, r.Name, other.Vnum, other.Name))
				continue
			}
			place(r, r.Position.X, r.Position.Y, r.Position.Z)
			queue = append(queue, r)
		}
		walk := func() {
			for len(queue) > 0 {
				r := queue[0]
				queue = queue[1:]
				for _, dir := range Directions {
					to, ok := r.Exits[dir]
					if !ok {
						continue
					}
					next := w.Rooms[to]
					if next == nil || next.Area != area || next.Placed {
						continue
					}
					d := deltas[dir]
					pos := [3]int{r.X + d[0], r.Y + d[1], r.Z + d[2]}
					if other, taken := occupied[pos]; taken {
						warnings = append(warnings, fmt.Sprintf("area %s: room %d (%s) %s leads to room %d (%s) but room %d (%s) is already there; give one a position", area, r.Vnum, r.Name, dir, next.Vnum, next.Name, other.Vnum, other.Name))
						continue
					}
					place(next, pos[0], pos[1], pos[2])
					queue = append(queue, next)
				}
			}
		}
		walk()
		for _, r := range rooms {
			if r.Placed {
				continue
			}
			// A component with no anchor, or a room a collision left out.
			x := 0
			if maxX >= 0 {
				x = maxX + 2
			}
			place(r, x, 0, 0)
			queue = append(queue, r)
			walk()
		}
	}
	return warnings
}
