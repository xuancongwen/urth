# Milestones

Each milestone is a vertical slice with a done criterion you can demonstrate
with two MUD client windows. Game rules deliberately do not appear until
milestone 8. Status: `[ ]` not started, `[~]` in progress, `[x]` done.

## 1. Skeleton and decisions `[x]`

Repo, Go module, config file, structured logging, one static binary, a run
target. Decisions recorded in `DECISIONS.md`.

**Done:** `make build && bin/urth` starts, logs its configuration, and exits
cleanly on SIGINT/SIGTERM.

## 2. Walking skeleton `[ ]`

Thin TCP line listener that tolerates telnet negotiation bytes, one goroutine
per session, a single world goroutine with a fixed tick, ROM-style command
table with prefix abbreviation, rooms and exits loaded from YAML.
Commands: `look`, `exits`, `north/south/east/west/up/down`, `say`, `who`, `quit`.

**Done:** two clients walk around and see each other's speech.

## 3. Output and messaging model `[ ]`

Structured events rendered to text at the transport edge. Perspective
messages ("You hit" / "Bob hits"). Per-tick output buffering. Prompts. Color
tokens. A minimal WebSocket transport to prove the core is transport-agnostic.

**Done:** the same session works over Telnet and a bare web page.

## 4. Persistence, accounts, and copyover `[ ]`

Accounts with bcrypt passwords. Character creation as an explicit state
machine. Player save/load as YAML. Autosave. Clean shutdown. Copyover: re-exec
the binary while handing off live sockets.

**Done:** a `copyover` command restarts the binary with nobody disconnected.

## 5. Objects and mobs, no rules `[ ]`

Items in rooms, inventory, and equipment slots. `get`, `drop`, `put`, `give`,
`wear`, `wield`, `remove`, `inventory`, `equipment`. Mobs that stand and
wander. Area files with resets and respawn timers.

**Done:** a 20-room area repopulates on schedule. No `kill` command exists yet.

## 6. Scripting layer and the rules seam `[ ]`

Embed goja. Expose actors, rooms, and items to scripts. Define hook points:
damage, hit resolution, cast, tick, level. Hot reload of scripts. A headless
bot harness that drives the server. A `simulate` command that runs N fights.

**Done:** a script returning damage of 1 drives a placeholder combat round,
and editing that file changes the outcome without a restart.

## 7. Builder tooling `[ ]`

`goto`, `at`, `stat`, `load`, `purge`, `force`, and reload an area from disk.

**Done:** content can be built while playing.

## 8. Rules, in scripts `[ ]`

Combat first, then magic, then progression. Each balanced with the simulator
from milestone 6.

## Deferred until after milestone 8

Web admin, i18n, GMCP, MCCP, Discord, mapper, quests, shops, boards.
