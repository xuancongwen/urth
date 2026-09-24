// Package world owns all mutable game state and the loop that mutates it.
// Exactly one goroutine runs World.Run; everything else talks to it through
// the session event channel or post(). See docs/DECISIONS.md D2.
package world

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"urth/internal/config"
	"urth/internal/copyover"
	"urth/internal/output"
	"urth/internal/room"
	"urth/internal/session"
	"urth/internal/store"
)

// eventBuffer is how many transport events may queue between ticks.
const eventBuffer = 4096

// maxEventsPerTick bounds how much transport traffic one tick will absorb so a
// flood cannot starve the game clock.
const maxEventsPerTick = 1024

// tokenTTL is how long a copyover reconnect token stays valid.
const tokenTTL = 2 * time.Minute

// Restore identifies a player to re-attach after a copyover.
type Restore = session.Restore

// Deps are the process-level hooks the world needs but must not own.
type Deps struct {
	Store *store.Store
	// Shutdown asks the process to stop. Used by the shutdown command.
	Shutdown func()
	// Copyover writes the state and execs the binary. It returns only on
	// failure. nil disables the copyover command.
	Copyover func(st copyover.State) error
	// Listeners reports bound sockets to hand over, descriptors already
	// marked inheritable.
	Listeners func() []copyover.Listener
}

// World is the game.
type World struct {
	cfg     config.Config
	log     *slog.Logger
	rooms   *room.World
	store   *store.Store
	deps    Deps
	players map[session.ID]*Player
	events  chan session.Event
	posts   chan func()
	tokens  map[string]pendingToken

	tick           time.Duration
	roundTicks     int
	autosaveRounds int
	tickCount      uint64
	roundCount     uint64
	started        time.Time
}

type pendingToken struct {
	restore Restore
	expires time.Time
}

// New creates a world over an already-loaded map.
func New(cfg config.Config, rooms *room.World, log *slog.Logger, deps Deps) *World {
	tick := time.Duration(cfg.Timing.TickMs) * time.Millisecond
	roundTicks := int(time.Duration(cfg.Timing.RoundSeconds) * time.Second / tick)
	if roundTicks < 1 {
		roundTicks = 1
	}
	autosaveRounds := cfg.Timing.AutosaveSeconds / cfg.Timing.RoundSeconds
	if autosaveRounds < 1 {
		autosaveRounds = 1
	}
	return &World{
		cfg:            cfg,
		log:            log,
		rooms:          rooms,
		store:          deps.Store,
		deps:           deps,
		players:        map[session.ID]*Player{},
		events:         make(chan session.Event, eventBuffer),
		posts:          make(chan func(), eventBuffer),
		tokens:         map[string]pendingToken{},
		tick:           tick,
		roundTicks:     roundTicks,
		autosaveRounds: autosaveRounds,
	}
}

// Events is the channel transports send to.
func (w *World) Events() chan<- session.Event { return w.events }

// post schedules fn to run on the world goroutine. It is how off-goroutine
// work (password hashing) hands results back.
func (w *World) post(fn func()) { w.posts <- fn }

// RegisterTokens accepts copyover reconnect tokens for clients that could
// not inherit a socket. Called once at startup before Run.
func (w *World) RegisterTokens(players []copyover.Player) {
	for _, cp := range players {
		if cp.Token != "" {
			w.tokens[cp.Token] = pendingToken{
				restore: Restore{Name: cp.Name, Room: cp.Room},
				expires: time.Now().Add(tokenTTL),
			}
		}
	}
}

// Run drives the world at a fixed tick until ctx is cancelled, then saves
// everyone, says goodbye, and returns.
func (w *World) Run(ctx context.Context) {
	w.started = time.Now()
	w.log.Info("world running", "tick", w.tick, "round_ticks", w.roundTicks, "autosave_rounds", w.autosaveRounds, "rooms", len(w.rooms.Rooms))
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.shutdown()
			return
		case <-ticker.C:
			w.Tick()
		}
	}
}

// Tick performs one iteration of the loop: absorb transport events and
// posted callbacks, run one queued command per player, fire round-based
// logic when due, then deliver everyone's output in one batch each.
// Exported so tests can drive the world deterministically.
func (w *World) Tick() {
	w.tickCount++
	w.drainEvents()
	w.drainPosts()
	w.runCommands()
	if w.tickCount%uint64(w.roundTicks) == 0 {
		w.round()
	}
	w.flush()
}

