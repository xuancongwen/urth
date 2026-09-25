package limit

import (
	"testing"
	"time"
)

func TestGateCaps(t *testing.T) {
	g := New(Config{MaxConns: 3, MaxPerIP: 2})
	if !g.Admit("a", false) || !g.Admit("a", false) {
		t.Fatal("first two from a should be admitted")
	}
	if g.Admit("a", false) {
		t.Fatal("third from a should hit the per-ip cap")
	}
	if !g.Admit("b", false) {
		t.Fatal("b should be admitted")
	}
	if g.Admit("c", false) {
		t.Fatal("fourth connection should hit the total cap")
	}
	if !g.Admit("c", true) {
		t.Fatal("forced admit must never refuse")
	}
	g.Release("a")
	g.Release("c")
	if !g.Admit("c", false) {
		t.Fatal("released slot should be reusable")
	}
	if got := g.Count(); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	var nilGate *Gate
	if !nilGate.Admit("x", false) || nilGate.NewBucket(time.Now()) != nil {
		t.Fatal("nil gate must admit everything")
	}
}

func TestBucket(t *testing.T) {
	g := New(Config{LinesPerSecond: 10, Burst: 3, FloodLimit: 2})
	now := time.Unix(0, 0)
	b := g.NewBucket(now)
	for i := 0; i < 3; i++ {
		if b.Allow(now) != Accept {
			t.Fatalf("burst line %d should be accepted", i)
		}
	}
	if b.Allow(now) != Drop || b.Allow(now) != Drop {
		t.Fatal("lines over the burst should be dropped")
	}
	if b.Allow(now) != Kick {
		t.Fatal("sustained flood should kick")
	}
	now = now.Add(100 * time.Millisecond)
	if b.Allow(now) != Accept {
		t.Fatal("one token should refill in 100ms at 10/s")
	}
	if b.Allow(now) != Drop {
		t.Fatal("and only one")
	}
	now = now.Add(time.Hour)
	if b.Allow(now) != Accept || b.Allow(now) != Accept || b.Allow(now) != Accept || b.Allow(now) != Drop {
		t.Fatal("refill must cap at the burst")
	}
}
