package world

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"urth/internal/config"
	"urth/internal/output"
	"urth/internal/room"
	"urth/internal/session"
)

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
	dir := t.TempDir()
	rooms := map[string]string{
		"1.yaml": "vnum: 1\nname: Hub\ndescription: The hub.\nexits:\n  north: 2\n",
		"2.yaml": "vnum: 2\nname: North\ndescription: Up north.\nexits:\n  south: 1\n",
	}
	for name, body := range rooms {
		p := filepath.Join(dir, "a", "rooms", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rw, err := room.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Timing.TickMs = 100
	cfg.Timing.RoundSeconds = 1
	return New(cfg, rw, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// login connects a fake client and names it, ticking as needed.
func login(t *testing.T, w *World, id session.ID, name string) *fakeConn {
	t.Helper()
	c := &fakeConn{id: id}
	w.Events() <- session.Connected{Conn: c}
	w.Tick()
	if !strings.Contains(c.take(), "By what name") {
		t.Fatal("no greeting")
	}
	w.Events() <- session.Input{ID: id, Line: name}
	w.Tick()
	if out := c.take(); !strings.Contains(out, "Welcome, "+name) || !strings.Contains(out, "Hub") {
		t.Fatalf("login output wrong: %q", out)
	}
	return c
}

func send(w *World, id session.ID, line string) {
	w.Events() <- session.Input{ID: id, Line: line}
	w.Tick()
}

func TestLoginRejectsBadNames(t *testing.T) {
	w := testWorld(t)
	c := &fakeConn{id: 1}
	w.Events() <- session.Connected{Conn: c}
	w.Tick()
	c.take()
	for _, bad := range []string{"ab", "toolongofaname", "b0b", "bob smith"} {
		send(w, 1, bad)
		if out := c.take(); !strings.Contains(out, "3 to 12 letters") {
			t.Fatalf("%q accepted: %q", bad, out)
		}
	}
	send(w, 1, "bob")
	if out := c.take(); !strings.Contains(out, "Welcome, Bob") {
		t.Fatalf("capitalised name not welcomed: %q", out)
	}
}

func TestDuplicateNameRefused(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	c := &fakeConn{id: 2}
	w.Events() <- session.Connected{Conn: c}
	w.Tick()
	c.take()
	send(w, 2, "Bob")
	if out := c.take(); !strings.Contains(out, "already playing") {
		t.Fatalf("duplicate accepted: %q", out)
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
	if out := bob.take(); !strings.Contains(out, "North\nUp north.\n[Exits: south]") {
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
	if out := bob.take(); !strings.Contains(out, "  Alice\n  Bob\n") || !strings.Contains(out, "2 players online") {
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
		"NORTH": "north",
	}
	for in, want := range cases {
		c := lookup(in)
		if c == nil || c.name != want {
			t.Errorf("lookup(%q) = %v, want %s", in, c, want)
		}
	}
	for _, in := range []string{"q", "x", "", "e x"} {
		if c := lookup(in); c != nil {
			t.Errorf("lookup(%q) = %s, want nil", in, c.name)
		}
	}
}
