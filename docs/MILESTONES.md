# Milestones

Each milestone is a vertical slice with a done criterion you can demonstrate
with two MUD client windows. Game rules deliberately do not appear until
milestone 8. Status: `[ ]` not started, `[~]` in progress, `[x]` done.

## 1. Skeleton and decisions `[x]`

Repo, Go module, config file, structured logging, one static binary, a run
target. Decisions recorded in `DECISIONS.md`.

**Done:** `make build && bin/urth` starts, logs its configuration, and exits
cleanly on SIGINT/SIGTERM.

## 2. Walking skeleton `[x]`

Thin TCP line listener that tolerates telnet negotiation bytes, one goroutine
per session, a single world goroutine with a fixed tick, ROM-style command
table with prefix abbreviation, rooms and exits loaded from YAML.
Commands: `look`, `exits`, `north/south/east/west/up/down`, `say`, `who`, `quit`.

**Done:** two clients walk around and see each other's speech.

## 3. Output and messaging model `[x]`

Structured events rendered to text at the transport edge. Perspective
messages ("You hit" / "Bob hits"). Per-tick output buffering. Prompts. Color
tokens. A minimal WebSocket transport to prove the core is transport-agnostic.

**Done:** the same session works over Telnet and a bare web page.

## 4. Persistence, accounts, and copyover `[x]`

Accounts with bcrypt passwords. Character creation as an explicit state
machine. Player save/load as YAML. Autosave. Clean shutdown. Copyover: re-exec
the binary while handing off live sockets.

**Done:** a `copyover` command restarts the binary with nobody disconnected.

## 5. Objects and mobs, no rules `[x]`

Items in rooms, inventory, and equipment slots. `get`, `drop`, `put`, `give`,
`wear`, `wield`, `remove`, `inventory`, `equipment`. Mobs that stand and
wander. Area files with resets and respawn timers.

**Done:** a 20-room area repopulates on schedule. No `kill` command exists yet.

## 6. Scripting layer and the rules seam `[x]`

Embed goja. Expose actors, rooms, and items to scripts. Define hook points:
damage, hit resolution, cast, tick, level. Hot reload of scripts. A headless
bot harness that drives the server. A `simulate` command that runs N fights.

**Done:** a script returning damage of 1 drives a placeholder combat round,
and editing that file changes the outcome without a restart.

## 7. Builder tooling `[x]`

`goto`, `at`, `stat`, `load`, `purge`, `force`, `restore`, `transfer`,
`peace`, and `reload area <name>` / `reload world`, which re-read every
area from disk, validate the whole set, and re-point live players, mobs,
and items at the new rooms and prototypes.

**Done:** content can be built while playing.

## 8. Rules, in scripts `[~]`

Built against `RULES.md`, in four steps, each leaving the simulator
runnable.

1. **Engine widenings** `[x]` Item spread, speed, verb, level, baseline,
   and effects; mob natural attack, armor, health and xp overrides;
   `itemBaseline` and `mobBaseline` hooks at load and reload; the swing
   meter for fractional speed; effects on characters and item instances
   with persistence and round lifetimes; stat points and `train`;
   `deathRules`; corpses; the simulator reports variance.
2. **Combat core in scripts** `[x]` Six stats with linear multipliers,
   baselines by level, spread and stat band, dodge, block, asymptotic
   reduction, level multiplier, regeneration, the hybrid experience
   curve, death cost, and the first effect kinds (stat, crit, block,
   dodge, attacks). First simulator run recorded in `RULES.md` 4.5.
3. **Progression** `[x]` Feat list in the rules with a `feat` command,
   picks granted by `onLevel`, a balance area with one plain mob per
   level, and `simulate fighter:N` for a level-N character in the
   rules' standard kit. The 4.5 gap row is measured; its finding on
   baseline shape is open in `RULES.md`. Skills and effectiveness
   ratings (`RULES.md` 7.4) are designed, not built.
4. **Magic and groups** `[x]` Groups, target switching, many-on-one, safe
   rooms (4.7). `spellList` and `resolveCast`; `cast` with cast time,
   interruption, committed materials, cooldowns, and engine-built target
   sets; `consume` for totems and `sacrifice` at temples; material and
   totem item types; the 6.5 spell list in `rules.js` with saves and
   scaling; the starter area carries two totems, a shrine, a sacrifice,
   and farmable materials with hints. Skills with effectiveness
   ratings (7.4) followed on 2026-09-25: Kick, Bash, Twin Strike, one
   action per round, ratings improving with use and persisting. Not
   built: mob casting, the simulator using skills or spells, general
   effect `state` write-back beyond ratings.

**Done:** a player in starter gear fights an even mob to the 4.5 targets,
and editing a baseline function changes every item's numbers without a
restart.

## Deferred until after milestone 8

Web admin, i18n, GMCP, MCCP, Discord, mapper, quests, shops, boards.
