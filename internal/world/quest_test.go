package world

import (
	"path/filepath"
	"strings"
	"testing"

	"urth/internal/item"
	"urth/internal/mob"
	"urth/internal/session"
)

// makeHerald turns the city guard in the hub into a questmaster that also
// sells the rusty sword and the leather cap for quest points.
func makeHerald(w *World) {
	guard := w.content.Mobs[20]
	guard.Flags = append(guard.Flags, "peaceful", "questmaster")
	guard.Sells = []mob.Sale{{Item: 10, Points: 20}, {Item: 11, Points: 100}}
}

// addQuestItem gives the test world one plantable quest item.
func addQuestItem(w *World) {
	p := &item.Proto{Vnum: 19, Name: "a bundle of dispatches", Keywords: []string{"bundle", "dispatches"}, Description: "A bundle of dispatches lies here.", Flags: []string{"quest"}, Area: "a"}
	p.ResolveStated()
	w.content.Items[19] = p
}

// requestUntil asks for quests, quitting the wrong kind, until one of the
// wanted kind is drawn.
func requestUntil(t *testing.T, w *World, c *fakeConn, p *Player, kind string) {
	t.Helper()
	for i := 0; i < 40; i++ {
		p.questWait = 0
		send(w, 1, "quest request")
		c.take()
		if p.quest != nil && p.quest.Kind == kind {
			return
		}
		send(w, 1, "quest quit")
		c.take()
	}
	t.Fatalf("no %s quest drawn in 40 tries", kind)
}

func TestQuestKill(t *testing.T) {
	w := testWorld(t)
	makeHerald(w)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]

	send(w, 1, "quest")
	if o := bob.take(); !strings.Contains(o, "You are not on a quest") {
		t.Fatalf("no quest: %q", o)
	}
	send(w, 1, "quest complete")
	if o := bob.take(); !strings.Contains(o, "You are not on a quest") {
		t.Fatalf("complete without quest: %q", o)
	}
	// The only candidate is the stray dog up north: the guard is the
	// questmaster, and there are no quest items to fetch.
	send(w, 1, "quest request")
	o := bob.take()
	if !strings.Contains(o, "Slay a stray dog") || !strings.Contains(o, "quest points") {
		t.Fatalf("request: %q", o)
	}
	if p.quest == nil || p.quest.Kind != "kill" || p.quest.Target != 21 || p.quest.Points != 5+1 {
		t.Fatalf("quest drawn wrong: %+v", p.quest)
	}
	send(w, 1, "quest request")
	if o := bob.take(); !strings.Contains(o, "Finish the quest you have first") {
		t.Fatalf("second request: %q", o)
	}
	send(w, 1, "quest complete")
	if o := bob.take(); !strings.Contains(o, "You have not finished") {
		t.Fatalf("early complete: %q", o)
	}
	send(w, 1, "quest info")
	if o := bob.take(); !strings.Contains(o, "on a quest to slay a stray dog") || !strings.Contains(o, "left") {
		t.Fatalf("info: %q", o)
	}

	send(w, 1, "north")
	bob.take()
	for _, m := range w.contents(p.Room).mobs {
		m.Health = 1
	}
	send(w, 1, "kill dog")
	o = tickUntil(t, w, bob, "almost done")
	if !strings.Contains(o, "is DEAD") || !p.quest.Done {
		t.Fatalf("kill credit: %q done=%v", o, p.quest.Done)
	}
	send(w, 1, "quest info")
	if o := bob.take(); !strings.Contains(o, "Return to the questmaster") {
		t.Fatalf("info after kill: %q", o)
	}
	send(w, 1, "south")
	bob.take()
	send(w, 1, "quest complete")
	o = bob.take()
	if !strings.Contains(o, "Well done") || !strings.Contains(o, "6 quest points") {
		t.Fatalf("complete: %q", o)
	}
	if p.quest != nil || p.questPoints != 6 || p.questWait <= 0 {
		t.Fatalf("after complete: quest=%v points=%d wait=%d", p.quest, p.questPoints, p.questWait)
	}
	send(w, 1, "quest request")
	if o := bob.take(); !strings.Contains(o, "Come back in") {
		t.Fatalf("cooldown: %q", o)
	}
	send(w, 1, "score")
	if o := bob.take(); !strings.Contains(o, "Quest points") || !strings.Contains(o, "6") {
		t.Fatalf("score: %q", o)
	}
}

