# data/

Root for everything the server reads and writes at runtime.

Layout:

- `world/<area>/`  `area.yaml`, `rooms/*.yaml`, `items/*.yaml`, `mobs/*.yaml`, `resets.yaml`
- `players/`  saved accounts and characters, one YAML file each (gitignored)
- `scripts/`  goja JavaScript rule hooks; `rules.js` is the live rule set (docs/RULES.md section 1 lists the hooks)
- rooms may carry `doors: {<direction>: {name: the oak door, closed: true}}` on an exit; declare it on one side and the loader mirrors it to the room beyond. A closed door blocks the way and hides the exit from `look`, `exits`, and `scan`; `look <direction>` finds it, `open` and `close` work it, and an area reset puts it back the way the file says. A door may also say `locked: true` (it starts shut and locked), `key: <item vnum>` (the item `lock` and `unlock` need, carried or held), and `pickproof: true` (`pick` never opens it); the tutorial's pine door (room 42) is the example
- rooms may carry `position: {x, y, z}` to anchor the area map the web client draws; most rooms are placed by walking exits from an anchor, and the server logs any room whose exits contradict the grid so a builder can anchor it (`internal/room/layout.go`)
- `banner.txt`  the sign-in screen, with `{Y}`-style color tokens; edit freely, no rebuild needed
- `schema/`  JSON Schema for each content file, for completion and validation in the editor. `.vscode/settings.json` maps them for VS Code's YAML extension. Any editor that runs yaml-language-server can use them; a file may also name its schema on its first line, `# yaml-language-server: $schema=../../../schema/room.schema.json`. `go test ./internal/content` fails if a schema drifts from the Go structs.
- a mob flagged `questmaster` hands out quests; a mob with `sells: [{item: <vnum>, points: <n>}]` trades those items for quest points; an item flagged `quest` is one the questmaster may plant for a fetch quest and is never placed by a reset (docs/RULES.md 7.6)
- a mob with `trades: [{item: <vnum>, silver: <n>, items: [<vnum>, ...]}]` is a merchant: `list` shows its wares, `buy` pays the silver, the listed items from the buyer's inventory (a vnum twice means two), or both, and `sell` pays half an item's `value` (a weapon or armor with none is worth ten silver a level)
- a mob with `spawns: {mob: <vnum>, every: <rounds>, max: <n>}` is a generator: while it lives it adds one of that mob to its room every `every` rounds (default 3) until `max` of them (default 3) stand there, fighting or not; a mob a placed generator spawns counts as placed
- an item flagged `newbie` is sub issue gear: every new character is created wearing one of each (the start area's items 130 to 134), and merchants will not buy it
- an area whose rooms are not meant to connect to the world, nor whose prototypes to be placed by resets (the balance range), says `detached: true` in `area.yaml`; `urth check` then leaves it alone

Areas under `world/`: `start` (the tutorial corridor where new
characters wake, rooms 40 to 46, opening north into the armory, the
Long Hall with its trainer and herald, and a shrine), `balance` (the test
lab, unreachable on foot: `goto 900`; one plain mob per level for the
simulator, and every mechanic in reach, see its README), and `solace` (a town in the vallenwoods, reached by
the stair under the armory). Below the stair, the Undercroft of Doors
(room 6) opens on four more: `shire` (west; Hobbiton, Bywater, Maggot's
farm, the Woody End and the Old Forest, levels 1 to 10), `swordcoast`
(east; Candlekeep, the Friendly Arm, Beregost, Nashkel and its mines,
levels 2 to 9), `icewind` (north; Bryn Shander, Lonelywood, the tundra,
and the dwarven mines under Kelvin's Cairn, levels 3 to 10), and
`drevlin` (south; the Gegs, the Kicksey-winsey, and an elven watership,
levels 3 to 9). `underdark` (levels 8 to 18, the drow and Blingdenstone)
has no door of its own: it is reached down the shaft at the bottom of the
Icewind Dale mines. A spiral stair goes down from the Undercroft to the Hall of Far
Doors (room 7), which opens on `winterfell` (north, levels 6 to 14),
`avalon` (east, 4 to 12), `narnia` (south, through the wardrobe, 2 to
9), and `midgaard` (west, classic ROM Midgaard, 1 to 15), and on down
to the Deeper Hall (room 8), which opens on `warcraft` (north, 8 to
16), `draenor` (east, 12 to 20), `diablo` (south, Tristram and the
cathedral, 14 to 24), and `starcraft` (west, 16 to 24). Below that the halls change their
rule: an area there runs from a low entrance up to its famous threats at
the level their source made them. The Hall of Legends (room 9) opens on
`velen` (north, The Witcher, 6 to 36), `moria` (east, 10 to 50),
`barovia` (south, Ravenloft, 12 to 45), and `nublar` (west, Jurassic
Park, 4 to 40); the Hall of Distant Stars (room 10) on `klendathu`
(north, Starship Troopers, 16 to 42), `arrakis` (east, Dune, 14 to 60),
`deathstar` (south, 15 to 55), `cadia` (west, Warhammer 40K, 22 to 65),
and down to `gauntlet` (the arcade game, 28 to 34). A hatch in the
Gauntlet's first room (2900) goes down to the Hall of Far Futures (room
11), which opens north on `foundation` (Asimov's Foundation, 20 to 45,
with the Mule as a raid boss for a full group); its other three doors
are sealed, kept for areas to come. Five farming areas
keep a tight band, crowded rooms, a two-minute reset, and plain drop gear
at every level: `ratcellar` (2 to 5, below Midgaard's Grunting Boar Inn),
`murloccoast` (8 to 12, west of Azeroth's wheat fields), `deaditecrypt`
(14 to 18, below Barovia's graveyard), `cowlevel` (20 to 26, through a
locked portal in Tristram whose key is Wirt's leg, which Wirt sells), and
`gauntlet`. From Tristram's north road a road runs east into Diablo
II, five areas in a chain, each opening on the next: `rogue` (the Rogue
Encampment and Andariel, 22 to 30), `lutgholein` (by Warriv's caravan;
the desert and Duriel, 28 to 38), `kurast` (by Meshif's ship; the
jungle, Travincal, and Mephisto, 36 to 46), `pandemonium` (down
Mephisto's red portal; the Pandemonium Fortress, the Hellforge, and the
Chaos Sanctuary's seals, 44 to 52), and `harrogath` (through Tyrael's
portal; Mount Arreat and Baal, 50 to 60). Each area's README.md carries
its map.
