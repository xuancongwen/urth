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
	// Expeditions lists the areas an expedition can set out for, in Data
	// as []Expedition, beside the text listing. Text clients ignore it.
	Expeditions Type = "expeditions"
	// Offers is what a merchant, trainer, or quest vendor offers, in Data
	// as OffersData, beside the text listing, so a client can make a
	// button of each. Text clients ignore it.
	Offers Type = "offers"
	// Session carries a resume token in Data (SessionData) once a player
	// is in the game. A web client that loses its connection reconnects
	// with it and gets the same character back, link-dead in the meantime.
	// The token is single-use; every resume brings a new one. Text clients
	// ignore it.
	Session Type = "session"
	// Reward is what a player gained in one tick, in Data as RewardData:
	// a kill's experience and loot, a quest's pay, an expedition's haul.
	// Text clients ignore it; the same news is in the text.
	Reward Type = "reward"
	// Chat is something a player said: say, chat, yell, tell, group
	// talk. Text clients show it like any text; a client may set it apart.
	Chat Type = "chat"
	// Death tells a player where their corpse lies, in Data as DeathData,
	// so a client can lead them back. Text clients ignore it.
	Death Type = "death"
)

// DeathData accompanies a Death message: the room the corpse is in, its
// name and area (directory and name), and the rounds before it rots.
type DeathData struct {
	Room     int    `json:"room"`
	Name     string `json:"name"`
	AreaDir  string `json:"areaDir"`
	AreaName string `json:"areaName"`
	Rounds   int    `json:"rounds"`
	Seconds  int    `json:"seconds"`
}

// RewardData accompanies a Reward message. Kills names each foe whose
// death paid it, in order; Title names anything else that did ("Quest
// complete"); Level is the new level when one was gained.
type RewardData struct {
	Kills       []string   `json:"kills,omitempty"`
	Title       string     `json:"title,omitempty"`
	XP          int        `json:"xp,omitempty"`
	Silver      int        `json:"silver,omitempty"`
	QuestPoints int        `json:"questPoints,omitempty"`
	Items       []GearItem `json:"items"`
	Level       int        `json:"level,omitempty"`
}

// ChatData accompanies a Chat message that is a tell to the reader:
// From is who sent it, so a client can offer to reply.
type ChatData struct {
	Tell bool   `json:"tell"`
	From string `json:"from"`
}

// SessionData accompanies a Session message.
type SessionData struct {
	Token string `json:"token"`
}

// OffersData accompanies an Offers message.
type OffersData struct {
	From  string  `json:"from"`
	Items []Offer `json:"items"`
}

