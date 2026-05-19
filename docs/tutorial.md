# dun-cli tutorial

This guide walks you through everything `dun-cli` can do today, end-to-end.

## 1. Overview

`dun-cli` is the terminal client for **dun**, an async multiplayer
medieval-fantasy strategy game. It is designed for workday micro-idle
moments: fast launch, snappy keyboard-only navigation, low cognitive
load.

The interaction model is a long-running **REPL prompt** — `dun>` — in
the spirit of `psql`, `mongosh`, and `redis-cli`. It is **not** a
full-screen TUI. Your terminal scrollback is preserved so you can
copy/paste the output of any command. Some verbs (the post-login
server picker, `profile set` with no flags) briefly pop a transient
selector or form, then drop you back at the prompt with the result
printed into scrollback.

Two layers of commands:

- **Top-level `dun` subcommands** are reserved for things that make
  sense *outside* an authenticated session: `dun login`, `dun logout`,
  `dun keys ...`, `dun account ...`, `dun version`.
- **REPL verbs** are everything else: `servers`, `server join`,
  `profile`, `player`, `help`, `where`, `whoami`, `quit`, …

You refer to game entities by their **name or slug** (`acme`,
`IronFist`), never by an internal ID.

## 2. Installation

There are no pre-built binaries yet — install from source.

Requirements:
- Go 1.26 or newer.
- `make`.

```
git clone https://github.com/fguillen/dun-cli
cd dun-cli
make build
```

`make build` produces a binary at `bin/dun`. Move or symlink it onto
your `$PATH` (e.g. `cp bin/dun /usr/local/bin/`) and you're done.

Other useful targets:

| Target | What it does |
|--------|--------------|
| `make build` | Compile `bin/dun`. |
| `make run` | Run the CLI directly from source (`go run ./cmd/dun`). |
| `make test` | Run the test suite. |
| `make lint` | Run `golangci-lint`. |
| `make generate` | Regenerate the OpenAPI client. |

## 3. Configuration

The CLI looks for an optional config file at `~/.dun/config.toml`:

```toml
base_url = "http://localhost:3000/v1"
```

If the file is missing, `base_url` defaults to
`http://localhost:3000/v1`. To point at a remote backend, just write
the file with your URL.

There are **no environment-variable overrides** — no `$DUN_HOME`, no
XDG variables. Everything lives strictly under `~/.dun/`.

Logging verbosity is controlled by a single flag on the `dun` command:

```
dun --log-level debug
```

Levels: `debug`, `info` (default), `warn`, `error`. Log output is
written to `~/.dun/dun-cli.log` as JSON, never to your terminal.

## 4. First login

Authenticate with a magic-link email:

```
$ dun login
Email: alice@example.com
Magic-link email sent. Check your inbox.
Token: 7f3a8b2c9d1e4f6a
Logged in as alice@example.com (expires 2026-08-17).
```

The `Token:` prompt expects the raw token string from the magic-link
email — not the full URL. Copy/paste it on the same line.

On success, the API key is persisted to `~/.dun/credentials` (TOML,
mode `0600`) and marked as the current credential. All subsequent
commands use it automatically.

Immediately after the login message, the CLI fetches your server
memberships and decides how to drop you into the shell:

- **No memberships yet** — the shell opens with no server scope. Use
  `servers` to see what's available and `server join <slug>` to join.
- **Exactly one membership** — that server is silently set as your
  current scope. The prompt is ready to go.
- **Two or more memberships** — a picker titled `Pick a server` opens.
  Arrow keys to move, `Enter` to select, `Esc` / `Ctrl-C` / `q` to
  skip. On selection you'll see `scope set to server "acme"`.

If the membership probe fails (network blip, backend down), login
still succeeds and you'll see:

```
(could not pre-fetch server memberships, continuing without picker)
```

## 5. The interactive shell

Running bare `dun` once you're logged in drops you straight at the
prompt. The first line you'll see is the connectivity probe greeting:

```
$ dun
Connected to http://localhost:3000/v1 as alice@example.com
dun>
```

### Line editing

- **Up / Down** — walk the persistent history (stored at
  `~/.dun/history`).
- **Ctrl-R** — reverse history search.
- **Ctrl-C** — abort the line you're typing. Doesn't quit the shell.
- **Ctrl-D** — exit cleanly (equivalent to `quit`).

### Tab completion