func (w *World) drainEvents() {
	for i := 0; i < maxEventsPerTick; i++ {
		select {
		case ev := <-w.events:
			w.handleEvent(ev)
		default:
			return
		}
	}
}

func (w *World) drainPosts() {
	for i := 0; i < maxEventsPerTick; i++ {
		select {
		case fn := <-w.posts:
			fn()
		default:
			return
		}
	}
}

func (w *World) handleEvent(ev session.Event) {
	switch e := ev.(type) {
	case session.Connected:
		p := newPlayer(e.Conn)
		w.players[e.Conn.ID()] = p
		w.log.Info("connected", "session", e.Conn.ID(), "addr", e.Conn.RemoteAddr())
		switch {
		case e.Restore != nil:
			w.restore(p, *e.Restore)
		case e.Token != "":
			if t, ok := w.tokens[e.Token]; ok && time.Now().Before(t.expires) {
				delete(w.tokens, e.Token)
				w.restore(p, t.restore)
			} else {
				w.log.Warn("bad or expired copyover token", "session", e.Conn.ID())
				w.greet(p)
			}
		default:
			w.greet(p)
		}
	case session.Disconnected:
		p, ok := w.players[e.ID]
		if !ok {
			return
		}
		w.save(p)
		w.leave(p)
		delete(w.players, e.ID)
		w.log.Info("disconnected", "session", e.ID, "name", p.Name, "reason", e.Reason)
	case session.Input:
		if p, ok := w.players[e.ID]; ok {
			p.queue(e.Line)
		}
	}
}

// runCommands gives every player at most one command this tick, in a stable
// order so no session is systematically favoured.
func (w *World) runCommands() {
	ids := make([]session.ID, 0, len(w.players))
	for id, p := range w.players {
		if p.hasInput() {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		p := w.players[id]
		line := p.dequeue()
		if p.State == StatePlaying {
			w.dispatch(p, line)
		} else {
			w.handleLogin(p, line)
		}
	}
}

// round runs once every Timing.RoundSeconds.
func (w *World) round() {
	w.roundCount++
	if w.roundCount%uint64(w.autosaveRounds) == 0 {
		if n := w.saveAll(); n > 0 {
			w.log.Debug("autosave", "players", n)
		}
	}
	now := time.Now()
	for tok, t := range w.tokens {
		if now.After(t.expires) {
			delete(w.tokens, tok)
		}
	}
}

// flush sends each player's pending output. Players who asked to quit are
// disconnected after their farewell goes out.
func (w *World) flush() {
	for id, p := range w.players {
		p.flush(w.prompt(p))
		if p.closing {
			// The transport will still emit Disconnected; drop the player now
			// so late input is not processed.
			w.save(p)
			w.leave(p)
			delete(w.players, id)
		}
	}
}

// prompt builds the in-game prompt. Vitals plug in here once they exist.
func (w *World) prompt(_ *Player) output.Message {
	return output.Message{Type: output.Prompt, Text: "> "}
}

func (w *World) shutdown() {
	w.log.Info("world stopping", "players", len(w.players), "saved", w.saveAll())
	for _, p := range w.players {
		p.SendMsg(output.Message{Type: output.System, Text: "\nThe world is shutting down. Goodbye.\n"})
		p.disconnect()
	}
	w.flush()
}

// leave removes a player from the map, telling the room.
func (w *World) leave(p *Player) {
	if p.Room == nil {
		return
	}
	w.act("$n has left the game.", p, nil, "", toRoom)
	p.Room = nil
}

// playersIn returns everyone in r, in join order.
func (w *World) playersIn(r *room.Room) []*Player {
	var out []*Player
	for _, p := range w.players {
		if p.Room == r {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].conn.ID() < out[j].conn.ID() })
	return out
}

// broadcast sends a system message to every playing character.
func (w *World) broadcast(text string) {
	for _, p := range w.players {
		if p.State == StatePlaying {
			p.SendMsg(output.Message{Type: output.System, Text: text})
		}
	}
}
