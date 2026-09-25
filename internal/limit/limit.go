// Package limit protects a listener from abuse: a cap on live connections,
// a cap per remote address, and a token bucket on input lines per
// connection. Both transports share it so a public web hostname and a
// forwarded telnet port get the same protection. Zero values disable a
// limit. See docs/DEPLOY.md.
package limit

import (
	"sync"
	"time"
)

// Config is the set of limits, as loaded from config.yaml.
type Config struct {
	// MaxConns caps connections on the listener. 0 is unlimited.
	MaxConns int
	// MaxPerIP caps connections from one remote address. 0 is unlimited.
	MaxPerIP int
	// LinesPerSecond is the sustained input rate one connection may send;
	// Burst is how many lines it may send at once. 0 disables the bucket.
	LinesPerSecond int
	Burst          int
	// FloodLimit is how many lines over the rate a connection may send
	// before it is dropped rather than throttled. 0 means never drop.
	FloodLimit int
}

// Gate tracks connections for one listener.
type Gate struct {
	cfg   Config
	mu    sync.Mutex
	total int
	perIP map[string]int
}

// New creates a gate. A nil Gate admits everything.
func New(cfg Config) *Gate {
	return &Gate{cfg: cfg, perIP: map[string]int{}}
}

// Admit registers a connection from ip and reports whether it may stay.
// The caller must call Release once for every admitted connection. With
// force set the connection is counted but never refused; used for sockets
// inherited across a copyover, which are already live.
func (g *Gate) Admit(ip string, force bool) (ok bool) {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !force {
		if g.cfg.MaxConns > 0 && g.total >= g.cfg.MaxConns {
			return false
		}
		if g.cfg.MaxPerIP > 0 && g.perIP[ip] >= g.cfg.MaxPerIP {
			return false
		}
	}
	g.total++
	g.perIP[ip]++
	return true
}

// Release undoes Admit.
func (g *Gate) Release(ip string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.total--
	if g.perIP[ip] <= 1 {
		delete(g.perIP, ip)
	} else {
		g.perIP[ip]--
	}
}

// Count reports live connections, for tests and stats.
func (g *Gate) Count() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.total
}

// Bucket is one connection's input allowance. It is used from a single
// read loop and needs no locking.
type Bucket struct {
	rate   float64 // tokens per second
	burst  float64
	tokens float64
	last   time.Time
	over   int // lines refused since the last accepted one
	limit  int
}

// NewBucket returns a full bucket for one connection. A nil Gate or a zero
// rate returns nil, which Allow treats as unlimited.
func (g *Gate) NewBucket(now time.Time) *Bucket {
	if g == nil || g.cfg.LinesPerSecond <= 0 {
		return nil
	}
	burst := max(g.cfg.Burst, 1)
	return &Bucket{rate: float64(g.cfg.LinesPerSecond), burst: float64(burst), tokens: float64(burst), last: now, limit: g.cfg.FloodLimit}
}

// Verdict is what Allow decides about one line.
type Verdict int

const (
	// Accept: deliver the line.
	Accept Verdict = iota
	// Drop: discard the line, keep the connection.
	Drop
	// Kick: the client is flooding; close it.
	Kick
)

// Allow spends one token for a line arriving at now.
func (b *Bucket) Allow(now time.Time) Verdict {
	if b == nil {
		return Accept
	}
	b.tokens = min(b.burst, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		b.over = 0
		return Accept
	}
	b.over++
	if b.limit > 0 && b.over > b.limit {
		return Kick
	}
	return Drop
}
