# TODO — dun-cli implementation roadmap

Numbered phases. Each phase lists sub-steps as `[ ]` checkboxes and
references the relevant `operationId`s from
[docs/backend/openapi.yaml](docs/backend/openapi.yaml) so they're
greppable. Each phase builds on the previous.

**Interaction model.** Every command from Phase 4 onwards ships as a
verb inside the interactive shell built in Phase 3 (e.g. `servers`,
`world join`, `kingdom build`), not as a `dun servers list`-style
one-shot. See [PRODUCT.md](PRODUCT.md) "Interaction model" for the
rationale and [CLAUDE.md](CLAUDE.md) "Interaction model — REPL, not
full-screen TUI" for implementation rules. Entity references are by
name/slug, not ULID — see CLAUDE.md "Entity identification".

**v1 scope is player surface only.** The following admin operationIds
are explicitly **out of scope** for v1 and will be revisited later:
`requestAdminMagicLink`, `exchangeAdminMagicLink`, `listAdminApiKeys`,
`revokeAdminApiKey`, `listAdminServers`, `createServer`, `updateServer`,
`deleteServer`, `listServerAdmins`, `inviteServerAdmin`,
`revokeServerAdmin`, `listServerInvitations`, `createServerInvitation`,
`deleteServerInvitation`, `listServerMembers`, `listAdminWorlds`,
`proposeWorld`, `showAdminWorld`, `configureWorld`, `cancelWorld`,
`startWorld`, `listWorldInvitations`, `createWorldInvitation`,
`deleteWorldInvitation`, `listWorldBattles`.

---

## Phase 0 — Project foundations

- [x] `go mod init github.com/fguillen/dun-cli`
- [x] Project skeleton: `cmd/dun/`, `internal/{api,auth,config,log,tui}/`
- [x] `.golangci.yml`, `.gitignore`, `.editorconfig`
- [x] Wire `ogen-go/ogen` codegen via `ogen.yml`; generate
      [internal/api/gen/](internal/api/gen/) (client features only)
- [x] `Makefile`: `generate`, `build`, `test`, `lint`, `run`, `tidy`,
      `clean`, `help`
- [x] `internal/log/`: slog JSON logger to `~/.dun/dun-cli.log`
- [x] `cmd/dun/`: cobra root + `version` + `tui` subcommands
      *(Note: `tui` is a Phase 0 placeholder splash. Phase 3 replaces
      it with the real REPL shell.)*
- [x] `internal/tui/`: Bubble Tea splash with snapshot test
- [x] `CLAUDE.md`, `PRODUCT.md`, `README.md`, `TODO.md`
- [ ] GitHub Actions CI: matrix on linux + macos, Go current + previous;
      runs `make lint`, `make test`, `make build`
- [ ] Branch protection: require CI green on `main`

## Phase 1 — API client wrapper

Hand-rolled `internal/api/` layer that wraps the ogen-generated client.
No new endpoints — just the plumbing every later phase consumes.

- [x] `internal/api/client.go`: constructor taking base URL, token
      provider, and `*http.Client`; thin wrapper over `gen.Client`
- [x] Auth: `Authorization: Bearer <api_key>` header — implemented via
      ogen's `SecuritySource` (more idiomatic than a custom
      `RoundTripper`; both achieve the lazy-load goal). The custom
      `requestIDTransport` handles the orthogonal concern of capturing
      `X-Request-Id` from every response.
- [x] Error envelope decoding: parse
      `{ "error": { "code", "message", "retry_after?" } }` into a typed
      `api.Error` with `Code`, `Message`, `RequestID`, `HTTPStatus`
- [x] Request-ID propagation: capture `X-Request-Id` from every response,
      attach to `api.Error`, log via slog with `slog.String("request_id", ...)`
- [x] 429 handling: detected at the transport (the OpenAPI spec does
      not declare 429s, so ogen would otherwise return
      `UnexpectedStatusCode`); surfaced as `api.RateLimitError`. No
      auto-retry.
