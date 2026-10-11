package world

import (
	"strings"
	"testing"
	"time"

	"urth/internal/effect"
	"urth/internal/output"
	"urth/internal/session"
)

// sessionToken returns the last resume token sent on c.
func sessionToken(c *fakeConn) string {
	token := ""
	for _, b := range c.batches {
		for _, m := range b.Messages {
			if m.Type == output.Session {
				token = m.Data.(output.SessionData).Token
			}
		}
	}
	return token
}

func TestLinkdeadAndResume(t *testing.T) {
	w := testWorld(t)
	bob := login(t, w, 1, "Bob")
	alice := login(t, w, 2, "Alice")
	p := w.players[1]
	p.Silver = 77
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "feat", Params: map[string]any{"feat": "tough"}}})
	token := sessionToken(bob)
	if token == "" {
		t.Fatal("no resume token on entering the game")
	}
	alice.take()

	w.Events() <- session.Disconnected{ID: 1, Reason: "client disconnected"}
	w.Tick()
	if q, ok := w.players[1]; !ok || !q.linkdead() || q.Room == nil {
		t.Fatal("dropped connection did not leave the character link-dead in the world")
	}
	if o := alice.take(); !strings.Contains(o, "Bob has lost the link.") {
		t.Fatalf("room not told: %q", o)
	}

	c := &fakeConn{id: 3}
	w.Events() <- session.Connected{Conn: c, Token: token}
	w.Tick()
	q, ok := w.players[3]
	if !ok || q != p || q.linkdead() || q.Silver != 77 || len(q.Effects) == 0 {
		t.Fatalf("resume: same=%v linkdead=%v silver=%d effects=%d", q == p, q != nil && q.linkdead(), q.Silver, len(q.Effects))
	}
	if _, still := w.players[1]; still {
		t.Fatal("old session still listed")
	}
	if o := c.take(); !strings.Contains(o, "Reconnecting.") || !strings.Contains(o, "Hub") {
		t.Fatalf("resumed output: %q", o)
	}
	if next := sessionToken(c); next == "" || next == token {
		t.Fatalf("token not rotated: %q", next)
	}
	if o := alice.take(); !strings.Contains(o, "Bob has reconnected.") {
		t.Fatalf("room not told of the return: %q", o)
	}

	// The spent token no longer resumes anything.
	d := &fakeConn{id: 4}
	w.Events() <- session.Connected{Conn: d, Token: token}
	w.Tick()
	if o := d.take(); !strings.Contains(o, "By what name") {
		t.Fatalf("spent token accepted: %q", o)
	}
}

func TestLinkdeadExpiresAndQuitDoesNotLinger(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	w.Events() <- session.Disconnected{ID: 1, Reason: "client disconnected"}
	w.Tick()
	w.players[1].linkdeadUntil = time.Now().Add(-time.Second)
	for i := 0; i < w.roundTicks; i++ {
		w.Tick()
	}
	if _, still := w.players[1]; still {
		t.Fatal("link-dead character outstayed its grace")
	}
	if len(w.resume) != 0 {
		t.Fatalf("tokens left behind: %d", len(w.resume))
	}

	login(t, w, 2, "Alice")
	send(w, 2, "quit")
	w.Events() <- session.Disconnected{ID: 2, Reason: "closed by server"}
	w.Tick()
	if _, still := w.players[2]; still {
		t.Fatal("a quit left a link-dead character")
	}

	w.cfg.World.LinkdeadMinutes = 0
	login(t, w, 5, "Carol")
	w.Events() <- session.Disconnected{ID: 5, Reason: "client disconnected"}
	w.Tick()
	if _, still := w.players[5]; still {
		t.Fatal("with no grace, a drop should remove the character")
	}
}

// TestSigninToPlayingCharacterKeepsEverything: a second sign-in takes the
// character over whole (it used to lose silver, effects, and quest points).
func TestSigninToPlayingCharacterKeepsEverything(t *testing.T) {
	w := testWorld(t)
	login(t, w, 1, "Bob")
	p := w.players[1]
	p.Silver, p.questPoints = 55, 9
	p.Effects = append(p.Effects, effect.Active{Spec: effect.Spec{Kind: "feat", Params: map[string]any{"feat": "tough"}}})
	signin(t, w, 2, "Bob", "secret5")
	q := w.players[2]
	if q != p || q.Silver != 55 || q.questPoints != 9 || len(q.Effects) == 0 {
		t.Fatalf("takeover: same=%v silver=%d qp=%d effects=%d", q == p, q.Silver, q.questPoints, len(q.Effects))
	}
	// And a sign-in reclaims a link-dead character the same way.
	w.Events() <- session.Disconnected{ID: 2, Reason: "client disconnected"}
	w.Tick()
	signin(t, w, 3, "Bob", "secret5")
	if q := w.players[3]; q != p || q.linkdead() || q.Silver != 55 {
		t.Fatalf("reclaim link-dead: same=%v", q == p)
	}
}
