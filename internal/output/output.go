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
	// Commands carries the command names the player may use, in Data as a
	// []string, once on entering the game. Clients use it for completion.
	Commands Type = "commands"
	// Reconnect tells a client the server is about to go away and come
	// back: with a token in Data (ReconnectData) before a copyover, so the
	// client resumes without a login; with no Data before a shutdown, so
	// the client retries until the server returns. Text clients ignore it.
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
	Area    string   `json:"area"`
	Exits   []string `json:"exits"`
	Players []string `json:"players"`
	// Mobs and Items are what is here, each with the reference a command
	// would use to name it ("guard", "2.guard"), so a client can act on a
	// click.
	Mobs  []Entity `json:"mobs"`
	Items []Entity `json:"items"`
	// Map is the neighbourhood for drawing; nil when the room is not on
	// the map or cannot be seen.
	Map *MapData `json:"map,omitempty"`
}

// Entity is one thing in a room as a client sees it.
type Entity struct {
	Name  string `json:"name"`
	Ref   string `json:"ref"`
	Count int    `json:"count,omitempty"`
}

// MapData is the rooms near the player, positioned for drawing.
type MapData struct {
	Area  string    `json:"area"`
	Rooms []MapRoom `json:"rooms"`
}

// MapRoom is one room on the map. Exits map direction to vnum; Seen says
// whether this player has stood in it.
type MapRoom struct {
	Vnum  int            `json:"vnum"`
	Name  string         `json:"name"`
	X     int            `json:"x"`
	Y     int            `json:"y"`
	Z     int            `json:"z"`
	Exits map[string]int `json:"exits"`
	Seen  bool           `json:"seen"`
	Safe  bool           `json:"safe,omitempty"`
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