- [x] Context cancellation: every wrapper method takes `context.Context`
      and threads it into the generated client
- [x] Entity resolvers: `ResolveServer(slug)`, `ResolveWorld(slug)`,
      `ResolveKingdom(worldID)` (caller's own kingdom via
      `showWorld.my_kingdom`), `ResolveRegion(name)`, `ResolveArmy(name)`,
      `ResolvePlayer(handle)` — each backed by the relevant `list*` /
      `show*` endpoint with a per-session memoization cache. Returns
      the ULID for downstream calls. `Invalidate{Servers,Worlds,Armies}`
      hooks are exposed for the mutations later phases will wire in.
- [x] Tests: `httptest.Server` exercising the auth header, the error
      decoder, request-id capture, the 429 path, and resolver
      cache invalidation

## Phase 2 — Auth & account

operationIds: `requestPlayerMagicLink`, `exchangePlayerMagicLink`,
`listPlayerApiKeys`, `revokePlayerApiKey`, `deleteAccount`.

- [x] `internal/config/`: viper-backed loader; resolves
      `~/.dun/config.toml`; default base URL
      `http://localhost:3000/v1`
- [x] `internal/auth/store.go`: TOML file at `~/.dun/credentials`
      (mode 0600), keyed by `(base_url, email)`. No OS keychain.
- [x] `dun login`: prompt for email →  `requestPlayerMagicLink` → prompt
      for token from email → `exchangePlayerMagicLink` → persist
      `api_key` + `expires_at` to `~/.dun/credentials`
- [x] `dun logout`: revoke current key via `revokePlayerApiKey`, remove
      the entry from `~/.dun/credentials`
- [x] `dun keys list` / `dun keys revoke <id>`: `listPlayerApiKeys` /
      `revokePlayerApiKey`
- [x] `dun account delete`: single y/N confirmation → `deleteAccount`
      *(deviation from original "confirm twice" — user direction at plan
      review)*
- [x] Tests: stubbed backend covering happy path, expired token, wrong
      scope (401), and `~/.dun/credentials` round-trip (file perms,
      TOML schema, missing-file behavior)

## Phase 3 — Interactive shell (REPL)

No new player endpoints. Uses `getHealth` for a connectivity probe on
startup. Replaces the Phase 0 splash with a long-running `dun>` prompt
in the spirit of psql / mongosh / redis-cli. The shell does **not**
use alt-screen; scrollback is preserved. Individual commands may pop
up transient Bubble Tea selectors (alt-screen ok there).

**Non-goal for v1:** no background data refresh, no live toasts, no
periodic polling. The shell renders only in response to user input.
See [PRODUCT.md](PRODUCT.md) anti-goals.

- [x] Delete the Phase 0 splash (`internal/tui/splash.go`,
      `splash_test.go`); bare `dun` now enters the shell directly —
      no `dun tui` / `dun shell` subcommand
- [x] Prompt component: line editor with persistent history
      (`~/.dun/history`), `dun>` prefix, Ctrl-C to abort current
      line, Ctrl-D to exit *(via `chzyer/readline`)*
- [x] Command dispatcher: parses (verb, subverb, positional args,
      flags) via `shlex`; routes to the registered handler; pretty-
      prints errors to scrollback
- [x] Built-in shell commands: `help [verb]`, `quit` / `exit`,
      `clear`, `version`, `whoami`, `where`, `join <server-slug>`
      *(`join world <slug>` registered but stubbed pending Phase 5)*
- [x] Tab completion engine: pluggable per-verb; static keywords +
      dynamic entity references; flag-name + flag-value completion
- [x] Interactive selector primitive: `bubbles/list` picker
      (`internal/tui/selector/list.go`)
- [x] Form primitive: `huh`-backed multi-field form
      (`internal/tui/selector/form.go`)
