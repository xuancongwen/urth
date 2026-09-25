package world

import (
	"urth/internal/effect"
	"urth/internal/item"
	"urth/internal/output"
	"urth/internal/store"
)

// Inventory persistence: player files store prototype vnums, and instances
// are rebuilt at login. Unknown vnums (an item removed from the world) are
// dropped with a warning.

func saveItems(list []*item.Item) []store.SavedItem {
	var out []store.SavedItem
	for _, it := range list {
		if it.Proto.Vnum == 0 {
			continue // corpses and other synthetic items are not saved
		}
		out = append(out, store.SavedItem{Vnum: it.Proto.Vnum, Contents: saveItems(it.Contents), Effects: it.Effects})
	}
	return out
}

func (w *World) loadItems(saved []store.SavedItem, owner string) []*item.Item {
	var out []*item.Item
	for _, s := range saved {
		proto, ok := w.content.Items[s.Vnum]
		if !ok {
			w.log.Warn("dropping unknown item from player file", "player", owner, "vnum", s.Vnum)
			continue
		}
		it := item.New(proto)
		it.Contents = w.loadItems(s.Contents, owner)
		it.Effects = s.Effects
		out = append(out, it)
	}
	return out
}

func (w *World) saveCharacterItems(p *Player) {
	p.rec.Inventory = saveItems(p.Inventory)
	p.rec.Equipment = map[string]store.SavedItem{}
	for slot, it := range p.Equipment {
		p.rec.Equipment[string(slot)] = store.SavedItem{Vnum: it.Proto.Vnum, Contents: saveItems(it.Contents), Effects: it.Effects}
	}
}

func (w *World) loadCharacterItems(p *Player) {
	p.Inventory = w.loadItems(p.rec.Inventory, p.Name)
	p.Equipment = map[item.Slot]*item.Item{}
	for slot, s := range p.rec.Equipment {
		if !item.ValidSlot(item.Slot(slot)) {
			w.log.Warn("dropping item in unknown slot from player file", "player", p.Name, "slot", slot)
			continue
		}
		list := w.loadItems([]store.SavedItem{s}, p.Name)
		if len(list) == 1 {
			p.Equipment[item.Slot(slot)] = list[0]
		}
	}
}

// loadSheet restores level, experience, stats, and vitals, then derives
// maxima. A brand-new record has zero health; it starts full. A record
// with no stats yet (new, or made before the rules defined any) gets its
// sheet from the onCreate hook.
func (w *World) loadSheet(p *Player) {
	p.Level = max(p.rec.Level, 1)
	p.Experience = p.rec.Experience
	p.Stats = copyStats(p.rec.Stats)
	p.StatPoints = p.rec.StatPoints
	p.Effects = append([]effect.Active(nil), p.rec.Effects...)
	if len(p.Stats) == 0 {
		r := w.onCreate(p.Character)
		for k, v := range r.Stats {
			p.Stats[k] = v
		}
		p.StatPoints += max(r.StatPoints, 0)
		if r.Message != "" {
			p.Send(output.Escape(r.Message) + "\n")
		}
	}
	w.recalc(p.Character)
	if p.rec.Health <= 0 {
		p.Health, p.Mana = p.HealthMax, p.ManaMax
	} else {
		p.Health = clamp(p.rec.Health, 1, p.HealthMax)
		p.Mana = clamp(p.rec.Mana, 0, p.ManaMax)
	}
}

func (w *World) saveSheet(p *Player) {
	p.rec.Level = p.Level
	p.rec.Experience = p.Experience
	p.rec.Stats = copyStats(p.Stats)
	p.rec.StatPoints = p.StatPoints
	p.rec.Effects = append([]effect.Active(nil), p.Effects...)
	p.rec.Health = p.Health
	p.rec.Mana = p.Mana
}
