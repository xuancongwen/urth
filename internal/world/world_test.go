package world

import (
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"errors"

	"golang.org/x/crypto/bcrypt"

	"urth/internal/config"
	"urth/internal/content"
	"urth/internal/copyover"
	"urth/internal/output"
	"urth/internal/script"
	"urth/internal/session"
	"urth/internal/store"
)

// testRules is the placeholder rule set used by world tests. Damage is
// weapon damage or 1; every swing hits; pools are small so fights end fast.
const testRules = `
function resolveAttack(a, d, w, round) {
  return { hit: true, damage: w && w.weapon ? w.weapon.damage : 1, crit: false, verb: w && w.weapon ? w.weapon.verb : "punch", stage: "" };
}
function derivedStats(c) { return { healthMax: 10 + c.level * 2 + c.effects.length * 5, manaMax: 5, speed: 1 }; }
function onTick(c) { return { healthDelta: c.fighting ? 0 : 1, manaDelta: 0 }; }
function xpForKill(k, v) { return 60; }
function xpToLevel(level) { return (level - 1) * 100; }
function onLevel(c, l) { return { statPoints: 1, statDeltas: { might: 1 }, message: "Level up." }; }
function onCreate(c) { return { stats: { might: 0 }, statPoints: 0 }; }
function deathRules() { return { xpFraction: 0, xpLevelCap: 0, corpseRounds: 3, respawnHealth: 1 }; }
`

// fakeConn is an in-memory session.Conn that records output rendered as
// plain text, plus the raw batches for structural assertions.
type fakeConn struct {
	id      session.ID
	out     strings.Builder
	batches []output.Batch
	closed  bool
}

func (f *fakeConn) ID() session.ID     { return f.id }
func (f *fakeConn) RemoteAddr() string { return "test" }
func (f *fakeConn) Close()             { f.closed = true }
func (f *fakeConn) Send(b output.Batch) {
	f.batches = append(f.batches, b)
	b.Color = false
	f.out.WriteString(output.RenderText(b))
}

// take returns and clears everything sent so far.
func (f *fakeConn) take() string {
	s := f.out.String()
	f.out.Reset()
	return s
}

func testWorld(t *testing.T) *World {
	t.Helper()
	w, _ := testWorldWithStore(t, "")
	return w
}

