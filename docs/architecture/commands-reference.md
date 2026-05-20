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
| `where` | Active (server, world, kingdom) scope |

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

---

## Phases not yet shipped

The following phases of [TODO.md](../../TODO.md) are tracked but not
implemented. operationIds listed here for forward navigation; expect
this section to migrate into the verb table above as each phase
lands.

- **Phase 8 — Military**: `queueTrainingOrder`, `previewTrainingOrder`,
  `cancelTrainingOrder`, `listKingdomArmies`, `showArmy`, `splitArmy`,
  `renameArmy`, `mergeArmy`, `dispatchMarch`, `recallMarch`
- **Phase 9 — Combat & battle reports**: `listKingdomBattles`,
  `showBattle`
- **Phase 10 — Nodes & ruins capture flows**: composes Phase 6 + Phase 8;
  no new endpoints
- **Phase 11 — Trade**: `dispatchCaravan`, `listTradeLedger`
- **Phase 12 — Wonders**: `getWonder`, `startWonder`, `cancelWonder`,
  `repairWonder`, `payWonderMilestone`, `listWorldWonders`
- **Phase 13 — Archive & Hall of Fame**: `getWorldArchive`,
  `getHallOfFame`
- **Phase 14 — Polish, packaging & distribution**: no new endpoints

---

## Out of scope for v1

The following admin operationIds are **explicitly out of scope** for
v1 ([TODO.md](../../TODO.md)) and have no CLI surface:

`requestAdminMagicLink`, `exchangeAdminMagicLink`, `listAdminApiKeys`,
`revokeAdminApiKey`, `listAdminServers`, `createServer`,
`updateServer`, `deleteServer`, `listServerAdmins`,
`inviteServerAdmin`, `revokeServerAdmin`, `listServerInvitations`,
`createServerInvitation`, `deleteServerInvitation`,
`listServerMembers`, `listAdminWorlds`, `proposeWorld`,
`showAdminWorld`, `configureWorld`, `cancelWorld`, `startWorld`,
`listWorldInvitations`, `createWorldInvitation`,
`deleteWorldInvitation`, `listWorldBattles`.

The wrapper's `bearerSource.AdminBearer` returns an error rather than
silently sending an empty token — see
[02-api-client.md](02-api-client.md) "Lazy bearer tokens".
