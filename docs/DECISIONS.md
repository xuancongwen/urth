# Decisions

Short records of choices that shape the codebase. Add a new entry rather than
editing an old one when a decision changes.

## D1. Language: Go

Connection count is not a MUD's bottleneck; the shared world tick is. Go gives
a goroutine per socket, a small static binary, sub-10 MB idle memory, and
second-long builds. Erlang's hot code loading is attractive but the actor model
fights a shared mutable world. Rust slows the iteration this project depends on.

## D2. One world goroutine

All game state is owned by a single goroutine driven by a fixed tick
(`timing.tick_ms`). Session goroutines only do I/O and hand lines to the world
over a channel. The world never blocks on I/O and never iterates every player
for a per-room event. This is the ROM/GoMud shape and avoids a class of
concurrency bugs.

## D3. Transport is thin and dictated by the world loop

The world needs three things from a transport: inbound lines tagged with a
session ID, an outbound byte queue per session, and connect/disconnect events.
The TCP listener accepts raw text lines and strips telnet IAC sequences; it
does not negotiate options. Nearly every MUD client works with this. WebSocket
is a second transport behind the same interface (milestone 3).

## D4. Structured events, rendered at the edge

Game code emits structured events. Rendering to text (ANSI, prompts,
perspective wording) happens in the transport layer. This keeps Telnet and a
future web client thin and keeps game logic free of formatting.

## D5. Data format: YAML, one file per entity

Rooms, items, mobs, players, and config are YAML. Directory layout borrows from
GoMud (`data/world/<area>/rooms/*.yaml`). Human-diffable, no database.

## D6. Script engine: goja (JavaScript)

Rules live in scripts, not Go, so balance changes never require a rebuild.
goja chosen over gopher-lua for editor tooling and familiarity; GoMud's script
API is a usable reference. Rule hook points are defined in milestone 6 with
placeholder implementations before any real rules exist.

## D7. Command vocabulary: ROM 2.4

Command names and abbreviation behavior follow ROM 2.4 (`kill`, `wear`,
`wield`, `recall`, prefix matching, `'` for say). Only the vocabulary is
copied; no ROM code is used, because the Diku/Merc/ROM license is
non-commercial and requires credits.

## D8. Copyover is designed in early (milestone 4)

Live iteration on engine code needs restart-without-disconnect. Sockets must
be inheritable across exec and session state must be serializable. Retrofitting
this is painful, so it lands before objects, mobs, or rules.

## D9. Rules are deferred to milestone 8

Everything through milestone 7 is rules-agnostic. Combat orchestration is built
against a stub that returns damage 1. Balance work starts only once a
simulator and hot-reloading scripts exist.

## D10. Perspective messages use ROM's act() convention

A message that differs by viewer is written as separate format strings, one
per perspective (`toChar`, `toVict`, `toNotVict`, `toRoom`), with `$n` and
`$N` substituted for the actor and victim. No verb conjugation engine. This
is simple, matches ROM builders' expectations, and leaves room for `$e`/`$s`
pronouns once characters have them.

## D11. Color is brace-token markup, escaped at the input boundary

Game text carries `{r}`-style tokens (see `internal/output/color.go`). Each
transport renders them: ANSI for terminals, `<span>` classes for the browser,
stripped when a player turns color off. Anything a player typed passes
through `output.Escape` before it is embedded, so players cannot inject
color or markup.

## D12. Output is batched per tick

A player's messages accumulate during a tick and are delivered as one
`output.Batch`, with the prompt appended last if there was output. One write
per player per tick, and the prompt can never land mid-output.

## D13. The character is the account

As in ROM, a player file (`data/players/<name>.yaml`) holds the name, the
bcrypt password hash, and the character's state together. There is no
separate account object. If multi-character accounts are ever wanted, the
record splits along an obvious seam. Password hashing and checking run on a
helper goroutine and post their result back to the world loop, so a login
never stalls the tick.

## D14. Copyover inherits telnet sockets; web clients reconnect with a token

Telnet sockets and both listening sockets are handed to the exec'd binary by
clearing close-on-exec and passing descriptor numbers in a state file. A
WebSocket connection carries framing state inside the library that cannot be
reconstructed after exec, so those clients receive a single-use token and the
browser page reconnects with it; the world re-attaches them without a login.
Sessions still at the login prompts are closed. The first character created
on a server is an admin so copyover and shutdown are reachable from day one.

## D15. Items and mobs are prototypes plus instances; the engine stores rule inputs blindly

Prototypes are YAML per vnum. Instances are created by resets or restored
from player files. Fields like weapon damage, armor defense, and mob stats
are kept on the prototype as plain numbers and maps that rule scripts read;
the engine never does arithmetic on them. Players and mobs share one
Character type for names, rooms, inventory, and equipment so that every
command and message works the same on both.

## D16. Rules are JavaScript functions with fixed signatures; the engine applies, never decides

Combat, regeneration, experience, and derived maxima are global functions
in `data/scripts/*.js` run by goja. The engine builds read-only snapshots,
calls a hook, and applies the returned numbers. Hooks have a 50 ms budget
and a safe default so a broken edit degrades rather than crashes. Files are
reloaded when their modification time changes, checked once per round, and
a failing set is reported once and left alone until it changes again. The
simulator runs the same hooks on detached characters with a seeded random
source, so balance work is a loop of edit, simulate, read.

## D17. Building is edit-file-then-reload, not in-game OLC

There is no online creation editor. Content is YAML edited outside the
game and pulled in with `reload area <name>`, which re-reads and validates
every area and re-points live state by vnum. This keeps one source of
truth on disk (and in git), avoids a second schema for OLC forms, and
makes a bad edit cheap: validation fails and the old world stays up. If
in-game editing is ever wanted, it can write the same files and call the
same reload.