- [x] Theme: Lipgloss palette in
      [internal/tui/theme/](internal/tui/theme/); light + dark
      detected via termenv
- [x] Connectivity probe: `getHealth` once on shell start; surfaces
      `backend unreachable: <url> (code=…, request_id=…)` and a
      non-zero exit code on failure
- [x] Error rendering: every command failure prints
      `error: <message> (code=<code>, request_id=<id>)` via
      `shell.Err` and `internal/api.AsError`
- [x] Session context: shell remembers (server, world, kingdom);
      verbs read it via `sess.Context.*Slug()` / `KingdomHandle()`
- [x] Session context persistence: writes `~/.dun/state.json` after
      every successful dispatch; reloaded on shell start (foreign-
      credential entries ignored)
- [x] Tests: dispatcher table-driven; completion engine unit tests;
      state-file round-trip + foreign-credential isolation
      *(teatest snapshots for the prompt/selector primitive
      deferred — not strictly required to ship Phase 3)*

## Phase 4 — Server membership

operationIds: `listPlayerServers`, `joinServer`, `updateOwnProfile`,
`showPlayerProfile`.

- [x] `servers` — `listPlayerServers`, member vs eligible split
- [x] `server join <slug>` — `joinServer` for invite-only servers
- [x] `profile set [--handle X] [--real-name "Y"]` —
      `updateOwnProfile`, respects §17.1 validation rules and the
      `handle_locked` guard. Bonus: `profile show` reads the caller's
      own profile via `showPlayerProfile`.
- [x] `player show <handle>` — `showPlayerProfile` (on the in-scope
      server). No completer: `listServerPlayers` is missing from the
      spec — flagged as a backend co-evolution candidate.
- [x] Post-login default: `dun login` queries `listPlayerServers`
      and either auto-applies the single membership or opens the
      picker on shell entry when there are 2+

## Phase 5 — World browse & join

operationIds: `listServerWorlds`, `showWorld`, `joinWorld`.

- [x] `worlds` — `listServerWorlds` for the in-scope server. Renders
      a flat list (slug / status / name); cannot split member-vs-
      eligible because `WorldSummary` does not carry `my_kingdom` —
      flagged as a backend co-evolution candidate.
