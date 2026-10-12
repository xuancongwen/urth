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
	1300, 1322, 1324, 1325, // diablo: Deckard Cain, Farnham, Gharbad the Weak, Zhar the Mad
	3105,                    // rogue: Flavie
	3203,                    // lutgholein: Geglash
	3504,                    // harrogath: Nihlathak the elder
	832, 833, 842, 843, 847, // drevlin: the High Froman, Limbeck, Hugh the Hand, Bane, Haplo's dog
	600, 603, 611, 612, 613, 621, 622, 623, // icewind: Cassius, Regis, Drizzt, Bruenor, Catti-brie, Kemp, Heafstaag, Akar Kessell
	1200, 1203, 1216, 1224, 1225, 1227, // narnia: Mr Tumnus, Mr Beaver, Aslan, Susan, Rumblebuffin, Caspian
	400, 401, 402, 404, 419, 420, 421, // shire: the Gaffer, Lobelia, Ted Sandyman, Farmer Maggot, Sam, Pippin, Merry
	2020, 2024, 2025, 2026, // moria: Gandalf, Boromir, Pippin, Gollum
	300, 301, 303, 305, 307, 313, 314, 320, 324, 325, 326, // solace: Otik, Tika, Raistlin, Hederick, Toede, Verminaard, Kitiara, Tanis, Tasslehoff, Goldmoon, Riverwind
	500, 501, 502, 503, 509, 510, 523, 524, 526, 527, // swordcoast: Winthrop, Imoen, Tethtoril, Gorion, Khalid, Jaheira, Edwin, Viconia, Sarevok, Tazok
	709, 715, 716, 717, 719, // underdark: Belwar, Matron Malice, Dinin, Zaknafein, King Schnicktick
	1504, 1505, 1520, 1521, 1524, 1525, 1526, 1527, 1528, 1529, 1530, // winterfell: Eddard, Old Nan, Catelyn, Robb, Sansa, Arya, Bran, Hodor, Rickon, Theon, the Night King
	1022, 1023, 1024, 1027, // avalon: the Lady of the Lake, Mordred, the Green Knight, Guinevere
	1403, 1414, 1425, 1426, // warcraft: the Warchief, the Lord-Marshal, King Llane, Medivh
	1125, 1126, // draenor: Thrall, Durotan
	1603, 1621, 1623, // starcraft: Magistrate Tarran, Mengsk, Tassadar
	2222, 2223, 2224, 2227, // velen: Ciri, Triss, Dandelion, Eredin
	2119, 2123, // barovia: Ireena, Baba Lysaga
	2421, 2422, 2423, 2425, // nublar: Malcolm, Lex, Tim, Gennaro
	1720, 1722, // klendathu: Dizzy Flores, Zander Barcalow
	1925, 1926, 1927, 1928, // arrakis: the Baron, Rabban, Piter de Vries, Thufir Hawat
	2304, 2324, 2325, 2326, // deathstar: C-3PO, Boba Fett, Motti, the Emperor
	1824,       // cadia: Haarken Worldclaimer
	3030, 3031, // midgaard: Odif Yltsaeb, Puff
	3610, 3611, 3614, 3617, 3621, 3624, 3625, 3627, 3630, 3634, 3635, // foundation: Lefkin, Wienis, Argo, Bel Riose, Seldon, Demerzel, Ebling Mis, Bayta, Arkady, Han Pritcher, the Mule
	3706, 3708, 3710, 3711, 3713, 3714, 3715, 3716, 3720, 3723, 3725, 3726, 3728, 3730, 3731, 3739, // ff7: Corneo, Reno, Red XIII, Hojo, President Shinra, Heidegger, Scarlet, Rufus, Elena, Gi Nattak, Vincent, Palmer, Rude, Yuffie, Tseng, Sephiroth
	3814, 3815, 3817, 3821, 3822, 3823, 3825, 3826, 3827, 3828, 3829, 3832, 3833, 3834, 3835, 3836, 3839, 3840, // xenogears: Bart, Vanderkaum, Rico, Stier, Seibzehn, Chu-Chu, the four Elements, Ramsus, Miang, Cain, Grahf, Weltall-Id, Id, Krelian, Deus
	3902, 3910, 3911, 3921, 3930, 3943, 3948, // zelda: Mido, Talon, Ingo, Darunia, Ruto, Sheik, Nabooru
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
	noAuto(p)
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

// TestShippedItemColors checks the shipped rules color the shipped items:
// every named item in the named color, every material in its rarity's,
// and plain gear not at all until it rolls.
func TestShippedItemColors(t *testing.T) {
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
	w := New(config.Default(), rw, slog.New(slog.NewTextHandler(io.Discard, nil)), Deps{Scripts: engine})
	want := map[string]string{"common": "", "uncommon": "G", "rare": "B", "deterministic": "C"}
	for _, p := range w.content.Items {
		var expect string
		switch {
		case len(p.Effects) > 0:
			expect = "C"
		case p.Material != "":
			expect = want[p.Rarity]
		}
		if p.Resolved.Color != expect {
			t.Errorf("%s [%d] is colored %q, want %q", p.Name, p.Vnum, p.Resolved.Color, expect)
		}
	}
}
