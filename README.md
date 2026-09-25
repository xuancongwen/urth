# Urth

A small, fast MUD server in Go: ROM 2.4-style commands, YAML content, and
game rules in scripts that reload live.

## Run

    make build          # static binary in bin/urth
    bin/urth            # serves config.yaml; Ctrl-C to stop
    make run            # same, from source
    make check          # gofmt, vet, tests, content check
    make content        # content check only

| Port | What | Config key |
|---|---|---|
| 4000 | telnet, any MUD client | `server.telnet_addr` |
| 4001 | browser client at `/` | `server.websocket_addr` |
| 4002 | builder page, dev only | `server.builder_addr` (empty = off) |

Subcommands:

    urth check [-config path] [-quiet]      # load and lint data/, no server
    urth admin [-config path] <cmd> [name]  # list show promote demote passwd deny allow delete

`urth admin` edits player files on disk, for when nobody with admin can
log in. For a character who is online, use the in-game command; the
server rewrites the file at its next save.

## Playing

Type a new name to create a character. The first character on a server
is an admin. `help` lists commands; `help <command>` explains one.
Prefixes work (`n`, `inv`, `wie`). Targets take ROM forms: `sword`,
`2.sword`, `all`, `all.sword`.

| Group | Commands |
|---|---|
| Movement | north east south west up down look exits scan open close |
| Objects | get drop put give wear wield hold remove inventory equipment |
| Combat | kill flee consider assist skills |
| Magic | cast spells consume sacrifice |
| Groups | follow group gtell |
| Talking | say (or `'`) chat yell |
| Character | score train feat practice quest who color save password quit |
| Builder (admin) | goto at stat load purge force restore transfer peace reload simulate copyover shutdown |
| Admin | promote demote passwd deny allow users |

Admin commands work on offline characters too. `deny` drops the session
and refuses login until `allow`.

## Rules

Every game number comes from `data/scripts/rules.js` through the hooks in
`docs/RULES.md` section 1. Edit it while the server runs; it reloads on
the next round.

## Building

An area is `data/world/<area>/` with `area.yaml`, `rooms/`, `items/`,
`mobs/`, and `resets.yaml`: one entity per file, vnums unique across all
areas. Resets run at boot and every `interval_seconds`. Numbers on items
and mobs are stored, never interpreted; rules do that.

The loop is edit a file, then `reload area <name>` in game (or save, with
the builder page running). A file that fails validation leaves the old
world up. Tools around the loop:

- **Schemas** in `data/schema/` for every content file. `.vscode/settings.json`
  maps them for VS Code's YAML extension; other editors see `data/README.md`.
  A test keeps them in step with the Go structs.
- **`urth check`** lints without a server. Errors: unreachable rooms.
  Warnings: one-way or mismatched exits, unplaceable rooms, prototypes no
  reset places, resets into another area, overlapping vnum ranges. Deploys
  run it first. `detached: true` in `area.yaml` exempts an area.
- **Builder page** at `server.builder_addr`: each area drawn whole, by
  level; live contents; problems, resets, items, mobs; each file's YAML;
  reload on save. No login, so loopback only and unset in production.

In game, `stat` shows vnums; `goto`, `at`, `load`, and `purge` place and
inspect without leaving.

## Layout

| Path | Holds |
|---|---|
| `cmd/urth/` | server, `check`, `admin` |
| `cmd/urthbot/` | headless client for scripts and tests |
| `internal/world/` | the single-goroutine game loop, players, command table |
| `internal/content/` | loads rooms, items, mobs, resets; validates; lints |
| `internal/room/` `item/` `mob/` `reset/` | the content types |
| `internal/script/` | goja engine, hot reload, time budget |
| `internal/store/` | player records, bcrypt passwords |
| `internal/builder/` | builder page and file watcher |
| `internal/telnet/` `web/` | transports; `web/` embeds the browser client |
| `internal/session/` `output/` | transport contract; messages, color, renderers |
| `internal/copyover/` | restart in place with sockets handed over |
| `internal/config/` `limit/` `version/` | config, flood limits, build metadata |
| `data/` | world, scripts, schemas, players (gitignored) |
| `docs/` | `MILESTONES.md` `DECISIONS.md` `RULES.md` `DEPLOY.md` `COORDINATE-PORT.md` |
| `deploy/` | LXC setup and deploy scripts |

## License

MIT. See `LICENSE`.
