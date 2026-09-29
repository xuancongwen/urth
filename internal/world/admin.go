package world

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"sort"
	"strings"

	"urth/internal/copyover"
	"urth/internal/item"
	"urth/internal/output"
	"urth/internal/session"
)

// Admin commands. Nothing here is a game rule.

func cmdShutdown(w *World, p *Player, _ string) {
	if w.deps.Shutdown == nil {
		p.Send("Shutdown is not available.\n")
		return
	}
	w.log.Warn("shutdown requested", "by", p.Name)
	w.broadcast("{R}Shutdown by " + output.Escape(p.Name) + ".{x}\n")
	w.deps.Shutdown()
}

// cmdCopyover saves everyone, hands every socket it can to a fresh copy of
// the binary, and execs it. Clients whose socket cannot be inherited
// (WebSocket) are given a token to reconnect with. Sessions still logging
// in are closed.
func cmdCopyover(w *World, p *Player, _ string) {
	if w.deps.Copyover == nil {
		p.Send("Copyover is not available.\n")
		return
	}
	w.log.Warn("copyover requested", "by", p.Name)

	var st copyover.State
	if w.deps.Listeners != nil {
		st.Listeners = w.deps.Listeners()
	}
	var files []*os.File
	for _, o := range w.players {
		if o.State != StatePlaying {
			o.SendMsg(output.Message{Type: output.System, Text: "\nThe server is restarting. Please reconnect in a moment.\n"})
			o.disconnect()
			continue
		}
		entry := copyover.Player{Name: o.Name, Room: 0}
		if o.Room != nil {
			entry.Room = o.Room.Vnum
		}
		if f, ok := o.conn.(session.Filer); ok {
			file, err := f.File()
			if err == nil {
				err = copyover.Inherit(int(file.Fd()))
			}
			if err != nil {
				w.log.Error("copyover: cannot hand over socket", "name", o.Name, "err", err)
				o.SendMsg(output.Message{Type: output.System, Text: "\nThe server is restarting. Please reconnect in a moment.\n"})
				o.disconnect()
				continue
			}
			files = append(files, file)
			entry.Kind = "telnet"
			entry.FD = int(file.Fd())
		} else {
			entry.Kind = "web"
			entry.Token = newToken()
			o.SendMsg(output.Message{Type: output.Reconnect, Data: output.ReconnectData{Token: entry.Token}})
		}
		st.Players = append(st.Players, entry)
	}

	w.saveAll()
	w.broadcast("{Y}Copyover by " + output.Escape(p.Name) + ". Please hold...{x}\n")
	w.flush()

	err := w.deps.Copyover(st)
	// Only reached on failure.
	for _, f := range files {
		f.Close()
	}
	w.log.Error("copyover failed", "err", err)
	if w.stillConnected(p) {
		p.Send("{R}Copyover failed: " + output.Escape(err.Error()) + "{x}\n")
	}
	w.broadcast("{Y}Copyover failed; carrying on.{x}\n")
}

// outfitFlag marks the item prototypes cmdOutfit hands out. The set lives
// in the balance lab (data/world/balance, items 900 to 914); a builder
// adds a piece by giving it the flag.
const outfitFlag = "outfit"

// newbieFlag marks the sub issue gear every new character starts with
// (data/world/start, items 130 to 134), as ROM's outfit did.
const newbieFlag = "newbie"

// cmdOutfit: outfit. Creates one of every item flagged "outfit" and puts
// each on where a slot is free; the rest go in the pack.
func cmdOutfit(w *World, p *Player, _ string) {
	worn, packed := w.outfit(p.Character, outfitFlag)
	if len(worn)+len(packed) == 0 {
		p.Send("No items are flagged " + outfitFlag + ".\n")
		return
	}
	w.recalc(p.Character)
	var b strings.Builder
	b.WriteString("{Y}The regalia answers your call.{x}\n")
	for _, s := range p.equippedList() {
		for _, it := range worn {
			if p.Equipment[s] == it {
				b.WriteString(slotLabel(s) + output.Escape(it.Name()) + "\n")
			}
		}
	}
	if len(packed) > 0 {
		b.WriteString("Into your pack, for want of a free slot:\n")
		for _, it := range packed {
			b.WriteString("    " + output.Escape(it.Name()) + "\n")
		}
	}
	p.Send(b.String())
	w.act("$n is suddenly clad in blazing regalia.", p.Character, nil, "", toRoom)
	w.save(p)
}

// outfit creates every item flagged flag for c, in vnum order, and
// equips each piece in the first free position its slot allows. It
// returns what it put on and what it left in the inventory.
func (w *World) outfit(c *Character, flag string) (worn, packed []*item.Item) {
	var protos []*item.Proto
	for _, p := range w.content.Items {
		if p.HasFlag(flag) {
			protos = append(protos, p)
		}
	}
	sort.Slice(protos, func(i, j int) bool { return protos[i].Vnum < protos[j].Vnum })
	for _, proto := range protos {
		it := item.New(proto)
		c.Inventory = append(c.Inventory, it)
		if w.equipFree(c, it) {
			worn = append(worn, it)
		} else {
			packed = append(packed, it)
		}
	}
	return worn, packed
}

// equipFree puts it on c in a free position, reporting whether one was.
func (w *World) equipFree(c *Character, it *item.Item) bool {
	slot := it.Proto.WearSlot()
	if slot == "" {
		return false
	}
	for _, pos := range item.Positions(slot) {
		if c.equip(it, pos) {
			return true
		}
	}
	return false
}

func newToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
