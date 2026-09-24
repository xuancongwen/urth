// Package world owns all mutable game state and the loop that mutates it.
// Exactly one goroutine runs World.Run; everything else talks to it through
// the session event channel. See docs/DECISIONS.md D2.
package world

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"urth/internal/config"
	"urth/internal/output"
	"urth/internal/room"
	"urth/internal/session"
)

// eventBuffer is how many transport events may queue between ticks.
const eventBuffer = 4096

// maxEventsPerTick bounds how much transport traffic one tick will absorb so a
// flood cannot starve the game clock.
const maxEventsPerTick = 1024

// World is the game.
type World struct {
	cfg     config.Config
	log     *slog.Logger
	rooms   *room.World
	players map[session.ID]*Player
	events  chan session.Event

	tick       time.Duration
	roundTicks int
	tickCount  uint64
	started    time.Time
}

// New creates a world over an already-loaded map.
func New(cfg config.Config, rooms *room.World, log *slog.Logger) *World {
	tick := time.Duration(cfg.Timing.TickMs) * time.Millisecond
	roundTicks := int(time.Duration(cfg.Timing.RoundSeconds) * time.Second / tick)
	if roundTicks < 1 {
		roundTicks = 1
	}
	return &World{
		cfg:        cfg,
		log:        log,
		rooms:      rooms,
		players:    map[session.ID]*Player{},
		events:     make(chan session.Event, eventBuffer),
		tick:       tick,
		roundTicks: roundTicks,
	}
}

// Events is the channel transports send to.
func (w *World) Events() chan<- session.Event { return w.events }

// Run drives the world at a fixed tick until ctx is cancelled, then says
// goodbye to everyone and returns.
func (w *World) Run(ctx context.Context) {
	w.started = time.Now()
	w.log.Info("world running", "tick", w.tick, "round_ticks", w.roundTicks, "rooms", len(w.rooms.Rooms))
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

// Tick performs one iteration of the loop: absorb transport events, run one
// queued command per player, fire round-based logic when due, then deliver
// everyone's output in one batch each. Exported so tests can drive the world
// deterministically.
func (w *World) Tick() {
	w.tickCount++
	w.drainEvents()
	w.runCommands()
	if w.tickCount%uint64(w.roundTicks) == 0 {
		w.round()
	}
	w.flush()
}

// flush sends each player's pending output. Players who asked to quit are
// disconnected after their farewell goes out.
func (w *World) flush() {
	for id, p := range w.players {
		p.flush(w.prompt(p))
		if p.closing {
			// The transport will still emit Disconnected; drop the player now
			// so late input is not processed.
			w.leave(p)
			delete(w.players, id)
		}
	}
}

// prompt builds the in-game prompt. Vitals plug in here once they exist.
func (w *World) prompt(_ *Player) output.Message {
	return output.Message{Type: output.Prompt, Text: "> "}
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

func (w *World) handleEvent(ev session.Event) {
	switch e := ev.(type) {
	case session.Connected:
		p := newPlayer(e.Conn)
		w.players[e.Conn.ID()] = p
		w.log.Info("connected", "session", e.Conn.ID(), "addr", e.Conn.RemoteAddr())
		p.Send("Welcome to {C}" + output.Escape(w.cfg.Server.Name) + "{x}.\n")
		p.SendPrompt("By what name do you wish to be known? ")
	case session.Disconnected:
		p, ok := w.players[e.ID]
		if !ok {
			return
		}
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
		switch p.State {
		case StateLogin:
			w.handleLogin(p, line)
		case StatePlaying:
			w.dispatch(p, line)
		}
	}
}

// round runs once every Timing.RoundSeconds. Nothing is paced by rounds yet.
func (w *World) round() {
	w.log.Debug("round", "tick", w.tickCount, "players", len(w.players))
}

func (w *World) shutdown() {
	w.log.Info("world stopping", "players", len(w.players))
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
