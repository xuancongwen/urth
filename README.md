# Urth

A small, fast MUD server with a ROM 2.4-style command set and game rules
written in scripts so they can be changed live.

## Run

    make build      # static binary in bin/urth
    bin/urth        # reads config.yaml, Ctrl-C to stop
    make run        # same, from source
    make check      # gofmt, vet, tests

## Layout

- `cmd/urth/`        entrypoint
- `internal/config/` YAML configuration with defaults and validation
- `internal/version/` build metadata (set by the Makefile)
- `data/`            world, players, scripts
- `docs/`            `MILESTONES.md` and `DECISIONS.md`