func TestQuestFetchIsOwnedAndCompletes(t *testing.T) {
	w := testWorld(t)
	makeHerald(w)
	addQuestItem(w)
	bob := login(t, w, 1, "Bob")
	al := login(t, w, 2, "Ann")
	p := w.players[1]
	requestUntil(t, w, bob, p, "fetch")
	if p.quest.Target != 19 {
		t.Fatalf("fetch target: %+v", p.quest)
	}
	// The item was planted in a room of area a that is not safe: 1 or 2.
	var where int
	for vnum, c := range w.rooms {
		for _, it := range c.items {
			if it.Quest == "Bob" {
				where = vnum
			}
		}
	}
	if where != p.quest.Room || (where != 1 && where != 2) {
		t.Fatalf("planted in %d, quest says %d", where, p.quest.Room)
	}
	walk := func(id session.ID, c *fakeConn) {
		if where == 2 {
			send(w, id, "north")
			c.take()
		}
	}
	walk(2, al)
	send(w, 2, "get bundle")
	if o := al.take(); !strings.Contains(o, "belongs to Bob's quest") {
		t.Fatalf("other player took quest item: %q", o)
	}
	walk(1, bob)
	send(w, 1, "get bundle")
	if o := bob.take(); !strings.Contains(o, "You get a bundle of dispatches") {
		t.Fatalf("owner get: %q", o)
	}
	send(w, 1, "quest info")
	if o := bob.take(); !strings.Contains(o, "Return to the questmaster") {
		t.Fatalf("info holding item: %q", o)
	}
	if where == 2 {
		send(w, 1, "south")
		bob.take()
	}
	send(w, 1, "quest complete")
	o := bob.take()
	if !strings.Contains(o, "You hand a bundle of dispatches to a city guard") || !strings.Contains(o, "Well done") {
		t.Fatalf("complete: %q", o)
	}
	if p.quest != nil || p.questPoints != 6 || p.questItemHeld() != nil || len(item.Find(p.Inventory, item.ParseTarget("bundle"))) != 0 {
		t.Fatalf("after complete: quest=%v points=%d inv=%v", p.quest, p.questPoints, p.Inventory)
	}
}

func TestQuestQuitSweepsAndExpires(t *testing.T) {
	w := testWorld(t)
	makeHerald(w)
	addQuestItem(w)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	requestUntil(t, w, bob, p, "fetch")
	send(w, 1, "quest quit")
	if o := bob.take(); !strings.Contains(o, "You give up") {
		t.Fatalf("quit: %q", o)
	}
	if p.quest != nil || p.questWait != 300 || w.questItemPlanted(p) {
		t.Fatalf("after quit: quest=%v wait=%d planted=%v", p.quest, p.questWait, w.questItemPlanted(p))
	}

	// Expiry: run the clock down.
	p.questWait = 0
	send(w, 1, "quest request")
	bob.take()
	if p.quest == nil {
		t.Fatal("no quest")
	}
	p.quest.Rounds = 2
	w.round()
	w.round()
	w.flush()
	if o := bob.take(); !strings.Contains(o, "Your time is up") {
		t.Fatalf("expiry: %q", o)
	}
	if p.quest != nil || p.questWait != 150 || w.questItemPlanted(p) {
		t.Fatalf("after expiry: quest=%v wait=%d planted=%v", p.quest, p.questWait, w.questItemPlanted(p))
	}
}

func TestQuestVendor(t *testing.T) {
	w := testWorld(t)
	makeHerald(w)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	send(w, 1, "quest list")
	o := bob.take()
	if !strings.Contains(o, "a rusty sword") || !strings.Contains(o, "20 qp") || !strings.Contains(o, "You have 0 quest points") {
		t.Fatalf("list: %q", o)
	}
	send(w, 1, "quest buy sword")
	if o := bob.take(); !strings.Contains(o, "You have 0") {
		t.Fatalf("buy broke: %q", o)
	}
	p.questPoints = 50
	send(w, 1, "quest buy sword")
	if o := bob.take(); !strings.Contains(o, "You trade 20 quest points to a city guard for a rusty sword") {
		t.Fatalf("buy: %q", o)
	}
	if p.questPoints != 30 || len(item.Find(p.Inventory, item.ParseTarget("sword"))) != 1 {
		t.Fatalf("after buy: points=%d inv=%v", p.questPoints, p.Inventory)
	}
	send(w, 1, "quest buy bread")
	if o := bob.take(); !strings.Contains(o, "does not offer that") {
		t.Fatalf("unknown: %q", o)
	}
	send(w, 1, "quest points")
	if o := bob.take(); !strings.Contains(o, "You have 30 quest points") {
		t.Fatalf("points: %q", o)
	}
}

