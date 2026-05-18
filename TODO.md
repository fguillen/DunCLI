# TODO — dun-cli implementation roadmap

Numbered phases. Each phase lists sub-steps as `[ ]` checkboxes and
references the relevant `operationId`s from
[docs/backend/openapi.yaml](docs/backend/openapi.yaml) so they're
greppable. Each phase builds on the previous.

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
- [x] `internal/log/`: slog JSON logger to `$XDG_STATE_HOME/dun-cli/`
- [x] `cmd/dun/`: cobra root + `version` + `tui` subcommands
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
      `http.RoundTripper` (token loaded lazily from the keychain in
      Phase 2)
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
- [ ] Tests: `httptest.Server` exercising the auth header, the error
      decoder, request-id capture, and 429 path

## Phase 2 — Auth & account

operationIds: `requestPlayerMagicLink`, `exchangePlayerMagicLink`,
`listPlayerApiKeys`, `revokePlayerApiKey`, `deleteAccount`.

- [ ] `internal/config/`: viper-backed loader; resolves
      `$XDG_CONFIG_HOME/dun-cli/config.toml`; default base URL
      `http://localhost:3000/v1`
- [ ] `internal/auth/store.go`: `go-keyring` storage with `0600`
      config-file fallback when keychain unavailable; key per
      `(base_url, email)`
- [ ] `dun login`: prompt for email →  `requestPlayerMagicLink` → prompt
      for token from email → `exchangePlayerMagicLink` → persist
      `api_key` + `expires_at` to keychain
- [ ] `dun logout`: revoke current key via `revokePlayerApiKey`, wipe
      keychain entry
- [ ] `dun keys list` / `dun keys revoke <id>`: `listPlayerApiKeys` /
      `revokePlayerApiKey`
- [ ] `dun account delete`: confirm twice → `deleteAccount`
- [ ] Tests: stubbed backend covering happy path, expired token, wrong
      scope (401), and keychain-unavailable fallback

## Phase 3 — TUI shell

No new player endpoints. Uses `getHealth` for a connectivity probe on
startup.

- [ ] Screen stack: push/pop/replace, with a clean "back" gesture
- [ ] Status bar: shows current screen, connection state, key hints
- [ ] Help overlay (`?`): renders the active screen's keymap
- [ ] Theme: Lipgloss palette in [internal/tui/theme/](internal/tui/theme/);
      light + dark variants
- [ ] Global keymap: `q` quit, `?` help, `r` refresh, arrows + `j/k`
      navigation, `g/G` first/last
- [ ] Toast system: success / warning / error, surfaces `X-Request-Id`
      on errors
- [ ] Connectivity probe: `getHealth` on startup; if backend is down
      show a clear "backend unreachable" state instead of a hung spinner
- [ ] Tests: screen-stack unit tests + `teatest` snapshots for status
      bar and help overlay

## Phase 4 — Server membership

operationIds: `listPlayerServers`, `joinServer`, `updateOwnProfile`,
`showPlayerProfile`.

- [ ] Servers list screen: `listPlayerServers`, member vs eligible split
- [ ] Join confirmation flow: `joinServer` for invite-only servers
- [ ] Profile editor: `updateOwnProfile` (handle + real name), respects
      §17.1 validation rules and the `handle_locked` guard
- [ ] Player profile viewer: `showPlayerProfile` (by handle, on a server
      the caller is a member of)
- [ ] Server picker as the post-login default screen

## Phase 5 — World browse & join

operationIds: `listServerWorlds`, `showWorld`, `joinWorld`.

