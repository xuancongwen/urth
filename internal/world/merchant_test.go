package world

import (
	"strings"
	"testing"

	"urth/internal/item"
	"urth/internal/mob"
)

// makeMerchant turns the city guard in the hub into a merchant: the rusty
// sword for silver, and the leather cap for silver and two loaves.
func makeMerchant(w *World) {
	guard := w.content.Mobs[20]
	guard.Flags = append(guard.Flags, "peaceful")
	guard.Trades = []mob.Trade{{Item: 10, Silver: 150}, {Item: 11, Silver: 20, Items: []int{13, 13}}}
}

func TestMerchantBuy(t *testing.T) {
	w := testWorld(t)
	makeMerchant(w)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]

	send(w, 1, "list")
	o := bob.take()
	if !strings.Contains(o, "a rusty sword") || !strings.Contains(o, "1 gold and 50 silver") ||
		!strings.Contains(o, "20 silver and 2 x a loaf of bread") {
		t.Fatalf("list: %q", o)
	}

	p.Silver = 100
	send(w, 1, "buy sword")
	if o := bob.take(); !strings.Contains(o, "Come back when you have it") || p.Silver != 100 {
		t.Fatalf("buy short: %q", o)
	}
	p.Silver = 200
	send(w, 1, "buy sword")
	if o := bob.take(); !strings.Contains(o, "You trade 1 gold and 50 silver to a city guard for a rusty sword") || p.Silver != 50 {
		t.Fatalf("buy: %q silver %d", o, p.Silver)
	}

	// Money and goods: one loaf is not enough.
	bread := w.content.Items[13]
	p.Inventory = append(p.Inventory, item.New(bread))
	send(w, 1, "buy cap")
	if o := bob.take(); !strings.Contains(o, "Come back") || p.Silver != 50 {
		t.Fatalf("buy cap short: %q", o)
	}
	p.Inventory = append(p.Inventory, item.New(bread))
	send(w, 1, "buy cap")
	if o := bob.take(); !strings.Contains(o, "for a leather cap") {
		t.Fatalf("buy cap: %q", o)
	}
	if p.Silver != 30 || len(item.Find(p.Inventory, item.ParseTarget("bread"))) != 0 ||
		len(item.Find(p.Inventory, item.ParseTarget("cap"))) != 1 {
		t.Fatalf("after cap: silver %d inventory %v", p.Silver, p.Inventory)
	}

	send(w, 1, "buy moon")
	if o := bob.take(); !strings.Contains(o, "does not sell that") {
		t.Fatalf("buy unknown: %q", o)
	}
}

func TestMerchantSell(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]

	send(w, 1, "sell sword")
	if o := bob.take(); !strings.Contains(o, "Nobody here is buying") {
		t.Fatalf("no merchant: %q", o)
	}
	makeMerchant(w)
	sword := *w.content.Items[10]
	sword.Value = 81
	p.Inventory = append(p.Inventory, item.New(&sword), item.New(w.content.Items[13]))
	send(w, 1, "sell bread")
	if o := bob.take(); !strings.Contains(o, "worth nothing") {
		t.Fatalf("worthless: %q", o)
	}
	send(w, 1, "sell sword")
	if o := bob.take(); !strings.Contains(o, "You sell a rusty sword to a city guard for 40 silver") || p.Silver != 40 {
		t.Fatalf("sell: %q silver %d", o, p.Silver)
	}
}
