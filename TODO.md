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

- [ ] `internal/api/client.go`: constructor taking base URL, token
      provider, and `*http.Client`; thin wrapper over `gen.Client`
- [ ] Auth: `Authorization: Bearer <api_key>` header via a custom
      `http.RoundTripper` (token loaded lazily from
      `~/.dun/credentials` in Phase 2)
- [ ] Error envelope decoding: parse
      `{ "error": { "code", "message", "retry_after?" } }` into a typed
      `api.Error` with `Code()`, `Message()`, `RetryAfter()`
- [ ] Request-ID propagation: capture `X-Request-Id` from every response,
      attach to `api.Error`, log via slog with `slog.String("request_id", ...)`
- [ ] 429 handling: when the envelope includes `retry_after`, surface it
      as `api.RateLimitError` carrying the duration; do NOT auto-retry
      silently (let the UI decide)
- [ ] Context cancellation: every wrapper method takes `context.Context`
      and threads it into the generated client
- [ ] Entity resolvers: `ResolveServer(slug)`, `ResolveWorld(slug)`,
      `ResolveKingdom(handle)`, `ResolveRegion(name)`, `ResolveArmy(name)`,
      `ResolvePlayer(handle)` — each backed by the relevant `list*` /
      `show*` endpoint with a per-session memoization cache that the
      shell owns. Returns the ULID for downstream calls. See
      CLAUDE.md "Entity identification". Mutations
      (`joinServer`, `joinWorld`, `splitArmy`, `renameArmy`,
      `mergeArmy`) must invalidate the relevant cache.
- [ ] Tests: `httptest.Server` exercising the auth header, the error
      decoder, request-id capture, the 429 path, and resolver
      cache invalidation

## Phase 2 — Auth & account

operationIds: `requestPlayerMagicLink`, `exchangePlayerMagicLink`,
`listPlayerApiKeys`, `revokePlayerApiKey`, `deleteAccount`.

- [ ] `internal/config/`: viper-backed loader; resolves
      `~/.dun/config.toml`; default base URL
      `http://localhost:3000/v1`
- [ ] `internal/auth/store.go`: TOML file at `~/.dun/credentials`
      (mode 0600), keyed by `(base_url, email)`. No OS keychain.
- [ ] `dun login`: prompt for email →  `requestPlayerMagicLink` → prompt
      for token from email → `exchangePlayerMagicLink` → persist
      `api_key` + `expires_at` to `~/.dun/credentials`
- [ ] `dun logout`: revoke current key via `revokePlayerApiKey`, remove
      the entry from `~/.dun/credentials`
- [ ] `dun keys list` / `dun keys revoke <id>`: `listPlayerApiKeys` /
      `revokePlayerApiKey`
- [ ] `dun account delete`: confirm twice → `deleteAccount`
- [ ] Tests: stubbed backend covering happy path, expired token, wrong
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

- [ ] Delete the Phase 0 splash (`internal/tui/splash.go`,
      `splash_test.go`); the `dun tui` subcommand becomes a thin
      wrapper that enters the shell (or is renamed to `dun shell` and
      the bare `dun` invocation also enters the shell — decide during
      implementation)
- [ ] Prompt component: line editor with persistent history
      (`~/.dun/history`), `dun>` prefix, multi-line input where
      useful, Ctrl-C to abort current line, Ctrl-D to exit
- [ ] Command dispatcher: parse the typed line into
      (verb, subverb, positional args, flags); route to the
      registered handler; pretty-print the result to scrollback
- [ ] Built-in shell commands: `help [verb]`, `quit` / `exit`,
      `clear`, `version`, `whoami`, `where` (current
      server/world/kingdom context), `use <server-slug>` /
      `use world <slug>` to scope subsequent commands
- [ ] Tab completion engine: pluggable per-verb. Static for keywords
      and enums (building kinds, unit kinds, intents); dynamic for
      entity references via the Phase 1 `Resolve*` helpers
- [ ] Interactive selector primitive: list-of-strings picker built on
      `charmbracelet/bubbles/list`; reused by every later phase when
      typed input would be tedious
- [ ] Form primitive: multi-field input built on `charmbracelet/huh`
      for things like march dispatch (target region + intent) and
      caravan build (receiver + payload + escort). **Add dep first.**
- [ ] Theme: Lipgloss palette in
      [internal/tui/theme/](internal/tui/theme/); light + dark
      variants applied to selectors, forms, and command output
- [ ] Connectivity probe: `getHealth` once on shell start; surface a
      clear "backend unreachable: <url>" state on failure with the
      relevant `X-Request-Id` and exit code
- [ ] Error rendering: every command failure prints
      `error: <human message> (code=<code>, request_id=<id>)` to
      scrollback; toast-style overlays only for selectors/forms
- [ ] Session context: shell remembers (server, world, kingdom)
      tuple; verbs default to the in-scope entity when not specified
- [ ] Session context persistence: write the current
      (server, world, kingdom) tuple to `~/.dun/state.json` after
      every successful `use` command and after `joinServer` /
      `joinWorld`. On shell start, re-load and re-apply so the user
      drops back into the same context — the "long-term memory"
      requirement
- [ ] Tests: dispatcher table-driven; completion engine unit tests;
      `teatest` snapshots for the prompt model and selector primitive

## Phase 4 — Server membership

operationIds: `listPlayerServers`, `joinServer`, `updateOwnProfile`,
`showPlayerProfile`.

- [ ] `servers` — `listPlayerServers`, member vs eligible split
- [ ] `server join <slug>` — `joinServer` for invite-only servers
- [ ] `profile set [--handle X] [--real-name "Y"]` — `updateOwnProfile`,
      respects §17.1 validation rules and the `handle_locked` guard