// Offer is one thing on offer: its name, its price as words, the command
// that takes it, and a note ("level 5", "you know this") when one applies.
type Offer struct {
	Name string `json:"name"`
	// Color is the color code an item's name is shown in, as listings do.
	Color   string `json:"color,omitempty"`
	Price   string `json:"price"`
	Command string `json:"command"`
	Note    string `json:"note,omitempty"`
	// About says what it does (a feat's description); Disabled marks an
	// offer that cannot be taken now, with Note saying why.
	About    string `json:"about,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Expedition is one area an expedition can go to: its directory, which
// the expedition command takes, its name, how many kinds of foe near the
// player's level it holds, and their levels.
type Expedition struct {
	Area  string `json:"area"`
	Name  string `json:"name"`
	Kinds int    `json:"kinds"`
	Low   int    `json:"low"`
	High  int    `json:"high"`
	// Safe says whether the player stands somewhere an expedition can
	// set out from; Wait is rounds of rest still owed.
	Safe bool `json:"safe"`
	Wait int  `json:"wait,omitempty"`
}

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
	// Fixed marks an item that is part of the room and cannot be taken;
	// Container one things can be put in.
	Fixed     bool `json:"fixed,omitempty"`
	Container bool `json:"container,omitempty"`
	// Rarity is a rolled item's tier and Color the color code its name is
	// shown in, a tier's or a named item's (docs/RULES.md 5.3), so a client
	// can show it as the text does.
	Rarity string `json:"rarity,omitempty"`
	Color  string `json:"color,omitempty"`
	// Services are what a creature does for players: shop, trainer, quest
	// (a questmaster), vendor (sells for quest points).
	Services []string `json:"services,omitempty"`
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
	// XP is total experience; LevelXP and NextXP are the totals at which
	// this level began and the next begins (NextXP 0 at the top), for a
	// progress bar.
	XP      int `json:"xp"`
	LevelXP int `json:"levelXp"`
	NextXP  int `json:"nextXp"`
	Silver  int `json:"silver"`
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
	// Stats are the character's stats in display order, each as trained
	// and as gear and effects make it; StatPoints and FeatPoints are
	// unspent picks; QuestPoints are banked.
	Stats []Stat `json:"stats"`
	// Quest is the quest under way, if any; Group the other members of the
	// character's group; Effects the timed effects on it.
	Quest       *QuestState   `json:"quest,omitempty"`
	Group       []Companion   `json:"group"`
	Effects     []EffectState `json:"effects"`
	StatPoints  int           `json:"statPoints"`
	FeatPoints  int           `json:"featPoints"`
	QuestPoints int           `json:"questPoints"`
}

// QuestState is a quest under way: kill or fetch, the target's name, the
// area it is in, the time left in words, and whether it only waits to be
// handed in.
type QuestState struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Area   string `json:"area"`
	Left   string `json:"left"`
	Done   bool   `json:"done,omitempty"`
	// Room is where the target was seen or the item was placed, for a
	// map marker; AreaDir is its area as the map names it.
	Room    int    `json:"room,omitempty"`
	AreaDir string `json:"areaDir,omitempty"`
}

// Companion is a fellow group member: Here says they share the room.
type Companion struct {
	Name      string `json:"name"`
	Health    int    `json:"health"`
	HealthMax int    `json:"healthMax"`
	Here      bool   `json:"here,omitempty"`
}

// EffectState is a timed effect: what it does, in words, and its time
// left, in rounds and in words.
type EffectState struct {
	Text   string `json:"text"`
	Rounds int    `json:"rounds"`
	Left   string `json:"left"`
}

// Stat is one stat on a state snapshot.
type Stat struct {
	Name      string `json:"name"`
	Base      int    `json:"base"`
	Effective int    `json:"effective"`
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
// its rarity's or its named prototype's color code, empty for a plain item.
type GearItem struct {
	// ID is the instance's id, which "#<id>" names in any command.
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Count  int    `json:"count,omitempty"`
	Type   string `json:"type"`
	Slot   string `json:"slot,omitempty"`
	Rarity string `json:"rarity,omitempty"`
	Color  string `json:"color,omitempty"`
	// The numbers a comparison needs: a weapon's damage range per swing
	// and swings per round, armor's defense, the item's level, and how many
	// effects it carries (affixes and intrinsic ones).
	Level      int     `json:"level,omitempty"`
	DamageLow  int     `json:"damageLow,omitempty"`
	DamageHigh int     `json:"damageHigh,omitempty"`
	Speed      float64 `json:"speed,omitempty"`
	Hands      int     `json:"hands,omitempty"`
	Defense    float64 `json:"defense,omitempty"`
	Effects    int     `json:"effects,omitempty"`
	// Compare weighs a carried weapon or armor piece against what is worn
	// where it would go; nil for anything else and for worn items.
	Compare *GearCompare `json:"compare,omitempty"`
}

// GearCompare is the verdict `inventory` marks an item with. Dir is up,
// down, or same. Delta is the item's score less the weakest worn piece's
// in its slot, Worn that piece's id; Empty means the slot has room, and
// then Delta is the item's own score.
type GearCompare struct {
	Dir   string  `json:"dir"`
	Delta float64 `json:"delta"`
	Worn  uint64  `json:"worn,omitempty"`
	Empty bool    `json:"empty,omitempty"`
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
	// Locked says what the character still lacks to use it ("level 10",
	// "16 intelligence"), empty when nothing.
	Locked string `json:"locked,omitempty"`
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
