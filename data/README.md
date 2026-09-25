# data/

Root for everything the server reads and writes at runtime.

Planned layout (filled in by later milestones):

- `world/<area>/`  `area.yaml`, `rooms/*.yaml`, `items/*.yaml`, `mobs/*.yaml`, `resets.yaml`
- `players/`  saved accounts and characters, one YAML file each (milestone 4, gitignored)
- `scripts/`  goja JavaScript rule hooks; `rules.js` is the live rule set (docs/RULES.md section 1 lists the hooks)
- rooms may carry `position: {x, y, z}` to anchor the area map the web client draws; most rooms are placed by walking exits from an anchor, and the server logs any room whose exits contradict the grid so a builder can anchor it (`internal/room/layout.go`)
- `banner.txt`  the sign-in screen, with `{Y}`-style color tokens; edit freely, no rebuild needed

Areas under `world/`: `start` (the armory, hall, arena, and colleges: a
test area), `balance` (one plain mob per level for the simulator,
unreachable on foot), and `solace` (a town in the vallenwoods, reached by
the stair under the armory). Below the stair, the Undercroft of Doors
(room 6) opens on four more: `shire` (west; Hobbiton, Bywater, Maggot's
farm, the Woody End and the Old Forest, levels 1 to 10), `swordcoast`
(east; Candlekeep, the Friendly Arm, Beregost, Nashkel and its mines,
levels 2 to 9), `icewind` (north; Bryn Shander, Lonelywood, the tundra,
and the dwarven mines under Kelvin's Cairn, levels 3 to 10), and
`drevlin` (south; the Gegs, the Kicksey-winsey, and an elven watership,
levels 3 to 9). `underdark` (levels 8 to 18, the drow and Blingdenstone)
has no door of its own: it is reached down the shaft at the bottom of the
Icewind Dale mines. Each area's README.md carries its map.
