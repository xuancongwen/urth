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
	// State is a snapshot of the character for clients that draw it rather
	// than read it: vitals, the fight, the room, gear, and skills. Data
	// carries StateData. Sent only to sessions that asked with "client
	// state", and only when it changed. Text clients ignore it.
	State Type = "state"
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
	// Fixed marks an item that is part of the room and cannot be taken.
	Fixed bool `json:"fixed,omitempty"`
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
	// What a player who has stood here knows of it: Dark, the deity of a
	// Temple, the Services of whoever stands here now (shop, trainer,
	// quest, vendor), and each door's state by direction (open, closed,
	// locked). Left empty for rooms not yet seen.
	Dark     bool              `json:"dark,omitempty"`
	Temple   string            `json:"temple,omitempty"`
	Services []string          `json:"services,omitempty"`
	Doors    map[string]string `json:"doors,omitempty"`
	// What scan would show, for this room and the rooms it can see into:
	// creatures that are not peaceful, how far above the player the
	// strongest of them is in levels (negative is below), and players.
	Foes    int `json:"foes,omitempty"`
	Danger  int `json:"danger,omitempty"`
	Players int `json:"players,omitempty"`
	// Items says something lies on the floor; only the player's own room.
	Items bool `json:"items,omitempty"`
}

// StateData accompanies a State message. Every Ref is what a command
// would take to name the same thing ("2.sword").
type StateData struct {
	Name       string `json:"name"`
	Level      int    `json:"level"`
	Health     int    `json:"health"`
	HealthMax  int    `json:"healthMax"`
	Mana       int    `json:"mana"`
	ManaMax    int    `json:"manaMax"`
	Stamina    int    `json:"stamina"`
	StaminaMax int    `json:"staminaMax"`
	TNL        int    `json:"tnl"`
	Silver     int    `json:"silver"`
	// Position is standing, resting, or sleeping.
	Position string `json:"position"`
	// Target is who the character is fighting; Enemies everyone fighting
	// it, the target included.
	Target  *Combatant  `json:"target"`
	Enemies []Combatant `json:"enemies"`
	// Casting is the spell under way and its rounds left, if any.
	Casting *Casting `json:"casting,omitempty"`
	// Room is where the character stands. Its mobs and items carry refs;
	// it has no map (the room message has that).
	Room StateRoom `json:"room"`
	// Map is the area around the player, as the room message carries it.
	Map       *MapData   `json:"map,omitempty"`
	Inventory []GearItem `json:"inventory"`
	Equipment []GearItem `json:"equipment"`
	Skills    []Action   `json:"skills"`
	Spells    []Action   `json:"spells"`
}

// Combatant is one side of a fight as a client draws it.
type Combatant struct {
	Name      string `json:"name"`
	Ref       string `json:"ref"`
	Health    int    `json:"health"`
	HealthMax int    `json:"healthMax"`
}

// Casting is a spell in progress.
type Casting struct {
	Spell  string `json:"spell"`
	Rounds int    `json:"rounds"`
}

// StateRoom is the room part of a state snapshot.
type StateRoom struct {
	Vnum    int      `json:"vnum"`
	Name    string   `json:"name"`
	Exits   []string `json:"exits"`
	Safe    bool     `json:"safe,omitempty"`
	Players []string `json:"players"`
	Mobs    []Entity `json:"mobs"`
	Items   []Entity `json:"items"`
}

// GearItem is an item carried or worn. Slot is the position worn on, or
// for a carried item the position it would go on ("" if none). Color is
// its rarity's color code, empty for a plain item.
type GearItem struct {
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Count  int    `json:"count,omitempty"`
	Type   string `json:"type"`
	Slot   string `json:"slot,omitempty"`
	Rarity string `json:"rarity,omitempty"`
	Color  string `json:"color,omitempty"`
}

// Action is a skill or spell the character can use. Command is what to
// send to use it (without a target); Target is single, area, ally, self,
// group, or none; Cooldown is rounds until it is ready again.
type Action struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Command  string `json:"command"`
	Target   string `json:"target"`
	Cooldown int    `json:"cooldown"`
	// Mana and Stamina are what a use costs: spells spend mana, skills
	// stamina.
	Mana    int `json:"mana,omitempty"`
	Stamina int `json:"stamina,omitempty"`
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
