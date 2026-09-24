// Package output is the structured message model the world emits and the
// renderers transports use to turn it into text, ANSI, or HTML. Game code
// never formats for a particular client. See docs/DECISIONS.md D4.
package output

// Type classifies a message so clients can style or route it.
type Type string

const (
	// Text is ordinary narration, speech, or command feedback.
	Text Type = "text"
	// Room is a room description. Data carries RoomData.
	Room Type = "room"
	// Prompt is the input prompt. Rendered on its own line by text clients.
	Prompt Type = "prompt"
	// System is server notices: shutdown, connection issues.
	System Type = "system"
	// EchoOff asks the client to stop echoing input (password entry).
	EchoOff Type = "echo-off"
	// EchoOn restores input echo.
	EchoOn Type = "echo-on"
	// Reconnect tells a client that cannot survive a copyover to reconnect
	// with the token in Data (ReconnectData). Text clients ignore it.
	Reconnect Type = "reconnect"
)

// ReconnectData accompanies a Reconnect message.
type ReconnectData struct {
	Token string `json:"token"`
}

// Message is one unit of output. Text uses brace color tokens (see color.go)
// and "\n" line endings; transports translate both.
type Message struct {
	Type Type   `json:"type"`
	Text string `json:"text"`
	Data any    `json:"data,omitempty"`
}

// RoomData accompanies a Room message for clients that want structure.
type RoomData struct {
	Vnum    int      `json:"vnum"`
	Name    string   `json:"name"`
	Exits   []string `json:"exits"`
	Players []string `json:"players"`
}

// Batch is everything a player receives in one tick, delivered as a unit so
// a transport can write once and place the prompt last.
type Batch struct {
	Messages []Message
	// Color is the player's preference. Text transports honor it; a web
	// client always receives markup.
	Color bool
}

// Empty reports whether there is nothing to send.
func (b Batch) Empty() bool { return len(b.Messages) == 0 }