func TestQuestPersistsAndReplants(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w1, st := testWorldWithStore(t, playerDir)
	makeHerald(w1)
	addQuestItem(w1)
	bob := login(t, w1, 1, "Bob")
	p := w1.players[1]
	p.questPoints = 7
	requestUntil(t, w1, bob, p, "fetch")
	room, rounds := p.quest.Room, p.quest.Rounds
	send(w1, 1, "quit")
	bob.take()
	rec, err := st.Load("Bob")
	if err != nil || rec.QuestPoints != 7 || rec.Quest == nil || rec.Quest.Kind != "fetch" || rec.Quest.Room != room {
		t.Fatalf("saved quest wrong: %+v err=%v", rec, err)
	}

	// A fresh world has no planted item; login plants it again.
	w2, _ := testWorldWithStore(t, playerDir)
	makeHerald(w2)
	addQuestItem(w2)
	c := signin(t, w2, 1, "Bob", "secret5")
	p2 := w2.players[1]
	if p2.questPoints != 7 || p2.quest == nil || p2.quest.Rounds > rounds || !w2.questItemPlanted(p2) {
		t.Fatalf("restored: points=%d quest=%+v planted=%v", p2.questPoints, p2.quest, w2.questItemPlanted(p2))
	}
	send(w2, 1, "quest info")
	if o := c.take(); !strings.Contains(o, "recover a bundle of dispatches") {
		t.Fatalf("info after login: %q", o)
	}

	// A held item is saved with its stamp and not planted twice.
	if p2.quest.Room == 2 {
		send(w2, 1, "north")
		c.take()
	}
	send(w2, 1, "get bundle")
	c.take()
	send(w2, 1, "quit")
	rec, _ = st.Load("Bob")
	if len(rec.Inventory) != 1 || rec.Inventory[0].Quest != "Bob" {
		t.Fatalf("held quest item not saved with stamp: %+v", rec.Inventory)
	}
	w3, _ := testWorldWithStore(t, playerDir)
	makeHerald(w3)
	addQuestItem(w3)
	signin(t, w3, 1, "Bob", "secret5")
	p3 := w3.players[1]
	if p3.questItemHeld() == nil || w3.questItemPlanted(p3) {
		t.Fatalf("held=%v planted=%v", p3.questItemHeld(), w3.questItemPlanted(p3))
	}
}

func TestQuestRulesHooks(t *testing.T) {
	w := testWorld(t)
	makeHerald(w)
	setRules(t, w, testRules+`
function questRules(c) { return { levelBand: 10, rounds: 99, cooldown: 3, quitCooldown: 4 }; }
function questReward(c, q) { return { points: 40 + q.level, silver: 12, xp: 150, message: "The herald nods." }; }
`)
	bob := login(t, w, 1, "Bob")
	p := w.players[1]
	send(w, 1, "quest request")
	o := bob.take()
	if !strings.Contains(o, "41 quest points and 12 silver and 150 experience") {
		t.Fatalf("request with hooks: %q", o)
	}
	if p.quest.Rounds != 99 {
		t.Fatalf("rounds: %d", p.quest.Rounds)
	}
	p.quest.Done = true
	send(w, 1, "quest complete")
	o = bob.take()
	if !strings.Contains(o, "The herald nods.") || !strings.Contains(o, "Level up.") {
		t.Fatalf("complete with hooks: %q", o)
	}
	if p.questPoints != 41 || p.Silver != 12 || p.Level != 2 || p.questWait != 3 {
		t.Fatalf("paid: points=%d silver=%d level=%d wait=%d", p.questPoints, p.Silver, p.Level, p.questWait)
	}
}