// testWorldWithStore builds a world; playerDir lets tests share a store
// across two worlds to check persistence.
func testWorldWithStore(t *testing.T, playerDir string) (*World, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	if playerDir == "" {
		playerDir = filepath.Join(dir, "players")
	}
	files := map[string]string{
		"a/rooms/1.yaml":  "vnum: 1\nname: Hub\ndescription: The hub.\nexits:\n  north: 2\n",
		"a/rooms/2.yaml":  "vnum: 2\nname: North\ndescription: Up north.\nexits:\n  south: 1\n  east: 3\n",
		"b/rooms/3.yaml":  "vnum: 3\nname: Elsewhere\ndescription: Another area.\nexits:\n  west: 2\n  east: 30\nflags: [safe]\ntemple: good\ndoors:\n  west: {name: the iron gate}\n",
		"b/rooms/30.yaml": "vnum: 30\nname: Three Out\ndescription: Three rooms out.\nexits:\n  west: 3\n  east: 31\n",
		"b/rooms/31.yaml": "vnum: 31\nname: Four Out\ndescription: Four rooms out.\nexits:\n  west: 30\n  east: 32\n",
		"b/rooms/32.yaml": "vnum: 32\nname: Five Out\ndescription: Five rooms out.\nexits:\n  west: 31\n",
		"a/items/16.yaml": "vnum: 16\nname: an ember totem\nkeywords: [ember, totem]\ntype: totem\nschool: evocation\n",
		"a/items/17.yaml": "vnum: 17\nname: a handful of ash\nkeywords: [handful, ash]\ntype: material\nmaterial: ash\n",
		"a/items/18.yaml": "vnum: 18\nname: a sun cup\nkeywords: [sun, cup]\nsacrifice: good\n",
		"a/items/10.yaml": "vnum: 10\nname: a rusty sword\nkeywords: [rusty, sword]\ndescription: A rusty sword lies here.\nlook: Pitted and dull.\ntype: weapon\nweapon:\n  damage: 4\n  hands: 1\n  verb: slash\n",
		"a/items/15.yaml": "vnum: 15\nname: a plain spear\nkeywords: [plain, spear]\ntype: weapon\nlevel: 3\nbaseline: standard\nweapon:\n  hands: 2\n",
		"a/items/11.yaml": "vnum: 11\nname: a leather cap\nkeywords: [leather, cap]\ntype: armor\nslot: head\narmor:\n  defense: 1\n",
		"a/items/12.yaml": "vnum: 12\nname: a small sack\nkeywords: [small, sack]\ntype: container\n",
		"a/items/13.yaml": "vnum: 13\nname: a loaf of bread\nkeywords: [loaf, bread]\n",
		"a/items/14.yaml": "vnum: 14\nname: a stone altar\nkeywords: [altar]\ndescription: A stone altar stands here.\nflags: [nopickup]\n",
		"a/mobs/20.yaml":  "vnum: 20\nname: a city guard\nkeywords: [city, guard]\ndescription: A city guard stands here.\nlook: Tall and bored.\nflags: [sentinel]\n",
		"a/mobs/21.yaml":  "vnum: 21\nname: a stray dog\nkeywords: [stray, dog]\ndescription: A stray dog sniffs about.\n",
		"a/resets.yaml": "interval_seconds: 5\nresets:\n" +
			"  - mob: 20\n    room: 1\n    equip:\n      - item: 10\n        slot: wield\n      - item: 13\n" +
			"  - mob: 21\n    room: 2\n    max: 2\n" +
			"  - item: 12\n    room: 1\n" +
			"  - item: 13\n    into: 12\n    room: 1\n" +
			"  - item: 11\n    room: 1\n" +
			"  - item: 14\n    room: 1\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rw, err := content.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	scriptDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "rules.js"), []byte(testRules), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := script.New(scriptDir, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Timing.TickMs = 100
	cfg.Timing.RoundMs = 1000
	cfg.Timing.AutosaveSeconds = 10
	st, err := store.New(playerDir, bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	w := New(cfg, rw, slog.New(slog.NewTextHandler(io.Discard, nil)), Deps{Store: st, Scripts: engine,
		LoadContent: func() (*content.World, error) { return content.Load(dir) }})
	w.scriptDir = scriptDir
	w.contentDir = dir
	// A fixed seed keeps mob wandering and flee directions the same run to
	// run; the world seeds from the clock in production.
	w.rng = rand.New(rand.NewPCG(7, 11))
	return w, st
}

// tickUntil ticks the world until the connection's output contains want or
// the deadline passes. Login involves off-goroutine hashing, so tests must
// wait for the posted result.
func tickUntil(t *testing.T, w *World, c *fakeConn, want string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w.Tick()
		if strings.Contains(c.out.String(), want) {
			return c.take()
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("never saw %q; output so far: %q", want, c.out.String())
	return ""
}

// connect attaches a fake client and returns it after the greeting.
func connect(t *testing.T, w *World, id session.ID) *fakeConn {
	t.Helper()
	c := &fakeConn{id: id}
	w.Events() <- session.Connected{Conn: c}
	w.Tick()
	if !strings.Contains(c.take(), "By what name") {
		t.Fatal("no greeting")
	}
	return c
}

// create walks a new character through the creation prompts.
func create(t *testing.T, w *World, id session.ID, name, password string) *fakeConn {
	t.Helper()
	c := connect(t, w, id)
	send(w, id, name)
	if out := c.take(); !strings.Contains(out, "Did I get that right, "+name) {
		t.Fatalf("no confirm prompt: %q", out)
	}
	send(w, id, "y")
	if out := c.take(); !strings.Contains(out, "Give me a password") {
		t.Fatalf("no password prompt: %q", out)
	}
	send(w, id, password)
	if out := c.take(); !strings.Contains(out, "retype") {
		t.Fatalf("no retype prompt: %q", out)
	}
	w.Events() <- session.Input{ID: id, Line: password}
	if out := tickUntil(t, w, c, "Welcome, "+name); !strings.Contains(out, "Hub") {
		t.Fatalf("creation output wrong: %q", out)
	}
	return c
}

// signin logs an existing character in.
func signin(t *testing.T, w *World, id session.ID, name, password string) *fakeConn {
	t.Helper()
	c := connect(t, w, id)
	send(w, id, name)
	if out := c.take(); !strings.Contains(out, "Password: ") {
		t.Fatalf("no password prompt: %q", out)
	}
	w.Events() <- session.Input{ID: id, Line: password}
	tickUntil(t, w, c, "tnl> ") // the in-game prompt: covers both login and reconnect
	return c
}

// login creates a fresh character named name with a fixed password.
func login(t *testing.T, w *World, id session.ID, name string) *fakeConn {
	t.Helper()
	return create(t, w, id, name, "secret5")
}

func send(w *World, id session.ID, line string) {
	w.Events() <- session.Input{ID: id, Line: line}
	w.Tick()
}

func TestLoginRejectsBadNames(t *testing.T) {
	w := testWorld(t)
	c := connect(t, w, 1)
	for _, bad := range []string{"ab", "toolongofaname", "b0b", "bob smith"} {
		send(w, 1, bad)
		if out := c.take(); !strings.Contains(out, "3 to 12 letters") {
			t.Fatalf("%q accepted: %q", bad, out)
		}
	}
	send(w, 1, "bob")
	if out := c.take(); !strings.Contains(out, "Did I get that right, Bob") {
		t.Fatalf("capitalised name not offered: %q", out)
	}
}

func TestCreationPromptsAndEcho(t *testing.T) {
	w := testWorld(t)
	c := connect(t, w, 1)
	send(w, 1, "Bob")
	send(w, 1, "maybe")
	if out := c.take(); !strings.Contains(out, "Yes or No") {
		t.Fatalf("bad confirm answer accepted: %q", out)
	}
	send(w, 1, "n")
	if out := c.take(); !strings.Contains(out, "what IS it") {
		t.Fatalf("no re-ask: %q", out)
	}
	send(w, 1, "Bob")
	send(w, 1, "y")
	c.take()
	if last := c.batches[len(c.batches)-1]; last.Messages[0].Type != output.EchoOff {
		t.Fatalf("echo not turned off for password: %+v", last.Messages)
	}
	send(w, 1, "abc")
	if out := c.take(); !strings.Contains(out, "5 to 64") {
		t.Fatalf("short password accepted: %q", out)
	}
	send(w, 1, "secret5")
	send(w, 1, "different")
	if out := c.take(); !strings.Contains(out, "don't match") {
		t.Fatalf("mismatch accepted: %q", out)
	}
	send(w, 1, "secret5")
	w.Events() <- session.Input{ID: 1, Line: "secret5"}
	out := tickUntil(t, w, c, "Welcome, Bob")
	if !strings.Contains(out, "made an admin") {
		t.Fatalf("first player should be admin: %q", out)
	}
	echoOn := false
	for _, b := range c.batches {
		for _, m := range b.Messages {
			if m.Type == output.EchoOn {
				echoOn = true
			}
		}
	}
	if !echoOn {
		t.Fatal("echo never restored")
	}
	if !w.players[1].Admin {
		t.Fatal("player not flagged admin")
	}
}

func TestPasswordCheckAndLockout(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "quit")
	bob.take()

	c := connect(t, w, 2)
	send(w, 2, "bob")
	c.take()
	for i := 1; i <= 2; i++ {
		w.Events() <- session.Input{ID: 2, Line: "wrong"}
		if out := tickUntil(t, w, c, "Wrong password."); !strings.Contains(out, "Password: ") {
			t.Fatalf("try %d: no re-prompt: %q", i, out)
		}
	}
	w.Events() <- session.Input{ID: 2, Line: "wrong"}
	tickUntil(t, w, c, "Goodbye")
	if !c.closed {
		t.Fatal("not disconnected after three failures")
	}

	d := signin(t, w, 3, "bob", "secret5")
	if d.closed || w.players[3].State != StatePlaying {
		t.Fatal("correct password did not log in")
	}
	if w.players[3].Admin != true {
		t.Fatal("admin flag not loaded from record")
	}
	// The second character created is not an admin.
	e := login(t, w, 4, "Alice")
	if w.players[4].Admin {
		t.Fatal("second player should not be admin")
	}
	_ = e
}

func TestPersistenceAcrossWorlds(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w1, _ := testWorldWithStore(t, playerDir)
	bob := login(t, w1, 1, "Bob")
	send(w1, 1, "north")
	send(w1, 1, "color")
	send(w1, 1, "quit")
	bob.take()

	w2, st := testWorldWithStore(t, playerDir)
	rec, err := st.Load("Bob")
	if err != nil || rec.Room != 2 || rec.Color {
		t.Fatalf("saved record wrong: %+v err=%v", rec, err)
	}
	c := signin(t, w2, 1, "Bob", "secret5")
	if p := w2.players[1]; p.Room.Vnum != 2 || p.Color {
		t.Fatalf("restored state wrong: room=%d color=%v", p.Room.Vnum, p.Color)
	}
	_ = c
}

func TestReconnectTakesOverSession(t *testing.T) {
	w := testWorld(t)
	first := login(t, w, 1, "Bob")
	send(w, 1, "north")
	first.take()

	second := signin(t, w, 2, "Bob", "secret5")
	if !first.closed {
		t.Fatal("old session not closed")
	}
	if out := first.out.String(); !strings.Contains(out, "reconnected from elsewhere") {
		t.Fatalf("old session not told: %q", out)
	}
	if _, still := w.players[1]; still {
		t.Fatal("old player not removed")
	}
	p := w.players[2]
	if p.Room.Vnum != 2 || p.Name != "Bob" {
		t.Fatalf("takeover lost state: room=%d name=%s", p.Room.Vnum, p.Name)
	}
	var all strings.Builder
	for _, b := range second.batches {
		b.Color = false
		all.WriteString(output.RenderText(b))
	}
	if out := all.String(); !strings.Contains(out, "Reconnecting") || strings.Contains(out, "has left the game") {
		t.Fatalf("new session output wrong: %q", out)
	}
}

func TestChangePassword(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	send(w, 1, "password")
	if out := bob.take(); !strings.Contains(out, "Syntax") {
		t.Fatalf("no syntax help: %q", out)
	}
	w.Events() <- session.Input{ID: 1, Line: "password wrong newpass1"}
	tickUntil(t, w, bob, "Nothing changed")
	w.Events() <- session.Input{ID: 1, Line: "password secret5 newpass1"}
	tickUntil(t, w, bob, "Password changed")
	send(w, 1, "quit")
	signin(t, w, 2, "Bob", "newpass1")
}

func TestAdminCommandsHiddenFromPlayers(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	alice.take()
	send(w, 2, "copyover")
	if out := alice.take(); !strings.Contains(out, "Huh?") {
		t.Fatalf("non-admin ran copyover: %q", out)
	}
	if c := lookup("shutdown", false); c != nil {
		t.Fatal("shutdown visible to non-admin")
	}
	if c := lookup("shutdown", true); c == nil {
		t.Fatal("shutdown hidden from admin")
	}
}

func TestCopyoverStateAndTokenRestore(t *testing.T) {
	playerDir := filepath.Join(t.TempDir(), "players")
	w, _ := testWorldWithStore(t, playerDir)
	var captured copyover.State
	w.deps.Copyover = func(st copyover.State) error {
		captured = st
		return errors.New("exec refused in test")
	}
	w.deps.Listeners = func() []copyover.Listener { return []copyover.Listener{{Kind: "telnet", FD: 3}} }
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	send(w, 2, "north")
	pending := connect(t, w, 3) // still at the name prompt
	bob.take()
	alice.take()

	send(w, 1, "copyover")
	if len(captured.Players) != 2 || captured.Listeners[0].FD != 3 {
		t.Fatalf("state wrong: %+v", captured)
	}
	var aliceTok string
	for _, cp := range captured.Players {
		if cp.Kind != "web" || cp.Token == "" {
			t.Fatalf("fake conns cannot hand over sockets; expected web token entries: %+v", cp)
		}
		if cp.Name == "Alice" {
			aliceTok = cp.Token
			if cp.Room != 2 {
				t.Fatalf("alice room not captured: %+v", cp)
			}
		}
	}
	if !pending.closed {
		t.Fatal("logging-in session should be closed on copyover")
	}
	if out := bob.take(); !strings.Contains(out, "Copyover failed") {
		t.Fatalf("failure not reported: %q", out)
	}
	tokenSeen := false
	for _, b := range alice.batches {
		for _, m := range b.Messages {
			if m.Type == output.Reconnect {
				if d, ok := m.Data.(output.ReconnectData); ok && d.Token == aliceTok {
					tokenSeen = true
				}
			}
		}
	}
	if !tokenSeen {
		t.Fatal("alice never received her reconnect token")
	}

	// Simulate the new process: tokens registered, alice reconnects with hers.
	w2, _ := testWorldWithStore(t, playerDir)
	w2.RegisterTokens(captured.Players)
	c := &fakeConn{id: 9}
	w2.Events() <- session.Connected{Conn: c, Token: aliceTok}
	w2.Tick()
	if out := c.take(); !strings.Contains(out, "Copyover complete") {
		t.Fatalf("token restore failed: %q", out)
	}
	if p := w2.players[9]; p.State != StatePlaying || p.Name != "Alice" || p.Room.Vnum != 2 {
		t.Fatalf("restored player wrong: %+v", p)
	}
	// A token is single use; a bad token gets the login prompt.
	d := &fakeConn{id: 10}
	w2.Events() <- session.Connected{Conn: d, Token: aliceTok}
	w2.Tick()
	if out := d.take(); !strings.Contains(out, "By what name") {
		t.Fatalf("reused token accepted: %q", out)
	}
	// Inherited-socket restore path.
	e := &fakeConn{id: 11}
	w2.Events() <- session.Connected{Conn: e, Restore: &Restore{Name: "Bob", Room: 1}}
	w2.Tick()
	if out := e.take(); !strings.Contains(out, "Copyover complete") || w2.players[11].Name != "Bob" {
		t.Fatalf("restore event failed: %q", out)
	}
}

func TestAutosave(t *testing.T) {
	w, st := testWorldWithStore(t, "")
	login(t, w, 1, "Bob")
	send(w, 1, "north")
	// autosave every 10 rounds of 1s at 100ms ticks = 100 ticks
	for i := 0; i < 100; i++ {
		w.Tick()
	}
	rec, err := st.Load("Bob")
	if err != nil || rec.Room != 2 {
		t.Fatalf("autosave did not persist room: %+v %v", rec, err)
	}
}

func TestWalkAndSee(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	if out := bob.take(); !strings.Contains(out, "Alice has entered the game") {
		t.Fatalf("bob not told of alice: %q", out)
	}

	send(w, 2, "l")
	if out := alice.take(); !strings.Contains(out, "Bob is here") {
		t.Fatalf("alice does not see bob: %q", out)
	}

	send(w, 1, "n")
	if out := bob.take(); !strings.Contains(out, "North\nUp north.\n[Exits: east south]") {
		t.Fatalf("bob move output wrong: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "Bob leaves north") {
		t.Fatalf("alice not told bob left: %q", out)
	}

	send(w, 2, "north")
	if out := bob.take(); !strings.Contains(out, "Alice arrives from the south") {
		t.Fatalf("bob not told alice arrived: %q", out)
	}
	alice.take()

	send(w, 1, "n")
	if out := bob.take(); !strings.Contains(out, "cannot go that way") {
		t.Fatalf("bad exit not refused: %q", out)
	}
}

func TestDoorsHideExitsAndScan(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	carol := login(t, w, 3, "Carol")
	send(w, 3, "n")
	send(w, 3, "e")
	send(w, 2, "n")
	bob.take()
	alice.take()
	carol.take()

	// Bob in the hub sees Alice and the dogs to the north.
	send(w, 1, "scan")
	if out := bob.take(); !strings.Contains(out, "North - North:\n    Alice\n    a stray dog\n") {
		t.Fatalf("scan wrong: %q", out)
	}
	// Alice, up north, sees Carol through the open gate and the gate itself.
	send(w, 2, "sca")
	if out := alice.take(); !strings.Contains(out, "East - Elsewhere:\n    Carol\n") || !strings.Contains(out, "South - Hub:\n    Bob\n") {
		t.Fatalf("scan through open door wrong: %q", out)
	}
	send(w, 2, "exits")
	if out := alice.take(); !strings.Contains(out, "East  - Elsewhere") {
		t.Fatalf("open door hidden from exits: %q", out)
	}
	send(w, 2, "look gate")
	if out := alice.take(); !strings.Contains(out, "The iron gate is open.") {
		t.Fatalf("look at open door: %q", out)
	}

	// Closing the gate hides the exit, and Carol, from both sides.
	send(w, 2, "close gate")
	if out := alice.take(); !strings.Contains(out, "You close the iron gate.") {
		t.Fatalf("close: %q", out)
	}
	if out := carol.take(); !strings.Contains(out, "The iron gate closes.") {
		t.Fatalf("far side not told: %q", out)
	}
	if out := bob.take(); out != "" {
		t.Fatalf("bob heard a door two rooms off: %q", out)
	}
	send(w, 2, "scan")
	if out := alice.take(); strings.Contains(out, "Carol") || !strings.Contains(out, "South - Hub:") {
		t.Fatalf("scan through closed door: %q", out)
	}
	send(w, 2, "exits")
	if out := alice.take(); strings.Contains(out, "East") || !strings.Contains(out, "South - Hub") {
		t.Fatalf("closed door listed: %q", out)
	}
	send(w, 2, "look")
	if out := alice.take(); !strings.Contains(out, "[Exits: south]") {
		t.Fatalf("closed door in look: %q", out)
	}
	send(w, 2, "east")
	if out := alice.take(); !strings.Contains(out, "The iron gate is closed.") || alice != w.players[2].conn {
		t.Fatalf("walked through a closed door: %q", out)
	}
	if w.players[2].Room.Vnum != 2 {
		t.Fatal("alice moved through a closed door")
	}
	send(w, 3, "exits")
	if out := carol.take(); !strings.Contains(out, "Obvious exits:\nEast  - Three Out\n") || strings.Contains(out, "West") {
		t.Fatalf("far side exits: %q", out)
	}
	send(w, 3, "scan")
	if out := carol.take(); strings.Contains(out, "Alice") || !strings.Contains(out, "You see no one nearby.") {
		t.Fatalf("far side scan: %q", out)
	}
	send(w, 3, "look west")
	if out := carol.take(); !strings.Contains(out, "The iron gate is closed.") {
		t.Fatalf("look at closed door: %q", out)
	}
	send(w, 3, "close w")
	if out := carol.take(); !strings.Contains(out, "It's already closed.") {
		t.Fatalf("double close: %q", out)
	}
	send(w, 3, "open north")
	if out := carol.take(); !strings.Contains(out, "There is no door there.") {
		t.Fatalf("open nothing: %q", out)
	}

	// Carol opens it from her side and Alice sees her again.
	send(w, 3, "open w")
	if out := carol.take(); !strings.Contains(out, "You open the iron gate.") {
		t.Fatalf("open: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "The iron gate opens.") {
		t.Fatalf("near side not told of opening: %q", out)
	}
	send(w, 2, "scan")
	if out := alice.take(); !strings.Contains(out, "East - Elsewhere:\n    Carol\n") {
		t.Fatalf("scan after reopening: %q", out)
	}
	send(w, 2, "e")
	if w.players[2].Room.Vnum != 3 {
		t.Fatal("alice could not walk through the open gate")
	}
	alice.take()

	// An area reset puts the gate back the way the file has it: open.
	send(w, 2, "close gate")
	alice.take()
	w.resetArea(w.areas["a"])
	send(w, 2, "exits")
	if out := alice.take(); !strings.Contains(out, "West  - North") {
		t.Fatalf("reset did not reopen the gate: %q", out)
	}
}

func TestSayReachesRoomOnly(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	carol := login(t, w, 3, "Carol")
	send(w, 3, "n")
	bob.take()
	alice.take()
	carol.take()

	send(w, 1, "'hello there")
	if out := bob.take(); !strings.Contains(out, "You say 'hello there'") {
		t.Fatalf("speaker echo wrong: %q", out)
	}
	if out := alice.take(); !strings.Contains(out, "Bob says 'hello there'") {
		t.Fatalf("listener wrong: %q", out)
	}
	if out := carol.take(); out != "" {
		t.Fatalf("carol heard through walls: %q", out)
	}
}

func TestWhoExitsQuitHuh(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	login(t, w, 2, "Alice")
	bob.take()

	send(w, 1, "wh")
	if out := bob.take(); !strings.Contains(out, "[  1] Alice\n[  1] Bob\n") || !strings.Contains(out, "2 players online") {
		t.Fatalf("who wrong: %q", out)
	}
	send(w, 1, "ex")
	if out := bob.take(); !strings.Contains(out, "North - North") {
		t.Fatalf("exits wrong: %q", out)
	}
	send(w, 1, "frobnicate")
	if out := bob.take(); !strings.Contains(out, "Huh?") {
		t.Fatalf("unknown command: %q", out)
	}
	// "q" must not quit; "qui" must.
	send(w, 1, "q")
	if out := bob.take(); !strings.Contains(out, "Huh?") || bob.closed {
		t.Fatalf("single q should not quit: %q closed=%v", out, bob.closed)
	}
	send(w, 1, "qui")
	if out := bob.take(); !strings.Contains(out, "all good things") || !bob.closed {
		t.Fatalf("quit failed: %q closed=%v", out, bob.closed)
	}
	if _, still := w.players[1]; still {
		t.Fatal("player not removed after quit")
	}
	// The transport's Disconnected for an already-removed player is harmless.
	w.Events() <- session.Disconnected{ID: 1, Reason: "closed by server"}
	w.Tick()
}

func TestBatchingAndPrompt(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	bob.batches = nil

	// A tick with no output produces no batch at all.
	w.Tick()
	if len(bob.batches) != 0 {
		t.Fatalf("idle tick sent %d batches", len(bob.batches))
	}

	// One command: one batch, ending in a prompt, room message structured.
	send(w, 1, "look")
	if len(bob.batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(bob.batches))
	}
	msgs := bob.batches[0].Messages
	if msgs[0].Type != output.Room || msgs[len(msgs)-1].Type != output.Prompt {
		t.Fatalf("batch shape wrong: %+v", msgs)
	}
	rd, ok := msgs[0].Data.(output.RoomData)
	if !ok || rd.Vnum != 1 || strings.Join(rd.Exits, ",") != "north" {
		t.Fatalf("room data wrong: %#v", msgs[0].Data)
	}
	if !bob.batches[0].Color {
		t.Fatal("color should default on")
	}
	bob.take()

	send(w, 1, "color")
	if out := bob.take(); !strings.Contains(out, "Color is now off") {
		t.Fatalf("color toggle: %q", out)
	}
	send(w, 1, "look")
	if last := bob.batches[len(bob.batches)-1]; last.Color {
		t.Fatal("batch should carry color off")
	}
}

func TestPlayerInputIsEscaped(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	bob.take()
	alice.take()
	send(w, 1, "say {R}red{x}")
	if out := alice.take(); !strings.Contains(out, "Bob says '{R}red{x}'") {
		t.Fatalf("player color tokens were interpreted: %q", out)
	}
}

func TestOneCommandPerTick(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	w.Events() <- session.Input{ID: 1, Line: "north"}
	w.Events() <- session.Input{ID: 1, Line: "south"}
	w.Tick()
	if out := bob.take(); !strings.Contains(out, "North\n") || strings.Contains(out, "Hub\n") {
		t.Fatalf("expected only first command to run: %q", out)
	}
	w.Tick()
	if out := bob.take(); !strings.Contains(out, "Hub\n") {
		t.Fatalf("expected second command on next tick: %q", out)
	}
}

func TestLookupPrefixOrder(t *testing.T) {
	cases := map[string]string{
		"n": "north", "s": "south", "e": "east", "w": "west", "u": "up", "d": "down",
		"l": "look", "lo": "look", "ex": "exits", "sa": "say", "wh": "who", "qui": "quit",
		"NORTH": "north", "sav": "save", "pass": "password",
	}
	for in, want := range cases {
		c := lookup(in, false)
		if c == nil || c.name != want {
			t.Errorf("lookup(%q) = %v, want %s", in, c, want)
		}
	}
	for _, in := range []string{"q", "x", "", "e x", "pas", "copyove"} {
		if c := lookup(in, true); c != nil {
			t.Errorf("lookup(%q) = %s, want nil", in, c.name)
		}
	}
}

func TestBannerOnConnect(t *testing.T) {
	w := testWorld(t)
	c := &fakeConn{id: 9}
	w.Events() <- session.Connected{Conn: c}
	w.Tick()
	out := c.take()
	if !strings.Contains(out, "\\____//_/") || !strings.Contains(out, "By what name") {
		t.Fatalf("banner missing: %q", out)
	}
	// A banner file under the data directory replaces the default.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "banner.txt"), []byte("CUSTOM ART"), 0o644)
	if got := loadBanner(dir); got != "CUSTOM ART\n" {
		t.Fatalf("custom banner: %q", got)
	}
	if got := loadBanner(filepath.Join(dir, "missing")); got != defaultBanner {
		t.Fatal("default banner not used when the file is missing")
	}
}

func TestHelpChatAndYell(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	send(w, 2, "help") // Alice is not an admin; Bob, the first character, is
	o := alice.take()
	for _, want := range []string{"Movement", "kill", "cast", "gtell", "yell", "help <command>"} {
		if !strings.Contains(o, want) {
			t.Fatalf("help missing %q: %q", want, o)
		}
	}
	if strings.Contains(o, "Builder") {
		t.Fatalf("help showed builder commands to a player: %q", o)
	}
	send(w, 1, "help")
	if o := bob.take(); !strings.Contains(o, "Builder") || !strings.Contains(o, "simulate") {
		t.Fatalf("admin help: %q", o)
	}
	send(w, 1, "help k")
	if o := bob.take(); !strings.Contains(o, "kill <target>") {
		t.Fatalf("help kill: %q", o)
	}
	send(w, 1, "help e")
	if o := bob.take(); !strings.Contains(o, "walk through an exit") {
		t.Fatalf("help east: %q", o)
	}
	send(w, 1, "help nonsense")
	if o := bob.take(); !strings.Contains(o, "no command called that") {
		t.Fatalf("help unknown: %q", o)
	}
	// Chat reaches everyone; yell reaches four rooms but not five.
	w.players[2].Room = w.content.Rooms.Rooms[31] // four steps from the hub: 1-2-3-30-31
	send(w, 1, "chat hello all")
	if o := alice.take(); !strings.Contains(o, "Bob chats 'hello all'") {
		t.Fatalf("chat: %q", o)
	}
	if o := bob.take(); !strings.Contains(o, "You chat 'hello all'") {
		t.Fatalf("chat self: %q", o)
	}
	send(w, 1, "yell over here")
	if o := alice.take(); !strings.Contains(o, "Bob yells 'over here'") {
		t.Fatalf("yell at range 4: %q", o)
	}
	w.players[2].Room = w.content.Rooms.Rooms[32]
	send(w, 1, "yell again")
	if o := alice.take(); strings.Contains(o, "yells") {
		t.Fatalf("yell heard at range 5: %q", o)
	}
	if o := bob.take(); !strings.Contains(o, "You yell 'again'") {
		t.Fatalf("yell self: %q", o)
	}
}
