# dun

`dun` (from Gaelic for an ancient or medieval fort) is an API-only Rails 8 backend for an async multiplayer medieval-fantasy strategy game designed for developers during workday micro-idle moments. The CLI client and any future integrations consume the JSON API exposed by this backend.

## Documentation

- **Product overview** — [PRODUCT.md](PRODUCT.md): what the game is, who it's for, core mechanics at a glance.
- **Game design (source of truth)** — [docs/dun Game Design Document.v3.md](docs/dun%20Game%20Design%20Document.v3.md).
- **HTTP API contract** — [docs/openapi.yaml](docs/openapi.yaml): OpenAPI 3.1 spec for everything implemented so far.
- **API tutorial** — [docs/tutorial.md](docs/tutorial.md): hands-on walkthrough of the API from the Admin and Player perspectives.
- **Backend architecture** — [docs/architecture/README.md](docs/architecture/README.md): system overview and per-phase chapters.
- **Implementation roadmap** — [TODO.md](TODO.md).
- **Contributor guide** — [CLAUDE.md](CLAUDE.md): conventions, workflow, code style.

## Stack

- Ruby 4.0.4, Rails 8.1.3 (`--api` mode)
- PostgreSQL 18+
- Solid Queue (background jobs), Solid Cache, Solid Cable
- Auth: magic link + 90-day Bearer `ApiKey` (polymorphic `Player` / `Admin`)
- Pagination: [pagy](https://github.com/ddnexus/pagy)
- Data migrations: [data_migrate](https://github.com/ilyakatz/data-migrate)
- Logs: [lograge](https://github.com/roidrage/lograge) JSON formatter
- Tracing/metrics: OpenTelemetry SDK + auto-instrumentation
- Testing: Minitest + Mocha + WebMock + Factory Bot
- Dev mail: `letter_opener`

## Prerequisites

- Ruby 4.0.4 (use your version manager of choice — `rbenv`, `asdf`, `chruby`)
- PostgreSQL 18+ running locally
- `foreman` (used by `bin/dev` to run web + jobs together)

## Setup

```bash
bin/setup
```

This installs gems, prepares the database, clears old logs, and starts `bin/dev`. Pass `--skip-server` to set up without launching, or `--reset` to drop and recreate the database.

### Required environment variables

The app fails loudly on missing envs — no silent fallbacks. Minimum set for development:

| Variable | Purpose |
| :---- | :---- |
| `ADMIN_BOOTSTRAP_EMAIL` | Seeded bootstrap admin email |
| `ADMIN_BOOTSTRAP_NAME` | Seeded bootstrap admin display name |
| `MAGIC_LINK_FROM_EMAIL` | `From:` address for magic-link emails |

Optional:

| Variable | Default | Purpose |
| :---- | :---- | :---- |
| `PORT` | `3000` | Puma port |
| `RAILS_MAX_THREADS` | `3` | Puma threads / DB pool |
| `JOB_CONCURRENCY` | `1` | Solid Queue worker processes |
| `OTEL_SERVICE_NAME` | `dun-backend` | OpenTelemetry service name |
| `RAILS_LOG_LEVEL` | `info` | Production log level |

## Development

```bash
bin/dev                  # starts web + Solid Queue worker via foreman
bin/rails db:migrate     # schema migrations
bin/rails data:migrate   # data migrations (data_migrate gem)
bin/rails db:seed        # idempotent; requires ADMIN_BOOTSTRAP_* envs
```

In development, magic-link emails are captured by `letter_opener` and rendered in the browser.

## Testing

```bash
bin/rails test                                                       # full suite (parallel)
bin/rails test test/models/example_test.rb                           # single file
bin/rails test test/models/example_test.rb -n test_name              # single test
```

Conventions:

- Factory Bot only — no fixtures. Factories live in [test/factories/](test/factories/).
- HTTP is stubbed with WebMock; net-connect is disabled (`localhost` allowed).
- Controller tests use `authenticate_as_player(player)` / `authenticate_as_admin(admin)` from [test/test_helpers/session_test_helper.rb](test/test_helpers/session_test_helper.rb).

## API surface

All routes mount under `/v1/...`. The player surface lives at the root of that namespace; admin endpoints sit under `/v1/admin/...`. Responses are JSON only. Errors use the envelope:

```json
{ "error": { "code": "string_code", "message": "human readable", "retry_after": 30 } }
```

Every response echoes `X-Request-Id`, which appears in lograge logs and OpenTelemetry traces. The full contract — every endpoint, request shape, and response shape — is in [docs/openapi.yaml](docs/openapi.yaml).

### Auth flow

1. `POST /v1/auth/magic_link` (or `/v1/admin/auth/magic_link`) with `{email}` — mailer enqueues a magic link.
2. `POST /v1/auth/exchange` (or `/v1/admin/auth/exchange`) with the raw token — receive `{api_key, expires_at}`.
3. Subsequent requests carry `Authorization: Bearer <api_key>`. The 90-day expiry refreshes on each authenticated request.

A player ApiKey at an admin endpoint 401s, and vice versa.

## Status

Phases 0–10 are complete. Phase 11 (Anti-Abuse: Reports, Rate Limits, Raid Cap) is next. See [TODO.md](TODO.md) for the full roadmap and [docs/architecture/](docs/architecture/) for per-phase implementation notes.
