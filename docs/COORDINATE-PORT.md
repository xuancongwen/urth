# Porting the core to a coordinate-based browser game

An analysis, written 2026-09-25 against commit 2950cd5, of what it would
take to fork this codebase into a real-time browser game over WebSocket:
positions instead of rooms, a tile map instead of exits, state deltas
instead of prose. It records which parts of the engine carry over, which
parts have to be replaced, and what the server would newly have to bear.
Nothing here is built; it is a map for a fork.

## 1. Summary

The rule layer, effects, items, persistence, scripting, and the simulator
are already coordinate-agnostic and port nearly untouched. Three things
have to change: the spatial primitive (`Character.Room`), the combat and
targeting orchestration built on it, and the output model, which is
English prose at roughly 325 call sites. Movement is wholly new code. The
browser client is the largest single piece and sits outside the Go server.

## 2. What ports as-is

**The rules seam.** `data/scripts/rules.js` (636 lines, about forty hook
functions) is pure functions of character and item snapshots. No hook
reads position. The `view()` snapshot in `internal/world/rules.go` does
pass a `room {vnum, name, area}` field, but nothing in the rules reads it,
so it can become `zone {id, name}` or be dropped. Attack resolution,
derived stats, regeneration, experience curves, feats, spells, skills,
money, and item and mob baselines all survive unchanged.

**Effects.** `internal/effect` is a kind, params, mutable state, and a
lifetime in rounds. The engine stores and counts; the rules interpret. No
spatial awareness anywhere.

**Items, equipment, money, trainers.** Prototypes and instances, the
equipment slots, inventory, containers, decay, the wallet, and the trainer
flow are the same. Only "what lies here" lookups (ground items keyed by
room vnum) change.

**Script engine and simulator.** `internal/script` (goja, hot reload on
mtime, 50 ms budget) and `internal/world/simulate.go` (seeded fights on
detached characters) need nothing. The simulator stays the balance tool.

**Accounts, login, persistence, copyover.** The login state machine,
bcrypt on a helper goroutine, YAML player files, autosave. The saved
position becomes `{zone, x, y}` instead of a vnum. The token-based
WebSocket reconnect across copyover (DECISIONS D14) is already the right
model for browser clients.

**The world loop.** One goroutine, a fixed tick, one queued command per
player per tick, round logic every N ticks, batched output per player. This
holds up for server-authoritative movement. The 100 ms tick is already in
the right band for a 10 Hz simulation; the 2 s round stays as the combat,
casting, cooldown, and regen clock.

**The transport contract.** `internal/session` gives the world a
connection ID, an outbound queue, and connect and disconnect events. A
WebSocket transport exists in `internal/web`. Both stay; what flows
through them changes (section 3.5).

## 3. What has to change

### 3.1 Space

`Character.Room *room.Room` is the spatial primitive. About 170 sites in
`internal/world` reference it, concentrated in `builder.go`, `group.go`,
`commands.go`, `combat.go`, `character.go`, `magic.go`, and `login.go`.
Every "who is here" question goes through one of three helpers:

- `playersIn(r)`: a linear scan of all players filtered by room.
- `charactersIn(r)`: players in r, then the mobs from `w.rooms[vnum]`.
- `contents(r)`: the per-vnum `roomContents{items, mobs}` map.

Replacement:

- A `Position{Zone, X, Y}` on the character, and a spatial index per zone
  (uniform grid buckets are enough; a quadtree if maps are sparse).
- The three helpers become `charactersWithin(pos, radius)`,
  `itemsWithin(pos, radius)`, and so on. Callers that mean "in melee
  reach", "in view", or "in this spell's radius" pass the matching radius
  rather than a room.
- Keep rooms alive at a coarser grain as **zone regions**. A zone is one
  map; a region is a rectangle or polygon inside it carrying the flags
  rooms carry today: `safe`, `dark`, `temple`, and the area name. Then
  `Room.Safe()` becomes `regionAt(pos).Safe()`, area resets and
  `reload area` keep their meaning, and builders keep a familiar unit.

### 3.2 Movement

Wholly new; nothing to port. What goes away: cardinal exits and their
validation in `internal/room`, the six direction commands, `flee` through
a random exit, and `wanderMobs` stepping through exits in `resets.go`.

What replaces it:

- The client sends an **intent** (a direction vector, or a click target);
  the server integrates position each tick against a tile map's collision
  layer using a speed stat. The client never sends a position.
- Mobs get steering or grid pathfinding (A* over the collision layer),
  an aggro radius, and a leash back to their spawn point.
- Content: Tiled JSON maps (tile layers, a collision layer, an object
  layer for regions and spawn points) replace `data/world/<area>/rooms`.
  `resets.yaml` becomes spawn tables keyed by spawn point.
- Builder commands (`goto`, `at`, `transfer`, `load`) take a zone and
  coordinates instead of a vnum.

### 3.3 Combat and targeting

Combat is orchestration around the hooks, so the refactor is contained.
In `combat.go` and `magic.go`:

- `violence()` drops a target when `def.Room != c.Room`; this becomes a
  melee range check, and a target out of reach means "close the gap"
  for a mob or "no swing this round" for a player.
- `tickCasting` drops targets who left the room; this becomes a range
  check at completion, and moving may interrupt a cast (a rules choice).
- `findCharacter(room, self, "2.guard")` resolves a typed keyword. The
  client will click or tab-target, so targeting is by entity ID. Keep the
  keyword path for the text command channel if it survives.
