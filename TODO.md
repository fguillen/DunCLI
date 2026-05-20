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
