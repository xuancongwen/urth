package world

import (
	"strings"

	"urth/internal/item"
	"urth/internal/output"
)

// Auto actions after a kill, each a toggle saved with the character:
// autoloot takes everything from the corpse, autogold only its coins, and
// autosac gives the corpse to the gods once nothing is left in it.

// autoToggle flips one saved setting and says where it now stands.
func autoToggle(w *World, p *Player, flag *bool, on, off string) {
	*flag = !*flag
	if *flag {
		p.Send(on + "\n")
	} else {
		p.Send(off + "\n")
	}
	w.save(p)
}

// cmdAutoloot: autoloot. Toggle taking everything from your kills.
func cmdAutoloot(w *World, p *Player, _ string) {
	if p.rec == nil {
		return
	}
	autoToggle(w, p, &p.rec.AutoLoot, "You will now loot your kills.", "You will no longer loot your kills.")
}

// cmdAutogold: autogold. Toggle taking the coins from your kills.
func cmdAutogold(w *World, p *Player, _ string) {
	if p.rec == nil {
		return
	}
	autoToggle(w, p, &p.rec.AutoGold, "You will now take the coins from your kills.", "You will no longer take the coins from your kills.")
}

// cmdAutosac: autosac. Toggle sacrificing your kills' empty corpses.
func cmdAutosac(w *World, p *Player, _ string) {
	if p.rec == nil {
		return
	}
	autoToggle(w, p, &p.rec.AutoSac, "You will now sacrifice your kills' corpses once they are empty.", "You will no longer sacrifice your kills' corpses.")
}

// autoLoot runs the killer's auto actions on a fresh corpse.
func (w *World) autoLoot(p *Player, corpse *item.Item) {
	if p == nil || p.rec == nil {
		return
	}
	if p.rec.AutoLoot || p.rec.AutoGold {
		w.lootCorpse(p, corpse, !p.rec.AutoLoot)
	}
	if p.rec.AutoSac && len(corpse.Contents) == 0 {
		w.sacrificeItem(p, corpse)
	}
}

// lootCorpse takes what p can from the corpse, only the coins if
// coinsOnly, with the same messages as get.
func (w *World) lootCorpse(p *Player, corpse *item.Item, coinsOnly bool) {
	from := output.Escape(corpse.Name())
	for _, it := range append([]*item.Item(nil), corpse.Contents...) {
		coins := it.Proto.HasFlag("coins")
		if coinsOnly && !coins {
			continue
		}
		if ok, why := canTake(p, it); !ok {
			p.Send(why + "\n")
			continue
		}
		corpse.Contents = item.Remove(corpse.Contents, it)
		if takeCoins(p.Character, it) {
			w.act("You get $t from "+from+".", p.Character, nil, escapeMoney(it.Coins), toChar)
			w.act("$n gets some coins from "+from+".", p.Character, nil, "", toRoom)
			continue
		}
		p.Inventory = append(p.Inventory, it)
		w.actItem("You get $p from $t.", p.Character, nil, output.Escape(it.Name()), from, toChar)
		w.actItem("$n gets $p from $t.", p.Character, nil, output.Escape(it.Name()), from, toRoom)
	}
}

// A setting is one of a player's saved toggles, as settings lists them.
type setting struct {
	name, about string
	on          func(p *Player) bool
	toggle      func(w *World, p *Player, args string)
}

var playerSettings = []setting{
	{"autoloot", "take everything from your kills", func(p *Player) bool { return p.rec != nil && p.rec.AutoLoot }, cmdAutoloot},
	{"autogold", "take the coins from your kills", func(p *Player) bool { return p.rec != nil && p.rec.AutoGold }, cmdAutogold},
	{"autosac", "sacrifice your kills' corpses once they are empty", func(p *Player) bool { return p.rec != nil && p.rec.AutoSac }, cmdAutosac},
	{"color", "color in what you read", func(p *Player) bool { return p.Color }, cmdColor},
}

// cmdSettings: settings | settings <name>. Lists every toggle and where it
// stands; with a name, flips that one, as typing its own command does.
func cmdSettings(w *World, p *Player, args string) {
	if word := strings.ToLower(strings.TrimSpace(args)); word != "" {
		for _, s := range playerSettings {
			if strings.HasPrefix(s.name, word) {
				s.toggle(w, p, "")
				return
			}
		}
		p.Send("No such setting. Type settings to see them.\n")
		return
	}
	var b strings.Builder
	b.WriteString("Your settings. Flip one with its name, or settings <name>.\n")
	for _, s := range playerSettings {
		state := "{R}off{x}"
		if s.on(p) {
			state = "{G}on {x}"
		}
		b.WriteString("  " + padRight(s.name, 10) + state + "  " + s.about + "\n")
	}
	p.Send(b.String())
}
