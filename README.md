# dun-cli

`dun-cli` is the terminal client for [`dun`](https://github.com/fguillen/dun),
an async multiplayer medieval-fantasy strategy game designed for the
30-second-to-5-minute windows between builds, deploys, and LLM responses.
Single binary, keyboard-driven, plays nicely with the terminal you
already have open.

## Status

Early scaffold. The CLI currently ships a splash screen and a `version`
command — enough to prove the build chain. See [TODO.md](TODO.md) for
the phased roadmap and [PRODUCT.md](PRODUCT.md) for positioning.

## Install

```bash
# Homebrew (Phase 14 — not yet shipped)
brew install fguillen/tap/dun-cli

# Or from source
go install github.com/fguillen/dun-cli/cmd/dun@latest
```

## Quickstart

`dun-cli` is an interactive shell in the spirit of `psql` or `mongosh`:
`dun login` authenticates and drops you into a `dun>` prompt where every
game action is a verb.

```bash
$ dun login alice@example.com
Magic link sent. Paste token: ************
Welcome, IronFist.

dun> servers
dun> world join spring-2026
dun> kingdom build barracks 5
dun> quit
```

See [PRODUCT.md](PRODUCT.md) "Interaction model" for the full UX
shape, and [TODO.md](TODO.md) for what's implemented per phase.

**Current state:** Phase 0 ships a placeholder splash at `dun tui`
(press `q` to quit) to prove the build chain. `dun login` lands in
Phase 2; the REPL shell lands in Phase 3.

## Configuration

- Config file: `$XDG_CONFIG_HOME/dun-cli/config.toml` (defaults to
  `~/.config/dun-cli/config.toml` on macOS/Linux).
- Logs (JSON): `$XDG_STATE_HOME/dun-cli/dun-cli.log` (defaults to
  `~/.local/state/dun-cli/dun-cli.log`).
- Credentials: OS keychain via `go-keyring`, with a `0600` config-file
  fallback when no keychain is available.

## API contract

The backend's HTTP API is the source of truth. Its public docs are
mirrored read-only into [docs/backend/](docs/backend/):

- [openapi.yaml](docs/backend/openapi.yaml) — the contract
- [tutorial.md](docs/backend/tutorial.md) — every player flow end-to-end
- [game-design.md](docs/backend/game-design.md) — mechanics (`§N.N` refs)
- [api-endpoints.md](docs/backend/api-endpoints.md) — flat endpoint index

Refresh the mirror with `./scripts/sync-backend-docs.sh` (set `DUN_REF`
to pin to a tag/sha). Then `make generate` regenerates the typed client.

## Development

```bash
make generate   # regenerate internal/api/gen/* from the OpenAPI spec
make build      # bin/dun
make test       # go test ./...
make lint       # golangci-lint run
make run        # go run ./cmd/dun tui
```

See [CLAUDE.md](CLAUDE.md) for the full contributor guide.

## License

TBD.