- [x] `world show <slug>` — `showWorld` (T0/grace timestamps,
      region/kingdom counts, caller's kingdom when joined)
- [x] `world join <slug>` — `joinWorld` (proposed/grace states);
      `join world <slug>` sugar in the `join` built-in now routes
      here (was stubbed in Phase 3)

## Phase 6 — Map & regions

operationIds: `showWorldMap`, `showRegion`, `showRegionAdjacent`,
`listRuins`, `listNodes`, `showNode`.

- [x] `map` — `showWorldMap` printed as a styled text region list
      for the in-scope world: one line per region with terrain glyph
      (`.` plains, `T` forest, `^` hills, `M` mountain, `~` marsh),
      node count, and adjacency by name. Output goes to scrollback;
      no alt-screen, no navigation
- [x] `region show <name>` — `showRegion` plus an appended
      `adjacent: …` line from `showRegionAdjacent`. To "step into" a
      neighbor, the user runs `region show <neighbor>`
- [x] `ruins` — `listRuins` for the in-scope world
- [x] `nodes [--owner mine|wild|captured|home-hoard]` — `listNodes`
      with client-side filtering; `node show <id-or-region>` for
      detail via `showNode` (region path uses `showRegion` and
      disambiguates if multiple nodes are present)

## Phase 7 — Kingdom dashboard & economy

operationIds: `showKingdom`, `listKingdomBuildings`, `previewBuildUpgrade`,
`queueBuildOrder`, `cancelBuildOrder`.

- [x] `kingdom` / `kingdom show` — `showKingdom` (stockpile,
      production rates, in-progress build / training orders with ETAs)
- [x] `buildings [--upgradable]` — `listKingdomBuildings` (status
      column derives from `upgrade_possible` / `at_max_level` /
      `tier_gates_met` / `affordable` / active `build_order`)
- [x] `build preview <kind>` — `previewBuildUpgrade` (cost, duration,
      tier gates, affordability with per-resource shortfall)
- [x] `build <kind>` — selector picker over upgradable buildings if
      kind is omitted, prints preview, `selector.Confirm` y/N before
      `queueBuildOrder`; reads defensive `target_level` from the
      preview response
- [x] `build cancel <id-or-kind>` — `cancelBuildOrder` (75% refund,
      time lost); resolves a `<kind>` to its order ID via
      `showKingdom.in_progress_builds`; confirms before calling

## Phase 8 — Military

operationIds: `queueTrainingOrder`, `previewTrainingOrder`,
`cancelTrainingOrder`, `listKingdomArmies`, `showArmy`, `splitArmy`,
`renameArmy`, `mergeArmy`, `dispatchMarch`, `recallMarch`.

- [x] `train preview <building> <unit> <count>` —
      `previewTrainingOrder`
- [x] `train <building> <unit> <count>` — `queueTrainingOrder`
      (per-building FIFO); selector chain (building → unit → count)
      kicks in when args are omitted
- [x] `train cancel <id-or-unit>` — `cancelTrainingOrder` (75%
      refund); resolves a unit kind to its order ID via
      `showKingdom.in_progress_training`, mirroring `build cancel`
- [x] `armies` — `listKingdomArmies` (region names resolved from
      `showWorldMap`)
- [x] `army show <name>` — `showArmy` (composition, status,
      location). Active-march detail is **not** rendered because the
      spec does not embed it on `Army` — flagged as backend
      co-evolution candidate
- [x] `army split <name>` — `splitArmy` (home-only invariants);
      `huh` form with one numeric input per unit in source
      composition + a name field
- [x] `army rename <name> <new-name>` — `renameArmy`
- [x] `army merge <name> --into <name>` — `mergeArmy`
- [x] `march <army> <target-region> [intent]` — `dispatchMarch`;
      intent selector for `attack | reinforce | scout | capture |
      claim_ruin | caravan` if omitted; path rendered with region
      names
- [x] `recall <army>` — `recallMarch` (no unit losses in v1); the
      spec's empty 404 is wrapped as a typed "no active march"
      `api.Error` so the user-facing error line is useful

**Phase 8 also landed:** dispatcher fix so leaf-with-Sub verbs route
correctly (e.g. `build preview town_hall` and `train preview …` now
reach their sub-handlers via the REPL; previously the `Sub` map was
only walked when the parent had no `Run`). Shared scope helpers moved
out of `kingdom.go` / `regions.go` into a sibling
[internal/tui/verbs/shared](internal/tui/verbs/shared/) package so the
new [internal/tui/verbs/armies](internal/tui/verbs/armies/)
subpackage can reuse them without exposing them on the parent's
public surface.

## Phase 9 — Combat & battle reports

operationIds: `listKingdomBattles`, `showBattle`.

- [x] `battles [--limit N] [--offset N]` — `listKingdomBattles`
      (newest-first; `--limit` is client-side capped at the spec's
      100; renders a "more: N remaining" hint when there are more
      pages)
- [x] `battle show <id>` — `showBattle` with verbose multi-line
      round log + participants block; `(you)` marker on the caller's
      side; tab completion lists IDs from the first page
- [x] Wilderness battles shown distinctly in the list when
      `defender_kingdom_id` is empty — opponent column collapses to
      `(wilderness)`. Spec gap: the field is `type: string` and
      listed in `required:`, but a wilderness battle has no defending
      kingdom. Flagged as a backend co-evolution candidate
      (`nullable: true` or a typed discriminator)

**Phase 9 also landed:** region names in both verbs go through the
in-scope world's `ShowWorldMap` cache (mirror of the Phase 8 armies
pattern); cross-world history falls back to raw region ULIDs —
flagged as a co-evolution candidate (`Battle.region_name`). Opponent
kingdoms render as truncated ULIDs because there's no
`(world_id, kingdom_id) → owner_handle` resolver — flagged for an
`attacker_handle` / `defender_handle` enrichment on `Battle`.

