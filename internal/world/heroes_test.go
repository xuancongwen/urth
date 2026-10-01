package world

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"urth/internal/config"
	"urth/internal/content"
	"urth/internal/item"
	"urth/internal/script"
	"urth/internal/session"
	"urth/internal/store"
)

// famousCharacters are the named figures of the shipped areas who can be
// fought and killed for what they carry. Those who trade, teach, or hand
// out quests stay peaceful, and so does Fizban.
var famousCharacters = []int{
	1300,               // diablo: Deckard Cain
	832, 833, 842, 843, // drevlin: the High Froman, Limbeck, Hugh the Hand, Bane
	600, 603, 611, 612, 613, // icewind: Cassius, Regis, Drizzt, Bruenor, Catti-brie
	1200, 1203, 1216, // narnia: Mr Tumnus, Mr Beaver, Aslan
	400, 401, 402, 404, // shire: the Gaffer, Lobelia, Ted Sandyman, Farmer Maggot
	300, 301, 303, 320, 324, 325, 326, // solace: Otik, Tika, Raistlin, Tanis, Tasslehoff, Goldmoon, Riverwind
	500, 501, 502, 503, 509, 510, // swordcoast: Winthrop, Imoen, Tethtoril, Gorion, Khalid, Jaheira
	709,        // underdark: Belwar Dissengulp
	1504, 1505, // winterfell: the Lord of Winterfell, Old Nan
	1403, 1414, // warcraft: the Warchief, the Lord-Marshal
	1603, // starcraft: Magistrate Tarran
}

// TestFamousCharactersCanBeKilledForTheirLoot holds the shipped world to
// it: each one is placed, may be attacked where it stands, and leaves a
// corpse with something in it.
func TestFamousCharactersCanBeKilledForTheirLoot(t *testing.T) {
	w := shippedWorld(t)
	for i := 0; i < 5; i++ {
		w.Tick()
	}
	live := map[int]*Mob{}
	for _, c := range w.rooms {
		for _, m := range c.mobs {
			live[m.Proto.Vnum] = m
		}
	}
	for _, vnum := range famousCharacters {
		m := live[vnum]
		if m == nil {
			t.Errorf("mob %d is not in the world", vnum)
			continue
		}
		p := m.Proto
		if p.HasFlag("peaceful") {
			t.Errorf("%s (%d) is peaceful", p.Name, vnum)
		}
		if m.Room.Safe() {
			t.Errorf("%s (%d) stands in safe room %d (%s), where nobody can fight", p.Name, vnum, m.Room.Vnum, m.Room.Name)
		}
		if len(p.Trades) > 0 || len(p.Sells) > 0 || len(p.Teaches) > 0 || isQuestmaster(m) {
			t.Errorf("%s (%d) trades, teaches, or gives quests, and should stay peaceful", p.Name, vnum)
		}
		if w.countMobs(p) != 1 {
			t.Errorf("%s (%d): %d in the world, want one", p.Name, vnum, w.countMobs(p))
		}
		room := m.Room
		w.die(m.Character, nil)
		var corpse *item.Item
		for _, it := range w.contents(room).items {
			if it.Proto.HasFlag("corpse") && it.Proto.Name == "the corpse of "+p.Name {
				corpse = it
			}
		}
		if corpse == nil {
			t.Errorf("%s (%d) left no corpse", p.Name, vnum)
			continue
		}
		loot := 0
		for _, it := range corpse.Contents {
			if !it.Proto.HasFlag("coins") {
				loot++
			}
		}
		if loot == 0 && p.Silver == 0 {
			t.Errorf("%s (%d) died with nothing to take", p.Name, vnum)
		}
	}
}

// TestKillTikaForHerFryingPan walks the whole path in the shipped world:
// a player in the Inn of the Last Home attacks Tika, she fights back with
// the pan, and the pan is in her corpse.
func TestKillTikaForHerFryingPan(t *testing.T) {
	if _, err := os.Stat("../../data/world"); err != nil {
		t.Skip("no shipped world")
	}
	rw, err := content.Load("../../data/world")
	if err != nil {
		t.Fatal(err)
	}
	engine := script.New("../../data/scripts", slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(t.TempDir(), "players"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Timing.TickMs, cfg.Timing.RoundMs = 100, 1000
	w := New(cfg, rw, slog.New(slog.NewTextHandler(io.Discard, nil)), Deps{Store: st, Scripts: engine})

	c := connect(t, w, 1)
	send(w, 1, "Bob")
	send(w, 1, "y")
	send(w, 1, "secret5")
	w.Events() <- session.Input{ID: 1, Line: "secret5"}
	tickUntil(t, w, c, "Welcome, Bob")
	p := w.players[1]
	p.Room = w.content.Rooms.Rooms[312]
	p.Level = 20
	w.recalc(p.Character)
	p.Health = p.HealthMax

	send(w, 1, "kill tika")
	if o := c.take(); strings.Contains(o, "can't bring yourself") || strings.Contains(o, "cannot fight here") {
		t.Fatalf("could not attack Tika: %q", o)
	}
	tika := p.Fighting
	if tika == nil || tika.Name != "Tika" {
		t.Fatalf("not fighting Tika: %+v", tika)
	}
	out := ""
	for i := 0; i < 3000 && tika.Health > 0; i++ {
		w.Tick()
		out += c.take()
	}
	if !strings.Contains(out, "Tika's clang") {
		t.Errorf("Tika never swung her frying pan: %q", out)
	}
	if !strings.Contains(out, "Tika is DEAD!!") {
		t.Fatalf("Tika did not die: %q", out)
	}
	send(w, 1, "get pan corpse")
	if o := c.take(); !strings.Contains(o, "Tika's frying pan") {
		t.Fatalf("no frying pan in the corpse: %q", o)
	}
}
