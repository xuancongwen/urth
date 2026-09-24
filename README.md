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
- `internal/telnet/`  line-oriented TCP transport, tolerant of telnet negotiation
- `internal/web/`     WebSocket transport and the embedded browser client
- `internal/room/`    the static map, loaded from `data/world/<area>/rooms/*.yaml`
- `internal/world/`   the single-goroutine game loop, players, and command table
- `internal/version/` build metadata (set by the Makefile)
- `data/`            world, players, scripts
- `docs/`            `MILESTONES.md` and `DECISIONS.md`

## License

MIT. See `LICENSE`.