## Phase 10 — Nodes & ruins capture flows

No new endpoints — composes Phase 6 (map/region/nodes/ruins) with
Phase 8 (march dispatch with `capture` / `claim_ruin` intents).

- [x] Wilderness node capture wizard: `node capture [<region>]`
      dispatches a `capture`-intent march; outcomes surface via the
      battle stream once the march arrives. Per user direction at plan
      review, the CLI does **not** enforce the Catapult precondition
      client-side — the confirm subtitle nudges, the backend remains
      authoritative on the arrival-time defeat
- [x] Owned node attack flow: `node attack [<region>]`; same wire
      intent (`capture`), backend dispatches `Nodes::Attack` based on
      the node's current owner. Walk-in vs PvP is opaque to the CLI —
      the confirm subtitle hedges accordingly (backend co-evolution
      candidate: a `defenders` indicator on `Region`)
- [x] Ruin claim flow: `ruin claim [<region>]` dispatches a
      `claim_ruin`-intent march; preview block calls out the §16.11
      warehouse-cap warning ("anything over your Warehouse cap is
      lost") before the confirm

**Phase 10 also landed:** the `printMarchOrder`, `pathNames`, and
`regionNameMap` / `lookupRegionName` helpers (previously duplicated in
[internal/tui/verbs/armies/](internal/tui/verbs/armies/) and
[internal/tui/verbs/battles/](internal/tui/verbs/battles/)) moved into
[internal/tui/verbs/shared/march_render.go](internal/tui/verbs/shared/march_render.go)
so [internal/tui/verbs/expeditions.go](internal/tui/verbs/expeditions.go)
can reuse them without exporting them from a sibling subpackage. Sub-
verb registrations live in [regions.go](internal/tui/verbs/regions.go)
next to the existing `node show` / `ruins` declarations; a brand-new
singular `ruin` parent verb hosts the `claim` sub.

## Phase 11 — Trade

operationIds: `dispatchCaravan`, `listTradeLedger`.

- [x] `caravan send <receiver-handle>` — `dispatchCaravan`; opens a
      form for source army + payload + escort units. Per user
      direction at plan review, the CLI runs only light client-side
      validation (non-negative ints, ≥1 payload entry, ≥1 escort
      unit) — capacity / stockpile / reachability remain
      backend-authoritative
- [x] `trade ledger [--player H] [--since 24h] [--limit N] [--page N]` —
      `listTradeLedger`. Matches the spec's 1-based `--page` (does
      **not** mirror Phase 9's `--offset`); `--since` is
      regex-validated client-side (`24h`, `7d`, `30m`, `1h30m`) before
      the HTTP call
- [x] Interception outcomes surfaced via the battle stream from
      Phase 9 — automatic from the player's point of view; the trade
      subpackage never imports `battles/`

**Phase 11 also landed:** trade verbs live in a new
[internal/tui/verbs/trade/](internal/tui/verbs/trade/) subpackage
(mirrors Phase 8 / 9 layout — new endpoint family, non-trivial form,
room for trade-local helpers). Backend co-evolution candidates flagged
on the way in: (1) `listServerPlayers` still missing → no completer
on `<receiver-handle>` or `--player` (re-surfaced from Phase 4 / 9 /
10); (2) `Caravan` response carries only kingdom IDs (no
`sender_handle` / `receiver_handle` snapshots — asymmetric with
`TradeLedgerEntry`); (3) per-unit carrying capacity stat is absent
from the generated `Unit` schema, so the caravan form can't surface a
live capacity meter.

## Phase 12 — Wonders

operationIds: `getWonder`, `startWonder`, `cancelWonder`, `repairWonder`,
`payWonderMilestone`, `listWorldWonders`.

- [x] `wonder` / `wonder show` — `getWonder` (lazy
      `Wonders::ApplyConstruction`). The wrapper bypasses ogen for
      this one endpoint because the spec's `oneOf: [Wonder, {wonder:
      null}]` 200 shape trips ogen's sum-type discriminator on the
      "no wonder" branch — flagged as a backend co-evolution
      candidate. Returns `(nil, nil)` for "no wonder" so the verb
      prints a friendly hint
- [x] `wonder start <name>` — `startWonder`; selector for name from
      the §14 fixed menu (slug value, title-case display); confirm
      describes the 25% foundation payment in prose. No per-resource
      cost surfaced — backend is authoritative; flagged
      `previewWonderStart` as a co-evolution candidate
- [x] `wonder milestone <25|50|75>` — `payWonderMilestone`. Fetches
      `pending_milestone_percent` first and either auto-uses it (no
      arg) or validates the supplied percent matches before the
      confirm. Cost rendered in the confirm comes from
      `pending_milestone_cost` (already backend-provided)
- [x] `wonder repair <hp>` — `repairWonder` (1 HP per 8 Stone;
      2000 HP/phase cap). No arg → single-field form; arg →
      positive-integer guard. Confirm describes §16.2 rules in prose;
      flagged `previewWonderRepair` as a co-evolution candidate
- [x] `wonder cancel` — `cancelWonder` (paid resources lost). Typed-
      name double-confirm via `selector.Form` — user must type the
      wonder's snake_case slug verbatim before the destructive call
      is made
- [x] `wonders` — `listWorldWonders` (public, in-scope world); one
      row per wonder with builder handle, title-case name, status,
      HP fraction + percentage, started_at

**Phase 12 also landed:** wonder verbs live in a new
[internal/tui/verbs/wonders/](internal/tui/verbs/wonders/) subpackage
(mirrors Phase 8 / 9 / 11 layout — new endpoint family, mix of picker /
form / typed-confirm flows). The `getWonder` ogen-bypass workaround
lives in [internal/api/client.go](internal/api/client.go) alongside a
new `newRawRequest` helper so the raw call still routes through the
shared `requestIDTransport` for X-Request-Id capture. Backend
co-evolution candidates flagged on the way in: (1)
`previewWonderStart` is missing → `wonder start` confirm has no per-
resource cost; (2) `previewWonderRepair` is missing → `wonder repair`
form has no live Stone cost or per-phase remaining cap; (3) `getWonder`
200 response shape can't be decoded by ogen on the `{"wonder": null}`
branch — flattening to a single nullable-`wonder` object would let
ogen own it; (4) `Wonder.builder_handle` is missing from the singular
schema (it exists on `WonderListItem`) — adding it would unblock a
future `wonder show <handle>` verb.

## Phase 13 — Archive & Hall of Fame

operationIds: `getWorldArchive`, `getHallOfFame`.

- [ ] `archive [<world-slug>]` — `getWorldArchive` (frozen
      end-of-round snapshot; 404 while live handled gracefully)
- [ ] `hall-of-fame [--kind champions|wreckers|warlords|veterans]` —
      `getHallOfFame` for the in-scope server

## Phase 14 — Polish, packaging & distribution

No new endpoints.

- [ ] GoReleaser config: cross-compile linux/macos/windows × amd64/arm64
- [ ] Homebrew tap: `fguillen/homebrew-tap/dun-cli`
- [ ] Scoop bucket: `fguillen/scoop-bucket/dun-cli`
- [ ] GitHub Releases automation with changelog from conventional
      commits
- [ ] Smoke-test workflow: run `dun login` + a read-only flow against a
      localhost backend in CI
- [ ] `dun --version` reads ldflags-injected version + commit
- [ ] Man page generation (cobra → mandoc)
- [ ] Shell completion: `dun completion {bash,zsh,fish}`