- `castTargets` builds target sets by the spell's `target` word: single,
  area, self, ally, group. `hostilesIn` is "everyone here not grouped
  with me" and `groupIn` is "my group, here". These become radius queries
  centred on the caster or on the target point. The `Spell` and `Skill`
  structs gain `range` and `radius` fields, which are additive in
  `rules.js` and default to melee reach.
- Groups (`group.go`) work by pointer and carry over. `enemiesOf` (my
  target plus everyone targeting me) drops its room filter for a radius.
- `die`, corpses, experience sharing, and respawn are unchanged except
  that the corpse lands at a position and respawn goes to a zone
  coordinate rather than `world.start_room`.

### 3.4 Output

This is the largest engine cost and it is not spatial. The world emits
`output.Message{Type, Text, Data}`; only `Room` (RoomData) and `Prompt`
(a stats map) carry structured data. Everything else is English:

- 101 `act()` calls, each with toChar / toVict / toNotVict / toRoom
  perspectives, delivered by iterating `playersIn(actor.Room)`.
- About 225 direct `Send()` calls with formatted text.

A browser client needs state deltas, not sentences. Replacement:

- New message types on the existing envelope: `entity.spawn`,
  `entity.despawn`, `entity.move`, `combat.swing` (hit, dodge, block,
  miss, crit, damage, verb), `effect.apply`, `effect.expire`,
  `stats` (health, mana, tnl, level), `inventory`, `chat`. The
  `wireMessage` in `internal/web/conn.go` currently ships both `text`
  and `html` per message; that duplication goes.
- **Interest management**: `act()`'s toRoom becomes "everyone within
  view radius of the actor", and `entity.move` for an entity is sent
  only to players whose view contains it. Each player's batch per tick
  is the set of deltas in their view.
- The prose is not lost: the client can render "Your slash hits a guard.
  [12]" from a `combat.swing` event, so the MUD feel can survive as a log
  panel. Doing it client-side means the server never formats text again.

The mechanical work is replacing each of the 325 sites with a typed emit.
The perspective logic (who sees what) moves into the interest filter.

### 3.5 Input

`session.Input{Line}` is one typed line. Movement needs a structured
command: `session.Command{Kind, Payload}` as a union alongside `Input`.
The ROM command table in `commands.go` can stay for chat, `who`, admin,
and builder commands, so a text channel and an intent channel coexist on
one socket. `maxEventsPerTick` and one-command-per-player-per-tick already
bound the text side; movement intents need their own per-player cap.

### 3.6 Light and vision

`light.go` gates `look` on room darkness and a carried light. In a tile
world this becomes a per-region ambient level and a light radius on the
character that shrinks the view radius, feeding straight into interest
management rather than into a text check.

## 4. Server requirements this incurs

| Concern | Today | Coordinate game |
|---|---|---|
| Per-tick work | Commands and flush only | Integrate every moving entity, update the index, build each player's view |
| Outbound traffic | Bytes, only on events | Position deltas for every visible entity each tick |
| Wire format | JSON array of {type, text, html, data} | Compact JSON or binary; sequence numbers; deltas against the last acked state |
| CPU, a few hundred players | Near idle | Roughly one core |
| Beyond that | Not needed | Shard by zone, one goroutine per zone; cross-zone groups and chat need a bus |
| Client | A bare page rendering HTML spans | Canvas or WebGL app with tilesets and sprites |

Consequences:

- **Bandwidth.** Twenty visible entities at 10 Hz and about twenty bytes
  each is around 4 KB/s per player. Delta compression and only-on-change
  sends keep it there; naive full-state sends do not.
- **Latency hiding.** The client predicts its own movement and
  interpolates others. That requires the server to timestamp and
  sequence updates and to echo the last processed input sequence.
- **Validation.** The server integrates movement, so speed hacks are
  rejected by construction, but intents must be rate-limited per player
  and checked against the collision layer every tick.
- **TLS and a reverse proxy** in front of the WebSocket listener. The
  listener-handoff copyover (D14) works behind a proxy because the
  browser reconnects with a token.
- **Static assets.** The embedded FS in `internal/web` still works for the
  client bundle; tilesets and sprites are better served from a CDN.
- **Hosting.** Still one small VM, one static binary, no database at this
  scale. YAML player files at a five-minute autosave remain adequate;
  position can be saved more often since it is one line.
- **Copyover.** Movement state is only positions and intents, both
  serializable, so the state file grows but the mechanism stays.

## 5. Suggested order for a fork

1. Add `Position` and the spatial index; make the three lookup helpers
   radius-based while rooms still exist as regions. Everything keeps
   working with every room a one-cell region.
2. Replace the exit graph with a tile map loader and server-side
   movement. Mobs stand still at first.
3. Introduce the typed output events and interest management behind
   the same `output.Message` envelope; render them to text in the
   existing web page so the game stays playable throughout.
4. Convert combat and casting to range and radius; add the fields to
   `rules.js`.
5. Mob steering, aggro, and leash.
6. The real client.

Steps 1 through 4 keep the simulator and the telnet listener alive, which
is worth preserving as long as possible: the balance loop of edit,
simulate, read is the most valuable thing the current codebase has.

## 6. Rough proportions

- Rules, effects, items, persistence, scripting, simulator: reuse, near
  total.
- Combat, casting, groups: refactor, about a week.
- Typed output and interest management: rewrite of the 325 emit sites,
  one to two weeks.
- Space, movement, collision, pathfinding: new, a few weeks.
- Content pipeline (Tiled maps, spawn tables, builder commands): new,
  a week.
- Browser client: new, the largest item, outside the server.
