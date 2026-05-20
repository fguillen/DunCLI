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
  world:   spring-2026
  kingdom: IronFist
```

- `server` is set by `server join` (or the post-login picker).
- `world` is set by `world join`.
- `kingdom` is your per-server handle, recorded by `profile set` and
  used as your kingdom's display name in every world on that server.

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

## 10. Worlds

Once you've joined a server, worlds are the next layer of scope. A
world is one round of `dun` on that server — players join it, get a
kingdom, build, fight, and (eventually) it ends.

### `worlds` — list worlds on the in-scope server

```
dun> worlds
Worlds:
  spring-2026          grace     Spring 2026
  autumn-2026          proposed  Autumn 2026
```

Columns: slug, status (`proposed`, `grace`, `active`, `archived`,
`cancelled`), name.

> The list does not split member vs eligible the way `servers` does —
> the OpenAPI shape for `listServerWorlds` does not carry a
> `my_kingdom` field. Flagged upstream as a backend co-evolution
> candidate; for now use `world show <slug>` to see whether you're in.

### `world show <slug>`

```
dun> world show spring-2026
Spring 2026  (slug=spring-2026)
  status:    grace
  T0:        2026-05-01 00:00 UTC
  grace end: 2026-05-03 00:00 UTC
  regions:   42
  kingdoms:  5 (min 8)
  your kingdom: id=01J...  home_region=reg-2
```

The `your kingdom:` line only appears if you've already joined this
world.

### `world join <slug>` (alias: `join world <slug>`)

```
dun> world join spring-2026
joined world (slug=spring-2026) as kingdom "IronFist"
home region assigned: reg-2
```

If the world is still `proposed`, your kingdom is a stub (no home
region yet) and you'll instead see:

```
no home region yet — assigned when the world starts
```

On success, the shell switches scope to that world. `where` will now
show it. Tab completion on `world join <Tab>` lists every world on
the in-scope server.

The convenience alias `join world <slug>` does the same thing. (And
`join <server-slug>` still works for servers — same `join` verb,
two forms.)

## 11. Map, regions, ruins, nodes

These verbs all require a world to be in scope. If you haven't joined
or set one, you'll see:

```
error: not in a world scope — try `world join <slug>` first
```

### `map` — every region on the in-scope world

```
dun> map
Map:
  T  Greyhollow        nodes=1  adj=Ironvale
  ^  Ironvale          nodes=0  adj=Greyhollow
```

The first column is a terrain glyph: `.` plains, `T` forest, `^`
hills, `M` mountain, `~` marsh. Output goes to scrollback — there's
no alt-screen and no navigation. To "step into" a neighbour, run
`region show <neighbour>`.

### `region show <name>`

```
dun> region show Greyhollow
Greyhollow  (T forest)
  position:  x=0.10 y=0.20
  owner:     kgd-7
  nodes:
    Greyhollow        gold    standard  owner=home-hoard
  adjacent:  Ironvale
```

`adjacent:` comes from a separate `showRegionAdjacent` call so the
neighbour names are always fresh.

### `ruins` — list ruins on the in-scope world

```
dun> ruins
Ruins:
  Ironvale          tier=major     unclaimed  garrison=archer=4, levy=12
```

Columns: region name, tier, claim state, garrison composition.

### `nodes [--owner mine|wild|captured|home-hoard]`

```
dun> nodes
Nodes:
  Greyhollow        gold    standard  owner=home-hoard
  Ironvale          iron    rich      owner=wild  garrison=pikeman=8
```

`--owner` filters client-side:

- `mine` — your kingdom owns it (and it's not a home-hoard).
- `home-hoard` — your one immovable starter node.
- `wild` — no owner.
- `captured` — owned, but not by you.

### `node show <id-or-region>`

Pass either a region name (handy when you've seen it on the map) or
the node's ULID directly.

```
dun> node show Greyhollow
Node nd-1  (gold, standard)
  region:    Greyhollow
  owner:     (home-hoard)
  base rate: 10/hr
```

If a region has more than one node, you'll be asked to pick one by
ULID — the verb prints the candidate IDs in that case.

Tab completion on `region show <Tab>` and `node show <Tab>` lists
region names from the world map.

## 12. Kingdom & economy

These verbs require a world in scope and that you have a kingdom in
it (i.e. you've already done `world join`). All of them target *your*
kingdom — there's no `kingdom show <handle>` for other players in v1.

### `kingdom` / `kingdom show`

```
dun> kingdom
Kingdom kgd-7
Stockpile (cap 1000):
  gold=100  wood=50  stone=25  iron=10
Production (per hour):
  gold=12  wood=6  stone=3  iron=1
Builds in progress:
  (none)
Training in progress:
  (none)
```

The stockpile is lazily accrued by the backend on every call against
current production rates and the warehouse cap. ETAs in the in-
progress sections are relative (e.g. `ETA 2h 14m`).

### `buildings [--upgradable]`

Lists one row per building kind in your kingdom.

```
dun> buildings
Buildings:
  barracks         L1   ready
  gold_mint        L3   building (ETA 14m)
  iron_mine        L0   tier-gated
  town_hall        L2   ready
  ...
```

Status column meanings: `ready` (can queue an upgrade now), `max`
(at level 20), `tier-gated` (needs another building's level first),
`unaffordable` (stockpile too low), `building (ETA ...)` (already
upgrading).

`--upgradable` trims the list to rows where `upgrade_possible` is
true backend-side.

### `build preview <kind>`

```
dun> build preview town_hall
town_hall upgrade preview
  level:     L2 → L3
  cost:      gold=100 wood=80 stone=40 iron=5
  duration:  30m
  tier:      gates met
  affords:   yes
```

If the upgrade isn't possible right now, the relevant line spells
out why (`tier:      unmet — needs stone_mason L1 (have L0)`,
`affords:   no (missing gold=50 wood=0 stone=0 iron=0)`).

Valid kinds: `town_hall`, `gold_mint`, `lumber_camp`, `quarry`,
`iron_mine`, `warehouse`, `barracks`, `stable`, `siege_workshop`,
`walls`, `watchtower`, `stone_mason`.

### `build <kind>`

Queues a real upgrade. First it prints the same preview as above,
then opens a yes/no confirmation:

```
dun> build town_hall
town_hall upgrade preview
  ...
Queue upgrade: town_hall → L3?
> Yes   No
```

On `Yes`, resources are deducted immediately and you'll see:

```
queued: town_hall → L3, completes 2026-05-19 21:30 UTC
```

Without a `<kind>` argument the verb opens an interactive picker
over the upgradable buildings instead:

```
dun> build
Pick a building to upgrade
> town_hall                L2 → L3
  gold_mint                L3 → L4
```

Common failure modes the verb catches client-side before sending the
request: `already at max level`, `tier gates unmet`, `can't afford
this upgrade right now`. Anything else surfaces as the usual
`error: ... (code=..., request_id=...)`.

### `build cancel <id-or-kind>`

You can cancel by either the build order's ULID or by the building
kind. If you pass a kind and there's exactly one active order for it,
the verb resolves it for you. If there are several, you'll be asked
to pick by ID.

```
dun> build cancel town_hall
Cancel this build order?
This refunds 75% of resources spent. Elapsed time is lost.
> Yes   No
cancelled build ord-9 (town_hall)
```

The build slot is freed immediately and 75% of the resources are
refunded; elapsed time is lost.

Tab completion on `build cancel <Tab>` lists the kinds with active
orders plus the order IDs themselves.

## 13. Storage layout

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

## 14. Troubleshooting

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
