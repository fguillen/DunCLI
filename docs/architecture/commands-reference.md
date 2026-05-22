# Commands Reference

A one-page index of every command shipped by dun-cli, grouped by
where it surfaces (cobra one-shot vs REPL verb) and cross-linked to
the backend `operationId` it calls.

The **authoritative request/response shape** is
[docs/backend/openapi.yaml](../backend/openapi.yaml); the flat backend
endpoint index is
[docs/backend/api-endpoints.md](../backend/api-endpoints.md). This
page is the client-side navigation aid — verbs/commands, the file
that registers them, and the architecture chapter to read for
context.

---

## Cobra one-shot commands

Commands that run outside the REPL — they exit when done. Defined
under [cmd/dun/](../../cmd/dun/) and wired in
[main.go](../../cmd/dun/main.go).

| Command | operationId(s) | Source | Notes |
|---|---|---|---|
| `dun` (bare) | `getHealth` (probe) | [main.go](../../cmd/dun/main.go) | Drops into the REPL shell after the connectivity probe |
| `dun version` | — | [main.go](../../cmd/dun/main.go) | Prints the `Version` constant (or ldflags override) |
| `dun login` | `requestPlayerMagicLink`, `exchangePlayerMagicLink`, `listPlayerServers` | [login.go](../../cmd/dun/login.go) | Magic-link flow + post-login handoff into the shell with `SingleMemberSlug` or `PostLoginPicker` set per membership count |
| `dun logout` | `listPlayerApiKeys`, `revokePlayerApiKey` | [logout.go](../../cmd/dun/logout.go) | Lists to find `current: true`, revokes, then clears local entry on success |
| `dun keys list` | `listPlayerApiKeys` | [keys.go](../../cmd/dun/keys.go) | Tabwriter dump; status is derived (revoked/current/active) |
| `dun keys revoke <id>` | `listPlayerApiKeys`, `revokePlayerApiKey` | [keys.go](../../cmd/dun/keys.go) | Detects "revoking my own current key" and clears local entry |
| `dun account delete` | `deleteAccount` | [account.go](../../cmd/dun/account.go) | Single y/N confirm; clears local credentials on success |
| `dun admin` (bare) | `getHealth` (probe) | [admin.go](../../cmd/dun/admin.go) | Drops into the `dun-admin>` REPL shell (Phase 14) |
| `dun admin login` | `requestAdminMagicLink`, `exchangeAdminMagicLink` | [admin_login.go](../../cmd/dun/admin_login.go) | Admin magic-link flow; validates owner type is `admin`; persists with `scope = "admin"`; hands off to the admin shell |
| `dun admin logout` | `listAdminApiKeys`, `revokeAdminApiKey` | [admin_logout.go](../../cmd/dun/admin_logout.go) | Lists to find `current: true`, revokes, then clears the local admin entry |

Architecture: see [03-auth-and-config.md](03-auth-and-config.md).

---

## REPL built-in verbs

Always available regardless of which game verbs are linked. Defined
in [shell/builtin.go](../../internal/tui/shell/builtin.go).

| Verb | Purpose |
|---|---|
| `help [verb]` | List verbs or print one verb's usage/flags |
| `quit` / `exit` | Exit the shell (also Ctrl-D) |
| `clear` | ANSI clear + home |
| `version` | Prints the shell's version |
| `whoami` | Active credential email + base URL |
| `where` | Active scope — `(server, world, kingdom)`, or `(server, world)` in the admin shell |

The built-ins are registered into both the player and the admin
registry, so they work identically in `dun>` and `dun-admin>`.

Architecture: see [04-repl-shell.md](04-repl-shell.md) "Built-in
verbs".

---

## REPL game verbs