`Tab` completes verb names, sub-verbs (`server <Tab>` →
`server join`), flag names (`profile set --<Tab>`), and — where the
verb supports it — dynamic argument values from the backend. For
example, `server join <Tab>` completes slugs of servers you can still
join.

### Error format

Every command failure prints exactly one styled line:

```
error: <message> (code=<code>, request_id=<id>)
```

When reporting a bug, copy the `request_id` — it correlates with the
backend's logs.

## 6. Built-in shell commands

These verbs are part of the shell's contract and have no backend
dependencies (apart from `version` which is purely local).

### `help` / `help <verb>`

```
dun> help
commands:
  ...

Tab completes verbs, slugs, and flags. Ctrl-D exits.
```

With an argument, prints detailed usage for one verb:

```
dun> help profile
profile
  Read or update your per-server profile
  usage: profile <show|set> [flags]

  subcommands:
    show         Show your profile on the in-scope server
    set          Update your handle and/or real name on the in-scope server
```

### `quit` / `exit`

Either command — or `Ctrl-D` — exits the shell cleanly. The session
state is saved on the way out.

### `clear`

Clears the screen (sends `\033[H\033[2J`).

### `version`

Prints the CLI version. Same value as `dun version`.

### `whoami`

Shows the active credential:

```
dun> whoami
alice@example.com
  base_url: http://localhost:3000/v1
```

### `where`

Shows your current `(server, world, kingdom)` scope:

```
dun> where
  server:  acme
  world:   (none)
  kingdom: IronFist
```

> The `world:` and `kingdom:` lines exist in the shell already, but
> only `server:` is set by anything in today's shipped verbs. The
> kingdom handle is recorded when you set your profile.

## 7. Servers

### `servers` — list servers you can see

Splits results into two sections. Empty sections print `(none)`.

```
dun> servers
Member of:
  acme                Acme

Eligible to join:
  beta                Beta
  gamma               Gamma
```

### `server join <slug>` (alias: `join <slug>`)

Joins a server by slug and switches your session scope to it. The
slug arg tab-completes from `Eligible to join` candidates.

```
dun> server join beta
joined "Beta" (slug=beta)
dun> where
  server:  beta
  world:   (none)
  kingdom: (none)
```

`join <slug>` is a convenience alias that does the same thing.

Common failure modes:

```
dun> server join private
error: invite only (code=forbidden, request_id=req-9c1a...)
```

```
dun> server join no-such-slug
error: not found (code=not_found, request_id=req-2e44...)
```

## 8. Your profile on a server

All `profile` and `player` verbs operate on the server currently in
scope. If you haven't joined or switched to one, you'll see:

```
error: not in a server scope — try `server join <slug>` first
```

### `profile show`

Reads your own profile on the in-scope server:

```
dun> profile show
IronFist
  real name: Alice Smith
  title:     Baron
  joined:    2026-05-12
  rounds:   3 played, 1 won
  wonders:  0 completed, 0 destroyed
  raids:    7 launched, 2 defended
```

If you just joined a server and haven't picked a handle yet:

```
dun> profile show
error: don't know your handle on this server yet — set one with `profile set --handle <name>`
```

The fix is `profile set` below.

### `profile set [--handle X] [--real-name "Y"]`

Updates your handle and/or real name. The handle must match
`^[A-Za-z0-9_-]{1,24}$` — letters, digits, underscore, hyphen, 1–24
characters.

**Flag-driven** (fast, scriptable):

```
dun> profile set --handle IronFist --real-name "Alice Smith"
IronFist
  real name: Alice Smith
  rounds:   0 played, 0 won
  wonders:  0 completed, 0 destroyed
  raids:    0 launched, 0 defended
```

**Interactive** (no flags → an in-terminal form pops up):

```
dun> profile set
```

Two fields appear:

```
  Handle (1-24 chars, A-Z 0-9 _ -)
  Real name (optional)
```

The handle field is pre-seeded with your current handle if known.
Submit to update, `Esc` to cancel.

Bad handle (caught client-side before the request leaves):

```
dun> profile set --handle "has spaces"
error: handle must be 1-24 chars: letters, digits, underscore, hyphen
```

Server-side handle lock (once the round starts):

```
dun> profile set --handle NewName
error: handle cannot be changed (code=handle_locked, request_id=req-7a...)
```

### `player show <handle>`

Reads another player's profile on the in-scope server. Same layout as
`profile show`:

