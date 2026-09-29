package world

import (
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
