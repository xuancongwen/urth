package world

import (
	"errors"
	"sort"
	"time"

	"urth/internal/builder"
	"urth/internal/content"
	"urth/internal/item"
)

// The builder page's view of the world. Every method runs its work on the
// world goroutine through post and waits for it, so the page never reads
// game state concurrently with the tick.

// builderCallTimeout bounds how long the page waits for the world.
const builderCallTimeout = 5 * time.Second

var errWorldBusy = errors.New("the world did not answer in time")

type builderAPI struct{ w *World }

// Builder returns the adapter the builder server drives.
func (w *World) Builder() builder.API { return builderAPI{w} }

func (b builderAPI) call(fn func()) error {
	done := make(chan struct{})
	select {
	case b.w.posts <- func() { fn(); close(done) }:
	case <-time.After(builderCallTimeout):
		return errWorldBusy
	}
	select {
	case <-done:
		return nil
	case <-time.After(builderCallTimeout):
		return errWorldBusy
	}
}

func (b builderAPI) Generation() uint64 { return b.w.generation.Load() }

func (b builderAPI) Snapshot() (*builder.Snapshot, error) {
	var snap *builder.Snapshot
	err := b.call(func() { snap = b.w.builderSnapshot() })
	return snap, err
}

func (b builderAPI) Area(name string) (*builder.AreaView, error) {
	var view *builder.AreaView
	var err error
	if cerr := b.call(func() { view, err = b.w.builderArea(name) }); cerr != nil {
		return nil, cerr
	}
	return view, err
}

func (b builderAPI) Reload(area string) (string, error) {
	var report string
	var err error
	if cerr := b.call(func() { report, err = b.w.reloadContent(area) }); cerr != nil {
		return "", cerr
	}
	return report, err
}

func (w *World) builderSnapshot() *builder.Snapshot {
	problems := content.Lint(w.content, w.cfg.World.StartRoom)
	byArea := map[string]*builder.AreaSummary{}
	for name, a := range w.content.Rooms.Areas {
		byArea[name] = &builder.AreaSummary{Name: name, Title: a.Name, Detached: a.Detached}
	}
	for _, r := range w.content.Rooms.Rooms {
		byArea[r.Area].Rooms++
	}
	for _, p := range w.content.Items {
		if a, ok := byArea[p.Area]; ok {
			a.Items++
		}
	}
	for _, p := range w.content.Mobs {
		if a, ok := byArea[p.Area]; ok {
			a.Mobs++
		}
	}
	for _, p := range problems {
		a, ok := byArea[p.Area]
		if !ok {
			continue
		}
		if p.Level == content.Error {
			a.Errors++
		} else {
			a.Warnings++
		}
	}
	snap := &builder.Snapshot{StartRoom: w.cfg.World.StartRoom, Generation: w.generation.Load(), Problems: problems}
	for _, a := range byArea {
		snap.Areas = append(snap.Areas, *a)
	}
	sort.Slice(snap.Areas, func(i, j int) bool { return snap.Areas[i].Name < snap.Areas[j].Name })
	if snap.Problems == nil {
		snap.Problems = []content.Problem{}
	}
	return snap
}

