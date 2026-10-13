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
browser page reconnects with it (on the player's click; the page never
connects unasked) and the world re-attaches them without a login.
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

## D17. Hooks consume items by returning their ids

D16 says the engine applies and never decides, so scripts have no handle
that destroys an item. Casting (RULES 6.2) and unlocking a school of magic
(RULES 6.1) both need to consume inventory items, so a hook result may
carry a `consume` list of item ids. The engine checks every listed item is
in the actor's inventory, removes them, and only then applies the rest of
the result; a missing item rejects the whole result so nothing half-fires.
This keeps D16 intact: the script still only returns values. Lands with
`resolveCast` and the `cast` command in milestone 8.

## D18. Mana is kept but not required

The material-based casting cost in RULES 6.2 makes a regenerating pool
redundant, but removing `mana` from the engine would be a one-way door
before the material system is proven. The fields stay; rules set
`manaMax` to zero and no cast checks it; a zero pool is hidden from the
prompt. If materials fail, mana is the fallback with no engine change.


## D19. Builder tooling is a page over the files; admin is in-game plus a CLI

The dev loop that produced the world is bulk file authoring followed by
reload (D17), and deploys overwrite `data/world` on the host from the
checkout. A web editor on the server would therefore make the server the
source of truth and lose git, so there is none. Instead the builder page
(`internal/builder`) runs on the dev machine against the checkout, draws
what the loaded world looks like, shows the content check's findings, and
reloads on save; editing stays in the editor with JSON schemas for
completion and validation. Position-dependent builder commands stay in
game because they mean "here". Account administration is in-game
commands for the common case and `urth admin` on the host for lockouts,
which together cover one operator with ssh; a status dashboard on
production waits until there is a second person who needs it.

## D20. The builder page edits the checkout's files

This supersedes D19's "no web editor". D19's worry was a server that
becomes the source of truth and loses git; the builder page never runs
on the host, so writing from it changes the same checkout an editor
would, and git sees every change. The page saves any content file as
text and does the common room work structurally: dig a room in a
direction, link two rooms both ways, unlink, delete. Structural edits go
through yaml.v3 nodes, so comments, key order, and styles survive and a
diff shows only the lines that changed. Every change is loaded in a
scratch copy of the world first and refused if it would not load (a
text save can be forced); saves carry the hash the file was read at and
are refused if it changed on disk meanwhile. Mutating requests need the
page's own header and a Host that is an address or localhost, which
turns away cross-site forms and DNS rebinding; the page still has no
login and stays off in production. Items, mobs, and resets are edited as
text; there are no forms for them.

## D21. An admin page on loopback, reached over ssh

D19 put off a production dashboard until a second person needed one; a
page is now wanted for looking through characters. It runs in the
server on `server.admin_addr` and refuses to listen anywhere but
loopback, so reaching it on the host means an ssh tunnel, and ssh is the
login. Every request must carry an address or localhost as its Host,
which stops DNS rebinding from reading characters, and every change
needs the page's own header, which stops cross-site forms. Account
changes go through the same world methods as the in-game commands, so
an online character is changed in place and saved, never overwritten at
the next autosave. Deletion is refused for someone online and moves the
file to `players/deleted/`, as `urth admin delete` does.

## D22. The admin page opens to the LAN behind an admin sign-in

This changes D21. The page may now listen on any address. A request from
loopback, which covers the machine itself and an ssh tunnel, still needs
no login. A request from anywhere else must sign in with the name and
game password of an admin character that is not denied; there is no
separate admin account to keep in step. Sessions are in memory, last 12
hours from last use, and are re-checked on every request, so a demotion
or deny ends them at once. Five failed sign-ins from one address lock it
out for a minute. The cookie is HttpOnly and SameSite=Strict, changes
still need the page's header, and the Host check now also admits .local
names, which public DNS cannot serve, so mDNS names on the LAN work. The
page is plain HTTP, so a password crosses the LAN in the clear; TLS is
left for when the page has to leave the LAN, and ssh covers that today.

## D23. Spells cost mana; items only where the spell is about an item

