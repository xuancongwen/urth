package world

import (
	"strings"
	"unicode"

	"urth/internal/output"
	"urth/internal/room"
	"urth/internal/session"
)

// State is where a session is in its life.
type State int

const (
	// StateLogin: connected, choosing a name.
	StateLogin State = iota
	// StatePlaying: in the world.
	StatePlaying
)

// maxQueuedInput bounds unprocessed lines per player. Beyond this, input is
// dropped rather than letting a client fill memory.
const maxQueuedInput = 32

// Player is a connected session and, once past login, a character. Accounts
// and persistence arrive in milestone 4; for now a player is just a name.
type Player struct {
	conn  session.Conn
	State State
	Name  string
	Room  *room.Room
	// Color is the player's color preference, honored by text transports.
	Color bool

	input   []string
	pending []output.Message
	// closing is set by quit; the connection is closed after the final flush
	// so the farewell is delivered.
	closing bool
}

func newPlayer(c session.Conn) *Player {
	return &Player{conn: c, State: StateLogin, Color: true}
}

// Send queues a text message for delivery at the end of the tick. Text may
// carry color tokens and should end with a newline.
func (p *Player) Send(text string) {
	p.SendMsg(output.Message{Type: output.Text, Text: text})
}

// SendMsg queues any message.
func (p *Player) SendMsg(m output.Message) { p.pending = append(p.pending, m) }

// SendPrompt queues a prompt-typed message, used during login where the
// default in-game prompt does not apply.
func (p *Player) SendPrompt(text string) {
	p.SendMsg(output.Message{Type: output.Prompt, Text: text})
}

// disconnect schedules the connection to close after this tick's output.
func (p *Player) disconnect() { p.closing = true }

// flush delivers pending output. Playing characters get a prompt whenever
// there was output, as ROM does.
func (p *Player) flush(prompt output.Message) {
	if len(p.pending) == 0 && !p.closing {
		return
	}
	if p.State == StatePlaying && len(p.pending) > 0 && !p.closing {
		p.pending = append(p.pending, prompt)
	}
	if len(p.pending) > 0 {
		p.conn.Send(output.Batch{Messages: p.pending, Color: p.Color})
		p.pending = nil
	}
	if p.closing {
		p.conn.Close()
	}
}

func (p *Player) queue(line string) {
	if len(p.input) >= maxQueuedInput {
		return
	}
	p.input = append(p.input, line)
}

func (p *Player) hasInput() bool { return len(p.input) > 0 }

func (p *Player) dequeue() string {
	line := p.input[0]
	p.input = p.input[1:]
	return line
}

// handleLogin is the milestone-2 stand-in for character creation: pick a
// name, enter the world.
func (w *World) handleLogin(p *Player, line string) {
	name, ok := cleanName(line)
	if !ok {
		p.SendPrompt("Names are 3 to 12 letters. Try again: ")
		return
	}
	for _, other := range w.players {
		if other != p && other.Name == name {
			p.SendPrompt(name + " is already playing. Choose another name: ")
			return
		}
	}
	start, ok := w.rooms.Get(w.cfg.World.StartRoom)
	if !ok {
		p.SendMsg(output.Message{Type: output.System, Text: "The world has no start room. Try again later.\n"})
		p.disconnect()
		return
	}
	p.Name = name
	p.State = StatePlaying
	p.Room = start
	w.log.Info("logged in", "session", p.conn.ID(), "name", name)
	p.Send("\nWelcome, " + name + ".\n\n")
	w.act("$n has entered the game.", p, nil, "", toRoom)
	w.look(p)
}

// cleanName validates and capitalises a chosen name.
func cleanName(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) < 3 || len(s) > 12 {
		return "", false
	}
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return "", false
		}
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:]), true
}
