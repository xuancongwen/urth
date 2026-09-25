package world

import (
	"strings"

	"urth/internal/output"
	"urth/internal/session"
	"urth/internal/store"
)

// State is where a session is in its life.
type State int

const (
	// StateGetName: connected, choosing a name.
	StateGetName State = iota
	// StateGetOldPassword: existing character, awaiting password.
	StateGetOldPassword
	// StateConfirmNewName: new name typed, awaiting yes/no.
	StateConfirmNewName
	// StateGetNewPassword: new character, choosing a password.
	StateGetNewPassword
	// StateConfirmNewPassword: retyping the new password.
	StateConfirmNewPassword
	// StateBusy: waiting on off-goroutine work; input is ignored.
	StateBusy
	// StatePlaying: in the world.
	StatePlaying
)

// maxQueuedInput bounds unprocessed lines per player. Beyond this, input is
// dropped rather than letting a client fill memory.
const maxQueuedInput = 32

// Player is a connected session and, once past login, a character.
type Player struct {
	*Character
	conn  session.Conn
	State State
	Admin bool
	// Color is the player's color preference, honored by text transports.
	Color bool

	rec *store.Record
	// visited is the set of room vnums this character has seen, for the
	// map; saved with the record.
	visited map[int]bool
	// Quests (quest.go): points banked, the quest under way, and rounds
	// left before the questmaster will give another.
	questPoints int
	quest       *store.SavedQuest
	questWait   int

	// Login scratch.
	pendingName     string
	pendingPassword string
	passwordTries   int

	input   []string
	pending []output.Message
	// closing is set by quit; the connection is closed after the final flush
	// so the farewell is delivered.
	closing bool
}

func newPlayer(c session.Conn) *Player {
	p := &Player{Character: newCharacter("", nil), conn: c, State: StateGetName, Color: true}
	p.Character.player = p
	return p
}

// setName sets the character's name and the keyword players use for it.
func (p *Player) setName(name string) {
	p.Name = name
	p.Keywords = []string{strings.ToLower(name)}
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
