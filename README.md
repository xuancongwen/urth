# Urth

A small, fast MUD server with a ROM 2.4-style command set and game rules
written in scripts so they can be changed live.

## Run

    make build      # static binary in bin/urth
    bin/urth        # reads config.yaml, Ctrl-C to stop
    make run        # same, from source
    make check      # gofmt, vet, tests

Then connect with any MUD client or `telnet 127.0.0.1 4000`, or open
http://127.0.0.1:4001/ in a browser.

## Layout

- `cmd/urth/`        entrypoint
- `internal/config/`  YAML configuration with defaults and validation
- `internal/session/` the contract between transports and the world
- `internal/output/`  structured messages, color tokens, ANSI/plain/HTML renderers
- `internal/item/`    item prototypes and instances, ROM-style targeting
- `internal/mob/`     mob prototypes
- `internal/reset/`   area repopulation schedules
- `internal/content/` loads rooms, items, mobs, and resets and checks references
- `internal/script/`  goja engine: loads `data/scripts/*.js`, hot reload, time budget
- `cmd/urthbot/`      headless client for driving a running server
- `internal/store/`   player records as YAML with bcrypt password hashes
- `internal/copyover/` restart-in-place state and descriptor handoff
- `internal/telnet/`  line-oriented TCP transport, tolerant of telnet negotiation
- `internal/web/`     WebSocket transport and the embedded browser client
- `internal/room/`    the static map, loaded from `data/world/<area>/rooms/*.yaml`
- `internal/world/`   the single-goroutine game loop, players, and command table
- `internal/version/` build metadata (set by the Makefile)
- `data/`            world, players (gitignored), scripts

## Playing

Create a character by typing a new name. The first character created on a
server is an admin.

Commands so far: look [thing | in container], exits, north/south/east/west/
up/down, say (or '), who, inventory, equipment, get [item [container] | all],
drop, put, give, wear, wield, hold, remove, kill, flee, score, color, save,
password, quit. Admin: goto, at, stat, load, purge, force, restore,
transfer, peace, reload [scripts | area <name> | world], simulate,
copyover, shutdown. Targets take ROM forms: `sword`, `2.sword`, `all`,
`all.sword`.

## Rules

Every game number comes from `data/scripts/rules.js` through the hooks
listed in `docs/RULES.md` section 1. Edit the file while the server runs;
it reloads on the next round. The shipped file is a placeholder.

## Building

An area is a directory under `data/world/` with `area.yaml`, `rooms/`,
`items/`, `mobs/`, and `resets.yaml`. One entity per YAML file, vnums
unique across all areas. Resets run at boot and every `interval_seconds`;
limits make them idempotent. Item and mob numbers such as damage, defense,
and stats are stored but never interpreted by the engine; rules do that.

Edit files in your editor, then `reload area <name>` in game. Players
standing in the area see the changes at once; anyone in a deleted room is
moved to the start room; a file that fails validation leaves the old world
running. `stat` shows vnums; `load`, `purge`, `goto`, and `at` place and
inspect things without leaving the game.
- `docs/`            `MILESTONES.md`, `DECISIONS.md`, `RULES.md`, `DEPLOY.md`, and `COORDINATE-PORT.md`
- `deploy/`          LXC setup and deploy scripts (`docs/DEPLOY.md`)

## License

MIT. See `LICENSE`.