func (w *World) builderArea(name string) (*builder.AreaView, error) {
	area, ok := w.content.Rooms.Areas[name]
	if !ok {
		return nil, errors.New("no area named " + name)
	}
	view := &builder.AreaView{Name: name, Title: area.Name, Detached: area.Detached,
		Rooms: []builder.RoomView{}, Items: []builder.ProtoView{}, Mobs: []builder.ProtoView{},
		Resets: []builder.ResetView{}, Problems: []content.Problem{}}
	for _, p := range content.Lint(w.content, w.cfg.World.StartRoom) {
		if p.Area == name {
			view.Problems = append(view.Problems, p)
		}
	}

	liveItems, liveMobs := w.liveCounts()
	for _, r := range w.content.Rooms.Rooms {
		if r.Area != name {
			continue
		}
		rv := builder.RoomView{Vnum: r.Vnum, Name: r.Name, Description: r.Description, File: r.File,
			X: r.X, Y: r.Y, Z: r.Z, Placed: r.Placed, Exits: r.Exits, Flags: r.Flags, Temple: r.Temple}
		for dir, to := range r.Exits {
			if next, ok := w.content.Rooms.Get(to); ok && next.Area != name {
				if rv.Links == nil {
					rv.Links = map[string]builder.Link{}
				}
				rv.Links[dir] = builder.Link{Vnum: to, Name: next.Name, Area: next.Area}
			}
		}
		for _, p := range w.playersIn(r) {
			rv.Players = append(rv.Players, p.Name)
		}
		c := w.contents(r)
		for _, m := range c.mobs {
			rv.Mobs = append(rv.Mobs, m.Name)
		}
		for _, g := range item.Group(c.items) {
			n := g.Item.Name()
			if g.Count > 1 {
				n = "(" + itoa(g.Count) + ") " + n
			}
			rv.Items = append(rv.Items, n)
		}
		view.Rooms = append(view.Rooms, rv)
	}
	sort.Slice(view.Rooms, func(i, j int) bool { return view.Rooms[i].Vnum < view.Rooms[j].Vnum })

	for _, p := range w.content.Items {
		if p.Area == name {
			view.Items = append(view.Items, builder.ProtoView{Vnum: p.Vnum, Name: p.Name, Level: p.Level,
				Type: string(p.Type), Slot: string(p.Slot), File: p.File, Live: liveItems[p.Vnum]})
		}
	}
	sort.Slice(view.Items, func(i, j int) bool { return view.Items[i].Vnum < view.Items[j].Vnum })
	for _, p := range w.content.Mobs {
		if p.Area == name {
			view.Mobs = append(view.Mobs, builder.ProtoView{Vnum: p.Vnum, Name: p.Name, Level: p.Level,
				File: p.File, Live: liveMobs[p.Vnum]})
		}
	}
	sort.Slice(view.Mobs, func(i, j int) bool { return view.Mobs[i].Vnum < view.Mobs[j].Vnum })

	itemName := func(vnum int) string {
		if p, ok := w.content.Items[vnum]; ok {
			return p.Name
		}
		return "?"
	}
	if resets, ok := w.content.Resets[name]; ok {
		for i, rs := range resets.Resets {
			rv := builder.ResetView{Index: i, Room: rs.Room, Mob: rs.Mob, Item: rs.Item, Into: rs.Into, Max: rs.Max}
			if r, ok := w.content.Rooms.Get(rs.Room); ok {
				rv.RoomName = r.Name
			}
			if rs.Mob != 0 {
				if p, ok := w.content.Mobs[rs.Mob]; ok {
					rv.Name = p.Name
				}
			} else {
				rv.Name = itemName(rs.Item)
			}
			for _, eq := range rs.Equip {
				rv.Equip = append(rv.Equip, builder.EquipView{Item: eq.Item, Name: itemName(eq.Item), Slot: eq.Slot})
			}
			view.Resets = append(view.Resets, rv)
		}
	}
	return view, nil
}

// liveCounts tallies item and mob instances in the world by prototype:
// on the floor, in containers, and carried or worn by anyone.
func (w *World) liveCounts() (items, mobs map[int]int) {
	items, mobs = map[int]int{}, map[int]int{}
	var walk func(list []*item.Item)
	walk = func(list []*item.Item) {
		for _, it := range list {
			items[it.Proto.Vnum]++
			walk(it.Contents)
		}
	}
	carried := func(c *Character) {
		walk(c.Inventory)
		for _, it := range c.Equipment {
			walk([]*item.Item{it})
		}
	}
	for _, c := range w.rooms {
		walk(c.items)
		for _, m := range c.mobs {
			mobs[m.Proto.Vnum]++
			carried(m.Character)
		}
	}
	for _, p := range w.players {
		if p.State == StatePlaying {
			carried(p.Character)
		}
	}
	return items, mobs
}