Registered from [internal/tui/verbs/*.go](../../internal/tui/verbs/)
via `init()` → `shell.Register`. Architecture chapter:
[05-verbs.md](05-verbs.md).

### Phase 4 — Server membership & profile

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `servers` | `listPlayerServers` | [servers.go](../../internal/tui/verbs/servers.go) | Splits into "Member of:" / "Eligible to join:" via `Member` flag |
| `server join <slug>` | `joinServer` | [servers.go](../../internal/tui/verbs/servers.go) | Sets server scope, persists state |
| `join <slug>` / `join world <slug>` | (sugar) | [servers.go](../../internal/tui/verbs/servers.go) | Two-form sugar — routes to `server join` or `world join` |
| `profile show` | `showPlayerProfile` | [profile.go](../../internal/tui/verbs/profile.go) | Caller's own handle from `Context.KingdomHandle()` |
| `profile set [--handle X] [--real-name "Y"]` | `updateOwnProfile` | [profile.go](../../internal/tui/verbs/profile.go) | No flags → `selector.Form`; client-side §17.1 handle regex |
| `player show <handle>` | `showPlayerProfile` | [player.go](../../internal/tui/verbs/player.go) | No completer — flagged as backend co-evolution candidate |

### Phase 5 — Worlds

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `worlds` | `listServerWorlds` | [worlds.go](../../internal/tui/verbs/worlds.go) | Flat list — `WorldSummary` lacks `my_kingdom`, flagged as co-evolution candidate |
| `world show <slug>` | `showWorld` | [worlds.go](../../internal/tui/verbs/worlds.go) | T0/grace, region/kingdom counts, caller's kingdom when joined |
| `world join <slug>` | `joinWorld` | [worlds.go](../../internal/tui/verbs/worlds.go) | Sets world scope; reports home region assignment |

### Phase 6 — Map, regions, nodes, ruins

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `map` | `showWorldMap` | [regions.go](../../internal/tui/verbs/regions.go) | Text-only styled list to scrollback (no alt-screen) |
| `region show <name>` | `showRegion`, `showRegionAdjacent` | [regions.go](../../internal/tui/verbs/regions.go) | Sequential calls; appended `adjacent: …` line |
| `ruins` | `listRuins` | [regions.go](../../internal/tui/verbs/regions.go) | Tier + claimed state + garrison composition |
| `nodes [--owner mine\|wild\|captured\|home-hoard]` | `listNodes` | [regions.go](../../internal/tui/verbs/regions.go) | Client-side filtering; `mine`/`captured` trigger `ResolveKingdom` |
| `node show <id-or-region>` | `showRegion` or `showNode` | [regions.go](../../internal/tui/verbs/regions.go) | Tries region name first; falls back to ULID |

### Phase 7 — Kingdom dashboard & economy

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `kingdom` / `kingdom show` | `showKingdom` | [kingdom.go](../../internal/tui/verbs/kingdom.go) | Stockpile, production, in-progress orders with ETAs |
| `buildings [--upgradable]` | `listKingdomBuildings` | [kingdom.go](../../internal/tui/verbs/kingdom.go) | Status derived from `upgrade_possible`/`at_max_level`/`tier_gates_met`/`affordable`/`build_order` |
| `build preview <kind>` | `previewBuildUpgrade` | [kingdom.go](../../internal/tui/verbs/kingdom.go) | Cost, duration, tier gates, per-resource affordability |
| `build <kind>` (or no arg → picker) | `previewBuildUpgrade`, `queueBuildOrder` | [kingdom.go](../../internal/tui/verbs/kingdom.go) | `selector.Pick` over upgradable, then `selector.Confirm`, then queue |
| `build cancel <id-or-kind>` | `showKingdom`, `cancelBuildOrder` | [kingdom.go](../../internal/tui/verbs/kingdom.go) | Resolves `<kind>` to order ID via `in_progress_builds`; confirm before cancel |

### Phase 8 — Military (training, armies, marches)

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `train preview <building> <unit> <count>` | `previewTrainingOrder` | [armies/train.go](../../internal/tui/verbs/armies/train.go) | Cost + duration + affordability + `max_affordable_count` |
| `train <building> <unit> <count>` (or chain of pickers) | `previewTrainingOrder`, `queueTrainingOrder` | [armies/train.go](../../internal/tui/verbs/armies/train.go) | Chains `selector.Pick`(building) → `selector.Pick`(unit) → `selector.Form`(count) when args omitted; preview + `selector.Confirm` before queue |
| `train cancel <id-or-unit>` | `showKingdom`, `cancelTrainingOrder` | [armies/train.go](../../internal/tui/verbs/armies/train.go) | Resolves a unit kind to its order ID via `in_progress_training`; 75% refund |
| `armies` | `listKingdomArmies` | [armies/armies.go](../../internal/tui/verbs/armies/armies.go) | Table with status, region (resolved from `ShowWorldMap`), capacity, composition summary |
| `army show <name>` | `showArmy` after `ResolveArmy` | [armies/armies.go](../../internal/tui/verbs/armies/armies.go) | Full composition + status + location |
| `army split <name>` | `splitArmy` after `ShowArmy` | [armies/armies.go](../../internal/tui/verbs/armies/armies.go) | `huh` form with one numeric input per unit in source + name field; client-side `status == home` guard |
| `army rename <name> <new-name>` | `renameArmy` | [armies/armies.go](../../internal/tui/verbs/armies/armies.go) | Client-side 1–60 char check; backend `name_taken` arrives wrapped as `code: invalid` |
| `army merge <name> --into <other-name>` | `mergeArmy` after two `ResolveArmy` | [armies/armies.go](../../internal/tui/verbs/armies/armies.go) | `selector.Confirm` before commit; backend enforces same-region + both-home |
| `march <army> <target-region> [intent]` | `dispatchMarch` after `ResolveArmy` + `ResolveRegion` | [armies/march.go](../../internal/tui/verbs/armies/march.go) | Intent picker over six values when omitted; path rendered with region names |
| `recall <army>` | `recallMarch` | [armies/march.go](../../internal/tui/verbs/armies/march.go) | No-loss recall; wrapper synthesises a typed `not_found` from the spec's empty 404 |

### Phase 9 — Combat & battle reports

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `battles [--limit N] [--offset N]` | `listKingdomBattles` | [battles/battles.go](../../internal/tui/verbs/battles/battles.go) | Newest-first table; opponent column is `(wilderness)` when `defender_kingdom_id` is empty, otherwise the *other* side's kingdom ID (handles caller-defended as well as caller-attacked). Region names come from the `ShowWorldMap` cache. Pagination via `--limit` (≤ 100) / `--offset`; total-count footer + "more" hint when more pages remain |
| `battle show <id>` | `showBattle` | [battles/battles.go](../../internal/tui/verbs/battles/battles.go) | Header (region, when, outcome, titles, march, loot) + per-side participants (`(you)` marker on the caller) + verbose multi-line round log with optional `walls:` line. Tab completion lists IDs from the most recent battles page. 404 covers both unknown IDs and someone else's battles |

### Phase 10 — Nodes & ruins capture flows

All three verbs are guided wizards that compose `listNodes` /
`listRuins` (target discovery) + `listKingdomArmies` (home-army
picker) + `dispatchMarch` (action). No new endpoints; the actual
outcome resolves at march arrival and surfaces through `battles`.

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `node capture [<region>]` | `listNodes`, `listKingdomArmies`, `dispatchMarch` (intent=`capture`) | [expeditions.go](../../internal/tui/verbs/expeditions.go) | Eligible regions: wilderness nodes (no owner, not home-hoard). Preview shows the static garrison from `Node.Garrison`. **No client-side Catapult enforcement** — confirm subtitle nudges, backend is authoritative |
| `node attack [<region>]` | `listNodes`, `listKingdomArmies`, `dispatchMarch` (intent=`capture`) | [expeditions.go](../../internal/tui/verbs/expeditions.go) | Eligible regions: nodes owned by another kingdom. Same wire intent as `node capture` — backend dispatches `Nodes::Attack` vs `Nodes::Capture` based on the node's current owner. Walk-in vs PvP is opaque to the CLI |
| `ruin claim [<region>]` | `listRuins`, `listKingdomArmies`, `dispatchMarch` (intent=`claim_ruin`) | [expeditions.go](../../internal/tui/verbs/expeditions.go) | Eligible regions: any ruin with `Claimed == false`. Preview embeds the §16.11 "anything over your Warehouse cap is lost" warning |

### Phase 11 — Trade

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `caravan send <receiver-handle>` | `dispatchCaravan` | [trade/caravan.go](../../internal/tui/verbs/trade/caravan.go) | Splits escort off a home army, packs `gold`/`wood`/`stone`/`iron` payload, dispatches `caravan`-intent march. Form gathers payload + per-unit escort; only light client-side validation (non-negative ints, ≥1 payload entry, ≥1 escort unit). No `<receiver-handle>` completer — `listServerPlayers` gap |
| `trade ledger [--player H] [--since 24h] [--limit N] [--page N]` | `listTradeLedger` | [trade/ledger.go](../../internal/tui/verbs/trade/ledger.go) | World-scoped, newest-first. `--page` matches the spec's 1-based shape (does **not** mirror Phase 9's `--offset`). `--since` regex-validated client-side; `--limit` capped at 100 |

Interception combat lands in the Phase 9 `battles` history (when a
hostile army is camped at the destination region), so the trade
subpackage never imports `battles/` directly.

### Phase 12 — Wonders

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `wonder` / `wonder show` | `getWonder` | [wonders/show.go](../../internal/tui/verbs/wonders/show.go) | Bare `wonder` routes to `show` (parent `Run`). The wrapper bypasses ogen because the spec's `oneOf: [Wonder, {wonder: null}]` 200 shape can't be decoded on the "no wonder" branch — flagged co-evolution. Returns `(nil, nil)` for "no wonder" so the verb prints `no wonder under construction — try \`wonder start <name>\`` |
| `wonder start [<name>]` | `startWonder` | [wonders/start.go](../../internal/tui/verbs/wonders/start.go) | No arg → `selector.Pick` over the six §14 slugs. With arg → strict slug validation client-side. Confirm subtitle describes the 25% foundation payment + build-queue lock in prose; no per-resource cost (no `previewWonderStart` endpoint — flagged co-evolution) |
| `wonder cancel` | `cancelWonder` | [wonders/cancel.go](../../internal/tui/verbs/wonders/cancel.go) | Typed-name double-confirm via `selector.Form` — user must type the wonder's snake_case slug verbatim (case-sensitive). Destructive: all paid resources are lost server-side |
| `wonder repair [<hp>]` | `repairWonder` | [wonders/repair.go](../../internal/tui/verbs/wonders/repair.go) | No arg → single-field form. With arg → positive-integer guard. Confirm describes §16.2 in prose (8 Stone/HP, 2000 HP/phase cap, 30 min pause per 500 HP); no live numbers (no `previewWonderRepair` endpoint — flagged co-evolution) |
| `wonder milestone [25\|50\|75]` | `payWonderMilestone` | [wonders/milestone.go](../../internal/tui/verbs/wonders/milestone.go) | Fetches wonder first; no-arg form auto-uses the pending percent. Mismatch + no-pending guards fire client-side before any HTTP call. Cost in the confirm comes from `pending_milestone_cost` (backend-provided) |
| `wonders` | `listWorldWonders` | [wonders/list.go](../../internal/tui/verbs/wonders/list.go) | World-scoped public list. One row per wonder with builder handle, title-case name, status, HP fraction + percentage, started_at |

### Phase 13 — Archive & Hall of Fame

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `archive [<world-slug>]` | `getWorldArchive` | [archive/archive.go](../../internal/tui/verbs/archive/archive.go) | No arg → in-scope world; arg overrides scope so users can read archives for worlds they never joined. Renders aggregate counts (regions / kingdoms / nodes / battles / caravans) plus an optional `Wonder:` block and a `Top kingdoms` highlight capped at 5 rows sorted by `final_node_count` desc. Backend returns 404 while the world is still live or has no archive row — both surface as `code=not_found` |
| `hall-of-fame [--kind champions\|wreckers\|warlords\|veterans]` | `getHallOfFame` | [archive/hall_of_fame.go](../../internal/tui/verbs/archive/hall_of_fame.go) | Server-scoped. No flag → all four boards capped at top 5 each with a `--kind <name>` hint; `--kind` → full list for that board. Client-side `--kind` validation against a hard-coded slice so a typo never reaches the spec's 422. Snapshots are recomputed only at round end (§17.4); the verb prints `(no snapshot yet)` for boards the backend hasn't built |

---

## Admin track — REPL verbs

Registered from
[internal/tui/verbs/admin/*.go](../../internal/tui/verbs/admin/) via
`init()` → `shell.RegisterAdmin`, so they surface only in the
`dun-admin>` shell. Architecture: [05-verbs.md](05-verbs.md) "admin/".

### Phase 14 — Admin auth & shell

| Verb | operationId | Source | Notes |
|---|---|---|---|
| `keys list` | `listAdminApiKeys` | [admin/keys.go](../../internal/tui/verbs/admin/keys.go) | Tabwriter dump; status derived (revoked/current/active) — the in-shell mirror of `dun keys list` |
| `keys revoke <id>` | `listAdminApiKeys`, `revokeAdminApiKey` | [admin/keys.go](../../internal/tui/verbs/admin/keys.go) | Detects "revoking this session's current key" and clears the local admin credential; `<id>` tab-completes from non-revoked keys |

Phase 14 also wired the admin foundation that Phases 15–18 consume:
the `dun admin` entrypoint, the `shell.Mode` discriminator + admin
verb registry, the `scope = "admin"` credential field +
`[current_admin]` pointer, and the `ResolveAdminServer` /
`ResolveAdminWorld` resolvers in `internal/api`.

---

## Phases not yet shipped

The following phases of [TODO.md](../../TODO.md) are tracked but not
implemented. Expect this section to migrate into the tables above as
each phase lands.

- **Phases 15–18 — Admin server / team / world / battle verbs**:
  `listAdminServers`, `createServer`, `updateServer`, `deleteServer`,
  `listServerAdmins`, `inviteServerAdmin`, `revokeServerAdmin`,
  `listServerInvitations`, `createServerInvitation`,
  `deleteServerInvitation`, `listServerMembers`, `listAdminWorlds`,
  `proposeWorld`, `showAdminWorld`, `configureWorld`, `cancelWorld`,
  `startWorld`, `listWorldInvitations`, `createWorldInvitation`,
  `deleteWorldInvitation`, `listWorldBattles`. (`listAdminServers` /
  `listAdminWorlds` already have wrapper methods + resolvers from
  Phase 14; Phases 15/17 add the verbs that call them.)
- **Phase 19 — Polish, packaging & distribution**: no new endpoints.