- [ ] World list screen per server: `listServerWorlds`
- [ ] World detail: `showWorld` (T0/grace timestamps, region/kingdom
      counts, caller's kingdom)
- [ ] Join flow: `joinWorld` (proposed/grace states); surface §16.7
      admission errors and the §16.8 late-joiner bonus

## Phase 6 — Map & regions

operationIds: `showWorldMap`, `showRegion`, `showRegionAdjacent`,
`listRuins`, `listNodes`, `showNode`.

- [ ] Map screen: `showWorldMap` rendered as an ASCII region graph
      (terrain glyphs per `Plains/Forest/Hills/Mountain/Marsh`)
- [ ] Region detail pane: `showRegion`, with `showRegionAdjacent`
      driving "step into neighbor" navigation
- [ ] Ruins overlay: `listRuins`
- [ ] Nodes overlay: `listNodes` (wilderness / captured / home hoard);
      detail via `showNode`

## Phase 7 — Kingdom dashboard & economy

operationIds: `showKingdom`, `listKingdomBuildings`, `previewBuildUpgrade`,
`queueBuildOrder`, `cancelBuildOrder`.

- [ ] Kingdom dashboard: `showKingdom` (stockpile, production rates,
      in-progress orders)
- [ ] Buildings list: `listKingdomBuildings` with `upgrade_possible`
      filter
- [ ] Upgrade preview pane: `previewBuildUpgrade` (cost, duration, tier
      gates, affordability)
- [ ] Queue upgrade action: `queueBuildOrder` (defensive `target_level`
      check)
- [ ] Cancel upgrade action: `cancelBuildOrder` (75% refund, time lost)

## Phase 8 — Military

operationIds: `queueTrainingOrder`, `previewTrainingOrder`,
`cancelTrainingOrder`, `listKingdomArmies`, `showArmy`, `splitArmy`,
`renameArmy`, `mergeArmy`, `dispatchMarch`, `recallMarch`.

- [ ] Training preview pane: `previewTrainingOrder` per
      barracks / stable / siege_workshop
- [ ] Queue training: `queueTrainingOrder` (per-building FIFO)
- [ ] Cancel training: `cancelTrainingOrder` (75% refund)
- [ ] Armies list: `listKingdomArmies`
- [ ] Army detail: `showArmy` (composition, active march, region)
- [ ] Split / rename / merge actions: `splitArmy`, `renameArmy`,
      `mergeArmy` (home-only invariants)
- [ ] March dispatch: `dispatchMarch` with intent selector
      (`attack | reinforce | scout | capture | claim_ruin | caravan`)
- [ ] March recall: `recallMarch` (no unit losses in v1)

## Phase 9 — Combat & battle reports

operationIds: `listKingdomBattles`, `showBattle`.

- [ ] Battles list: `listKingdomBattles` with `limit` / `offset`
      pagination
- [ ] Battle detail: `showBattle` with round-by-round log + participants
- [ ] Wilderness battles (Phase 7 capture flows) shown distinctly when
      `defender_kingdom_id` is null

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

- [ ] Caravan dispatch screen: `dispatchCaravan` (receiver handle,
      source army, payload, escort units; capacity validation)
- [ ] Trade ledger: `listTradeLedger` with `?player` / `?since` /
      `?limit` / `?page` filters
- [ ] Interception outcomes surfaced via the battle stream from Phase 9

## Phase 12 — Wonders

operationIds: `getWonder`, `startWonder`, `cancelWonder`, `repairWonder`,
`payWonderMilestone`, `listWorldWonders`.

- [ ] Wonder dashboard: `getWonder` (lazy `Wonders::ApplyConstruction`)
- [ ] Start Wonder: `startWonder` (name from §14 fixed menu, 25%
      foundation payment)
- [ ] Pay milestones: `payWonderMilestone` (25 / 50 / 75)
- [ ] Repair: `repairWonder` (1 HP per 8 Stone; 2000 HP/phase cap)
- [ ] Cancel: `cancelWonder` (paid resources lost — double-confirm)
- [ ] World Wonders list: `listWorldWonders` (public)

## Phase 13 — Archive & Hall of Fame

operationIds: `getWorldArchive`, `getHallOfFame`.

- [ ] World archive viewer: `getWorldArchive` (frozen end-of-round
      snapshot; 404 while live is handled gracefully)
- [ ] Hall of Fame: `getHallOfFame` with `?kind=` filter
      (Champions / Wreckers / Warlords / Veterans)

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