- [ ] `player show <handle>` — `showPlayerProfile` (on the in-scope
      server)
- [ ] Post-login default: drop into shell with the server picker open
      if the player has > 1 server membership

## Phase 5 — World browse & join

operationIds: `listServerWorlds`, `showWorld`, `joinWorld`.

- [ ] `worlds` — `listServerWorlds` for the in-scope server
- [ ] `world show <slug>` — `showWorld` (T0/grace timestamps,
      region/kingdom counts, caller's kingdom)
- [ ] `world join <slug>` — `joinWorld` (proposed/grace states);
      surface §16.7 admission errors and the §16.8 late-joiner bonus

## Phase 6 — Map & regions

operationIds: `showWorldMap`, `showRegion`, `showRegionAdjacent`,
`listRuins`, `listNodes`, `showNode`.

- [ ] `map` — `showWorldMap` printed as a styled text region list
      for the in-scope world: one line per region with terrain glyph
      (`Plains/Forest/Hills/Mountain/Marsh`), owner handle, node
      count, and adjacency. Output goes to scrollback; no alt-screen,
      no navigation
- [ ] `region show <name>` — `showRegion` plus an appended
      `Adjacent: …` line from `showRegionAdjacent`. To "step into" a
      neighbor, the user runs `region show <neighbor>`
- [ ] `ruins` — `listRuins` for the in-scope world
- [ ] `nodes [--owner mine|wild|captured|home-hoard]` — `listNodes`;
      `node show <id-or-region>` for detail via `showNode`

## Phase 7 — Kingdom dashboard & economy

operationIds: `showKingdom`, `listKingdomBuildings`, `previewBuildUpgrade`,
`queueBuildOrder`, `cancelBuildOrder`.

- [ ] `kingdom` / `kingdom show` — `showKingdom` (stockpile,
      production rates, in-progress orders)
- [ ] `buildings [--upgradable]` — `listKingdomBuildings`
- [ ] `build preview <kind>` — `previewBuildUpgrade` (cost, duration,
      tier gates, affordability)
- [ ] `build <kind>` — interactive selector for kind if omitted,
      confirms preview, calls `queueBuildOrder` with defensive
      `target_level` check
- [ ] `build cancel <id-or-kind>` — `cancelBuildOrder` (75% refund,
      time lost); confirms before calling

## Phase 8 — Military

operationIds: `queueTrainingOrder`, `previewTrainingOrder`,
`cancelTrainingOrder`, `listKingdomArmies`, `showArmy`, `splitArmy`,
`renameArmy`, `mergeArmy`, `dispatchMarch`, `recallMarch`.

- [ ] `train preview <building> <unit> <count>` —
      `previewTrainingOrder`
- [ ] `train <building> <unit> <count>` — `queueTrainingOrder`
      (per-building FIFO); selector for unit if omitted
- [ ] `train cancel <id>` — `cancelTrainingOrder` (75% refund)
- [ ] `armies` — `listKingdomArmies`
- [ ] `army show <name>` — `showArmy` (composition, active march,
      region)
- [ ] `army split <name>` — `splitArmy` (home-only invariants); form
      for new-army name + units
- [ ] `army rename <name> <new-name>` — `renameArmy`
- [ ] `army merge <name> --into <name>` — `mergeArmy`
- [ ] `march <army> <target-region> <intent>` — `dispatchMarch`;
      intent selector for `attack | reinforce | scout | capture |
      claim_ruin | caravan` if omitted
- [ ] `recall <army>` — `recallMarch` (no unit losses in v1)

## Phase 9 — Combat & battle reports

operationIds: `listKingdomBattles`, `showBattle`.

- [ ] `battles [--limit N] [--offset N]` — `listKingdomBattles`
- [ ] `battle show <id>` — `showBattle` with round-by-round log +
      participants
- [ ] Wilderness battles (Phase 7 capture flows) shown distinctly in
      the list when `defender_kingdom_id` is null

## Phase 10 — Nodes & ruins capture flows

No new endpoints — composes Phase 6 (map/region/nodes/ruins) with
Phase 8 (march dispatch with `capture` / `claim_ruin` intents).

- [ ] Wilderness node capture wizard: requires Catapult; surfaces
      `Nodes::Capture` / `Combat::ResolveGarrison` outcomes via the
      battle stream
- [ ] Owned node attack flow: `Nodes::Attack` (walk-in vs PvP)
- [ ] Ruin claim flow: `Ruins::Claim`; surface the warehouse-capped
      cache grant on success

## Phase 11 — Trade

operationIds: `dispatchCaravan`, `listTradeLedger`.

- [ ] `caravan send <receiver-handle>` — `dispatchCaravan`; opens a
      form for source army + payload + escort units with capacity
      validation
- [ ] `trade ledger [--player H] [--since 24h] [--limit N] [--page N]` —
      `listTradeLedger`
- [ ] Interception outcomes surfaced via the battle stream from
      Phase 9

## Phase 12 — Wonders

operationIds: `getWonder`, `startWonder`, `cancelWonder`, `repairWonder`,
`payWonderMilestone`, `listWorldWonders`.

- [ ] `wonder` / `wonder show` — `getWonder` (lazy
      `Wonders::ApplyConstruction`)
- [ ] `wonder start <name>` — `startWonder`; selector for name from
      the §14 fixed menu; 25% foundation payment with confirm
- [ ] `wonder milestone <25|50|75>` — `payWonderMilestone`
- [ ] `wonder repair <hp>` — `repairWonder` (1 HP per 8 Stone;
      2000 HP/phase cap)
- [ ] `wonder cancel` — `cancelWonder` (paid resources lost —
      double-confirm with typed name to match)
- [ ] `wonders` — `listWorldWonders` (public, in-scope world)

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