This supersedes D18. Material-costed casting (RULES 6.2 as decided on
2026-09-24) made every cast an inventory check and kept a newly found
school unusable until its materials were farmed. The designer asked on
2026-10-11 to move to mana and keep item requirements only for spells
where the item is the point, such as enchantments and portals. D18 kept
`mana`, `manaMax`, and `manaDelta` in the engine for exactly this, so
the change is in the rules: `derivedStats` returns a real pool,
`onTick` refills it out of a fight, and each spell names a `mana`
cost that the engine checks and spends when the cast begins, alongside
any materials it still names. Mana does not refill in a fight, which
is what keeps a cast a decision (RULES 2.1, 6.3).

## D24. Dropped connections leave the character link-dead, and web clients resume

A phone that sleeps drops its WebSocket, and so do proxies that see no
traffic for a while; until now any drop removed the character at once,
so a player lost their place, and their fight, to a dark screen. Now a
character whose connection drops without a quit stays in the world for
`world.linkdead_minutes` (10 by default; 0 restores the old behaviour),
as in ROM: saved, shown as link-dead to `who`, and removed only if no one
comes back. On entering the game a web client is given a resume token
(a `session` message); reconnecting with it reattaches the same
character to the new socket, and every resume issues a new token. A
password sign-in reclaims a link-dead character the same way, which also
serves telnet. Signing in to a character that is playing elsewhere
reattaches it too, rather than copying some of its fields into a fresh
one, which lost silver, feats, and quest state.

This relaxes D14's rule that a page never connects unasked, in one case:
after an unexpected drop, both web clients resume by themselves as soon
as the page is visible again, and a page that loads holding a token
resumes. Copyover, restarts, quits, kicks, and idle hang-ups still wait
for the player. The web server also pings every 30 seconds, so a proxy
never sees a quiet socket, and a peer that cannot answer is closed and
left link-dead.

## D25. Spells are unlocked by discovery and then learned; feats are gated

RULES 6.1 makes magic something found, not a function of level, and
the designer asked on 2026-10-11 for many more spells, most of them
trainable, and for feats, skills, and spells to be gated by level and
stats. Both hold: a school's totem or a god's sacrifice still decides
whether a character has that magic at all, and within it a spell is
innate (the first ones, known on unlocking), taught by a trainer for
coin, or found on a scroll (the strongest of each school and god). A
learned spell is a permanent effect, like a learned skill, so the
player file needs nothing new. Feats gain trained-stat requirements
and, for caster feats, a branch; caster feats and gear read new effect
kinds in the rules, and the engine exposes casting cost, time, and
interruption through one optional `castCost` hook (RULES 6.6, 7.2).

## D26. Weapons ride a flatter curve; damage has kinds, and kinds are resisted

Every baseline grew by 1.10 a level, weapons included, so late weapons
hit for thousands and a character's power sat in one number. The
designer asked on 2026-10-12 for high damage to come from effects
stacking instead, with health left as it is. The curve is split: the
power unit (`unit` in the rules) keeps 1.10 and sizes health, mob
attacks, spells, procs, and added damage; a weapon's own damage grows
by 1.07, a fifth of the power unit by level 60, and affixes, procs,
feats, and spells make up the rest (RULES 5.2, 5.3). To make the mix
of that stack matter, every part of a hit has a kind and a `resist`
effect grades it, from vulnerability through resistance to immunity,
with immunity kept for mobs (RULES 4.8). Both are rules-script work;
the engine's share is carrying a kind on spells and procs and showing
resistances on `consider`.

## D28. Mobs that onTick leaves unchanged are not asked again until they change

An idle server spent about 5 percent of a core with nobody online
(story 1044): every round asked `onTick` about each of some 2,200 mobs,
and nearly all of them were at full health with nothing to do. The cost
grew with the world, not with play. Two fixes were weighed: letting
areas without players sleep, which needs a catch-up rule for what
happened while they slept and stops time for mob-on-mob fights and
damage over time; and remembering that a call did nothing. The engine
takes the second. When `onTick` changes nothing for a mob that is not
fighting, attacked, casting, cooling down, grouped, or carrying effects
of its own, the engine keeps a key of what the call could see (health,
mana, stamina and their maxima, level, position, room, prototype,
equipment, and the rules generation, which every script reload and
prototype resolution bumps) and skips the mob while the key holds. The
engine still decides nothing about the rules; it relies on one contract
written into the hook table: given the same view, an `onTick` that
changed nothing changes nothing again. An idle round fell from about
56 ms to about 1.3 ms.