```
dun> player show ShadowWolf
ShadowWolf
  real name: Bob Jones
  title:     Warlord
  joined:    2026-04-30
  rounds:   12 played, 5 won
  wonders:  2 completed, 1 destroyed
  raids:    34 launched, 9 defended
```

There is no tab completion on this argument today — you need to know
the handle.

## 9. Account management

These are **top-level** `dun` subcommands. Run them from your normal
shell, not from the `dun>` prompt.

### `dun keys list`

Tab-separated table of every API key issued for your account:

```
$ dun keys list
ID            NAME    LAST USED                  EXPIRES      STATUS
kpub_01J...   laptop  2026-05-18T14:02:11Z       2026-08-17   current
kpub_01H...   -       -                          2026-02-01   revoked
```

Columns:

- **ID** — key identifier.
- **NAME** — optional label set when the key was created; `-` if
  unnamed.
- **LAST USED** — RFC3339 timestamp of the last request signed with
  this key; `-` if it has never been used.
- **EXPIRES** — `YYYY-MM-DD`.
- **STATUS** — one of `current` (the key this CLI is using right
  now), `active`, or `revoked`.

### `dun keys revoke <id>`

Revokes a key on the backend. No confirmation prompt.

```
$ dun keys revoke kpub_01HXY...
Revoked key kpub_01HXY....
```

> If you revoke the `current` key, the CLI also deletes your local
> credential — effectively logging you out. The next `dun` command
> will say `not logged in — run \`dun login\``.

### `dun logout`

Revokes the current key on the backend and removes it from
`~/.dun/credentials`. No confirmation prompt.

```
$ dun logout
Logged out alice@example.com.
```

If you weren't logged in:

```
$ dun logout
error: no active session — nothing to log out of
```

### `dun account delete`

Permanently deletes your player account. Real name is purged, every
per-server handle is anonymized, stats are zeroed, all keys are
revoked, and the local credentials file is cleared. **Irreversible.**

A single `[y/N]` confirmation that quotes your email, the last 4
characters of the current key, and the base URL:

```
$ dun account delete
Account: alice@example.com  (key …a3f1)
This will permanently delete your account on http://localhost:3000/v1.
delete account alice@example.com? [y/N]: y
Account deleted.
```

Only `y` or `yes` (case-insensitive) proceed; anything else prints
`error: aborted` and exits non-zero.

## 10. Storage layout

Everything the CLI persists lives under a single `~/.dun/` directory
(mode `0700`):

| Path | Purpose | Mode |
|------|---------|------|
| `~/.dun/config.toml` | Optional. Overrides default base URL. | 0644 |
| `~/.dun/credentials` | TOML. API keys and the `[current]` pointer. | 0600 |
| `~/.dun/history` | REPL line history. | 0644 |
| `~/.dun/state.json` | Last-used `(server, world, kingdom)` scope per credential. | 0644 |
| `~/.dun/dun-cli.log` | JSON slog output, append-mode. | 0644 |

### Multiple accounts

`~/.dun/credentials` holds a list of credentials keyed by
`(base_url, email)` plus a `[current]` table that points at the
active one. You can be logged in to several `(base_url, email)`
combinations simultaneously — e.g. one for a local dev backend and
one for a production server.

To switch accounts today:

- Re-run `dun login` with a different email. The new credential is
  added and made current.
- Or edit `~/.dun/credentials` directly and change the `[current]`
  block.

### Session state isolation

`~/.dun/state.json` records your scope (`server_slug`, `world_slug`,
`kingdom_handle`) together with the `(base_url, email)` it belongs
to. When the shell starts, scope is only re-applied if it matches the
current credential — so switching accounts doesn't accidentally drop
you into someone else's server.

## 11. Troubleshooting

**`not logged in — run \`dun login\`\`**
There is no `[current]` credential in `~/.dun/credentials`. Either
run `dun login`, or edit the file's `[current]` table to point at an
existing entry.

**`backend unreachable: http://localhost:3000/v1: ...`**
The startup health probe failed. Check that your backend is running
and that `base_url` in `~/.dun/config.toml` (or the built-in default)
matches it.

**`error: ... (code=..., request_id=...)`**
A backend call failed. The `code` (`forbidden`, `not_found`,
`handle_locked`, etc.) tells you the category; the `request_id`
correlates with the backend's logs and is the single most useful
field to include in any bug report.

**Where to find logs**
`~/.dun/dun-cli.log` (JSON lines). One object per request /
significant event. Bump verbosity with `dun --log-level debug` if you
need more detail.
