# 05 — Game Verbs

Phases 4-9 of [TODO.md](../../TODO.md). The files under
[internal/tui/verbs/](../../internal/tui/verbs/) — one per logical
area (servers, profile, players, worlds, regions, kingdom). Phase 8
moves into a sibling subpackage at
[internal/tui/verbs/armies/](../../internal/tui/verbs/armies/) so the
military surface (train, armies, march) can be split across multiple
files without crowding the parent. Phase 9 follows the same shape
under [internal/tui/verbs/battles/](../../internal/tui/verbs/battles/).
Each file registers its verbs into the shell's package-level registry
via `init()`, then implements handlers that follow a tight, repeating
shape.

If you're adding a new verb, this is the file. If you're debugging
why a specific verb behaves the way it does, the per-file tour at the
bottom lists the load-bearing decisions.

---

## How verbs plug in

```
internal/tui/verbs/<area>.go
    │
    ▼
func init() {
    shell.Register(&shell.Verb{
        Name: "...", Summary: "...", Usage: "...",
        Run:      runFoo,
        Complete: shell.SuggestFunc(...),
        Flags:    []shell.FlagSpec{...},
        Sub: map[string]*shell.Verb{...},
    })
}
```

`cmd/dun/main.go` brings the package(s) in with blank imports:

```go
_ "github.com/fguillen/dun-cli/internal/tui/verbs"
_ "github.com/fguillen/dun-cli/internal/tui/verbs/armies"
_ "github.com/fguillen/dun-cli/internal/tui/verbs/battles"
```

By the time `shell.Run` reads the registry, every verb is registered.
See [04-repl-shell.md](04-repl-shell.md) "The verb registry" for the
side of the contract the shell sees.

The verbs package can also call hooks the shell exposes:

```go
shell.SetPostLoginPicker(postLoginServerPicker)
```

That keeps the shell free of game-specific imports — the verbs
package decides what the post-login flow looks like.

---

## The canonical handler shape

Every leaf handler follows roughly the same six steps:

```go
func runXxx(ctx context.Context, sess *shell.Session, args []string, flags map[string]string) error {
    // 1. Validate args / scope guards.
    if len(args) != 1 { return errors.New("usage: xxx <slug>") }
    worldID, err := requireWorldID(ctx, sess)         // or requireKingdomID
    if err != nil { return err }

    // 2. Resolve user-typed names → ULIDs.
    targetID, err := sess.API.ResolveSomething(ctx, worldID, args[0])
    if err != nil { return err }

    // 3. Call the wrapped API method.
    res, err := sess.API.DoSomething(ctx, worldID, targetID)
    if err != nil { return err }

    // 4. Mutate session Context if relevant, persist.
    sess.Context.SetWorld(slug)
    if perr := sess.State.Save(sess.Context.Snapshot()); perr != nil {
        shell.Err(sess.Out, fmt.Errorf("persist state: %w", perr))
    }

    // 5. Render results into scrollback.
    printXxx(sess, res)

    // 6. Return nil. The dispatcher's catch-all save also runs.
    return nil
}
```

The dispatcher catches any returned error and renders it via
`shell.Err` (see [04-repl-shell.md](04-repl-shell.md) "Parse +
Dispatch"). Handlers never call `shell.Err` for their own primary
error — they `return err` and let the dispatcher format it. The only
exception is `persist state:` errors above, which are non-fatal: the
in-memory state is correct, the user just gets a heads-up.

---

## Scope guards: `shared.RequireWorldID`, `shared.RequireKingdomID`

[internal/tui/verbs/shared/scope.go](../../internal/tui/verbs/shared/scope.go)
holds the two reusable scope guards every Phase 5+ verb leans on.
They started life inside `package verbs` but moved into `shared` when
Phase 8's `armies` subpackage needed them too — keeping them in a
sibling package avoids exposing them on the parent's public surface.

```go
// RequireWorldID is the Phase 6+ equivalent of the Phase 4 "not in a
// server scope" guard.
func RequireWorldID(ctx, sess) (string, error) {
    if sess.Context.ServerSlug() == "" {
        return "", errors.New("not in a server scope — try `server join <slug>` first")
    }
    if sess.Context.WorldSlug() == "" {
        return "", errors.New("not in a world scope — try `world join <slug>` first")
    }
    serverID, _ := sess.API.ResolveServer(...)
    return sess.API.ResolveWorld(ctx, serverID, worldSlug)
}

// RequireKingdomID layers on top:
func RequireKingdomID(ctx, sess) (string, error) {
    worldID, err := RequireWorldID(ctx, sess); if err != nil { return "", err }
    id, err := sess.API.ResolveKingdom(ctx, worldID)
    if err != nil {
        if apiErr := api.AsError(err); apiErr != nil && apiErr.Code == "not_found" {
            return "", errors.New("you have no kingdom in this world — try `world join <slug>` first")
        }
        return "", err
    }
    return id, nil
}
```

The package also exports `RelTime(time.Time) string` — the shared
"X minutes from now" ETA formatter — used in every Phase 7+ printer.

The pattern: check `Context` first (no HTTP if the scope isn't set),
then call into the resolver, then translate a generic `not_found`
into a user-facing actionable hint. New verbs should reuse these
helpers, not reinvent the checks.

---

## Resolving names → ULIDs

Players type `acme`, `spring-2026`, `Greyhollow`, `Vanguard`,
`IronFist`. The backend speaks ULIDs. Verbs bridge the two via the
`Resolve<Entity>(ctx, ...)` methods on `*api.Client` — see
[02-api-client.md](02-api-client.md) "The resolver cache".

Common patterns:

```go
serverID, err := sess.API.ResolveServer(ctx, sess.Context.ServerSlug())
worldID,  err := sess.API.ResolveWorld(ctx, serverID, args[0])
regionID, err := sess.API.ResolveRegion(ctx, worldID, args[0])
kingdomID,err := sess.API.ResolveKingdom(ctx, worldID)   // caller's own
```

Per-session caching means the first resolve of each entity pays for
every subsequent lookup. The cache invalidates on mutations — see the
table in [02-api-client.md](02-api-client.md).

Resolvers return `*api.Error{Code: "not_found"}` on miss so handlers
can treat client-side and server-side 404s identically. `api.AsError`
extracts the typed error from any wrapped form.

---

## Mutation → context update → state save

Verbs that change the user's scope follow a strict order:

1. Server-side API call succeeds.
2. `sess.Context.Set...(...)` updates the in-memory scope. Setters
   automatically clear narrower scope (see
   [04-repl-shell.md](04-repl-shell.md) "Per-session Context").
3. `sess.State.Save(sess.Context.Snapshot())` persists the new scope.
   Failure is logged but non-fatal.
4. Render a `shell.Success` line so the user sees confirmation.

Examples ([verbs/servers.go](../../internal/tui/verbs/servers.go) and
[verbs/worlds.go](../../internal/tui/verbs/worlds.go)):

- `server join <slug>` → `Context.SetServer(slug)` →
  `shell.Success("joined "acme"")`.
- `world join <slug>` → `Context.SetWorld(slug)` →
  `shell.Success("joined world (slug=…) as kingdom ...")` →
  optional `shell.Info` line if a home region is assigned.

If the API call fails, `Context` is **not** mutated. The shell's
post-dispatch catch-all save then writes the unchanged snapshot — a
no-op in disk terms.

---

## Tab completion: static lists vs dynamic Suggesters

Each verb declares its completer via `Verb.Complete` or
`FlagSpec.Suggest`. Two shapes recur:

**Static** — fixed list at register-time:

```go
Complete: shell.SuggestFunc(func(_, _, _) ([]string, error) {
    return []string{"preview", "cancel"}, nil
})
```

**Dynamic** — consults the api client (often via a resolver):

```go
func suggestEligibleServerSlugs(ctx, sess, _) ([]string, error) {
    list, err := sess.API.ListPlayerServers(ctx)
    if err != nil { return nil, err }
    out := make([]string, 0, len(list))
    for _, s := range list { if !s.Member { out = append(out, s.Slug) } }
    return out, nil
}
```

Dynamic Suggesters run under an 800 ms timeout enforced by
`runSuggester` in [completion.go](../../internal/tui/shell/completion.go)
— see [04-repl-shell.md](04-repl-shell.md) "Tab completion". A wedged
Suggester can't freeze the prompt, but the Suggester is responsible
for honoring `ctx.Done()`.

A few verbs intentionally have **no completer**, with a CLAUDE.md
co-evolution comment explaining why. `player show` is the canonical
example: there is no `listServerPlayers` endpoint in v1, so
completion would either spam 404s or return nothing — left unset and
flagged for the backend to grow the missing endpoint.

---

## Rendering — the `shell.*` helpers

Verbs render via the four `shell.*` helpers in
[render.go](../../internal/tui/shell/render.go):

| Helper | Use for |
|---|---|
| `shell.Strong(out, "...")` | Section title, kingdom id, slug+name |
| `shell.Section(out, "Header:", body)` | Multi-line block with consistent indent; renders `(none)` when body is empty |
| `shell.Success(out, "...")` | Positive confirmation (`joined …`, `queued …`) |
| `shell.Info(out, "...")` | Neutral hint (`home region assigned`, `aborted`) |
| `shell.Err(out, err)` | Errors — used by the dispatcher; verbs only call this for the non-fatal `persist state` case |

A per-verb `print<Entity>(sess, *gen.X)` helper handles the layout of
each result type — line-per-field, narrow, easy to copy/paste:

```go
func printWorld(sess *shell.Session, w *gen.World) {
    shell.Strong(sess.Out, fmt.Sprintf("%s  (slug=%s)", w.Name, w.Slug))
    _, _ = fmt.Fprintln(sess.Out, "  status:    " + string(w.Status))
    _, _ = fmt.Fprintln(sess.Out, "  T0:        " + w.T0At.Format(...))
    ...
}
```

The format matters: we deliberately avoid wide tables and ASCII boxes
because scrollback users tend to want one fact per line. See the
existing `printKingdom`, `printPreview`, `printRegion`, `printNode`
for the conventions to follow.

---

## Per-file tour

### [servers.go](../../internal/tui/verbs/servers.go) — Phase 4

| Verb | operationId | Notes |
|---|---|---|
| `servers` | `listPlayerServers` | Splits results into "Member of:" / "Eligible to join:" via the `Member` flag |
| `server join <slug>` | `joinServer` | Resolves slug, calls JoinServer, sets server scope, persists |
| `join <slug>` / `join world <slug>` | (sugar) | Two-form sugar: position-1 token `world` routes to `runWorldJoin`, otherwise to `runServerJoin` |

Also registers the post-login picker hook (`SetPostLoginPicker`)
which opens a `selector.Pick` over member servers when the player has
≥2 memberships. Single-membership players are handled earlier in
`cmd/dun/login.go` via `Options.SingleMemberSlug`.

### [profile.go](../../internal/tui/verbs/profile.go) — Phase 4

| Verb | operationId | Notes |
|---|---|---|
| `profile show` | `showPlayerProfile` | Calls with caller's own handle pulled from `Context.KingdomHandle()`. If the handle isn't known yet, surfaces a clear "set one with `profile set --handle ...`" hint instead of an API call |
| `profile set --handle X --real-name "Y"` | `updateOwnProfile` | No flags → open a `selector.Form`. Client-side regex check `^[A-Za-z0-9_-]{1,24}$` from §17.1 before the HTTP call to save a round-trip on obviously-bad input |

`profile set` also updates `Context.KingdomHandle` on success so
subsequent `profile show` and Phase 7 verbs can read it without
another API call.

### [player.go](../../internal/tui/verbs/player.go) — Phase 4

| Verb | operationId | Notes |
|---|---|---|
| `player show <handle>` | `showPlayerProfile` | No completer — `listServerPlayers` is missing from v1, flagged as a backend co-evolution candidate in the source |

### [worlds.go](../../internal/tui/verbs/worlds.go) — Phase 5

| Verb | operationId | Notes |
|---|---|---|
| `worlds` | `listServerWorlds` | Flat list, cannot split member-vs-eligible because `WorldSummary` lacks `my_kingdom` — flagged as a backend co-evolution candidate |
| `world show <slug>` | `showWorld` | T0/grace, region/kingdom counts, caller's kingdom when present |
| `world join <slug>` | `joinWorld` | Sets world scope on success. Surfaces home-region assignment as a second info line |

`suggestAnyWorldSlug` is shared between `world show` and `world join`
because `WorldSummary` doesn't carry membership info — once we can
distinguish member vs eligible we'll split.

### [regions.go](../../internal/tui/verbs/regions.go) — Phase 6

| Verb | operationId | Notes |
|---|---|---|
| `map` | `showWorldMap` | Renders one styled line per region: terrain glyph (`.` plains, `T` forest, `^` hills, `M` mountain, `~` marsh) + name + node count + adjacency-by-name. Output goes to scrollback — **no** alt-screen, **no** ASCII navigation. Phase 6 design choice in CLAUDE.md |
| `region show <name>` | `showRegion` + `showRegionAdjacent` | Sequential — small calls, an errgroup wouldn't pay |
| `ruins` | `listRuins` | Tier + claimed state + garrison composition |
| `nodes [--owner mine\|wild\|captured\|home-hoard]` | `listNodes` | Client-side filtering: the spec has no server-side filter. The `mine` / `captured` filters trigger a `ResolveKingdom` only when needed — don't burn the extra HTTP call when the user just types `nodes` |
| `node show <id-or-region>` | `showRegion` or `showNode` | Tries arg first as a region name (the common case); falls back to ULID. Multiple nodes in a region → error listing the IDs |

`terrainGlyph` falls back to `?` on an unknown enum so a spec drift
doesn't crash the render — the user sees the unknown terrain and the
issue is obvious.

### [kingdom.go](../../internal/tui/verbs/kingdom.go) — Phase 7

| Verb | operationId | Notes |
|---|---|---|
| `kingdom` / `kingdom show` | `showKingdom` | Stockpile + production + in-progress builds + in-progress training, line-per-field |
| `buildings [--upgradable]` | `listKingdomBuildings` | Status derived from `upgrade_possible` / `at_max_level` / `tier_gates_met` / `affordable` / active `build_order` |
| `build preview <kind>` | `previewBuildUpgrade` | Cost, duration, tier gates with per-prereq detail, per-resource affordability shortfall |
| `build <kind>` (or no arg → picker) | `previewBuildUpgrade` + `selector.Confirm` + `queueBuildOrder` | Selector over upgradable buildings when kind is omitted. Preview is **always** shown before the confirm prompt. Reads defensive `target_level` from the preview response |
| `build cancel <id-or-kind>` | `cancelBuildOrder` (resolves order via `showKingdom`) | Resolves a kind to its order ID via `showKingdom.in_progress_builds`. Multiple orders for the same kind → user picks by ID |

`buildingKinds` is a hard-coded slice of the §17 catalog so completion
runs without an HTTP call. A regression test in
[kingdom_test.go](../../internal/tui/verbs/kingdom_test.go) pins it
against `gen.BuildingUpgradePreviewKind.AllValues()` so a spec drift
fails loudly.

### [armies/train.go](../../internal/tui/verbs/armies/train.go) — Phase 8

| Verb | operationId | Notes |
|---|---|---|
| `train preview <building> <unit> <count>` | `previewTrainingOrder` | Validates kind names against the local mirrors of `gen.QueueTrainingOrderReqBuilding` / `gen.Unit`; surfaces affordability + max_affordable_count |
| `train <building> <unit> <count>` (or chain of pickers) | `previewTrainingOrder` + `selector.Confirm` + `queueTrainingOrder` | Walks `selector.Pick`(building) → `selector.Pick`(unit) → `selector.Form`(count) when args are omitted. Preview is **always** shown before the confirm. Surfaces `unit_trainable_here: false` as an advisory but lets the backend's 422 be authoritative |
| `train cancel <id-or-unit>` | `cancelTrainingOrder` (resolves via `showKingdom.in_progress_training`) | Mirrors `build cancel`'s ambiguity handling: 1 match → resolve; ≥2 → ask for the ID |

`trainingBuildings` and `unitKinds` are hard-coded mirrors of the spec
enums; regression tests in
[armies/train_test.go](../../internal/tui/verbs/armies/train_test.go)
pin them against `gen.QueueTrainingOrderReqBuilding.AllValues()` and
`gen.Unit.AllValues()`.

### [armies/armies.go](../../internal/tui/verbs/armies/armies.go) — Phase 8

| Verb | operationId | Notes |
|---|---|---|
| `armies` | `listKingdomArmies` | Table with name, status, region name (resolved via `ShowWorldMap` cache), capacity, and the short composition summary from `compositionString` |
| `army show <name>` | `ShowArmy` after `ResolveArmy` | Long form; the `status` field is the only signal that an army is on the move (no embedded march in v1 — flagged as backend co-evolution) |
| `army split <name>` | `SplitArmy` after `ShowArmy` | Builds a `huh` form with one numeric input per unit in the source composition (`max <count>` validator) plus a name field. Checks `status == home` client-side to fail fast; the backend's 422 is still authoritative |
| `army rename <name> <new-name>` | `RenameArmy` | Client-side regex `^.{1,60}$`; backend enforces uniqueness with 422 `name_taken` (wrapped as `code: invalid` in v1 — see co-evolution below) |
| `army merge <name> --into <other-name>` | `MergeArmy` after two `ResolveArmy` | `selector.Confirm` before commit. Backend enforces same-kingdom, same-region, both `home` |

### [armies/march.go](../../internal/tui/verbs/armies/march.go) — Phase 8

| Verb | operationId | Notes |
|---|---|---|
| `march <army> <target-region> [intent]` | `DispatchMarch` after `ResolveArmy` + `ResolveRegion` | Intent picker opens when `<intent>` is omitted, with one-line descriptions for each of the six intents. The dispatched march's `path` is resolved back to region names for display |
| `recall <army>` | `RecallMarch` | The spec returns an empty 404 (not an error envelope) when there's no active march; the wrapper synthesizes an `api.Error{Code: "not_found", Message: "no active march for this army"}` so the user-facing error line is meaningful |

`marchIntents` is pinned to `gen.MarchOrderIntent.AllValues()` by
[armies/march_test.go](../../internal/tui/verbs/armies/march_test.go).

### [battles/battles.go](../../internal/tui/verbs/battles/battles.go) — Phase 9

| Verb | operationId | Notes |
|---|---|---|
| `battles [--limit N] [--offset N]` | `ListKingdomBattles` | Newest-first table. `parseListFlags` rejects bad inputs client-side (positive int, ≤ 100 for `--limit`; non-negative int for `--offset`) before any HTTP call. The opponent column resolves to `(wilderness)` when `defender_kingdom_id` is empty, otherwise to the *other* side's kingdom ID — when the caller defended, that means the attacker. Region names come from `ShowWorldMap` via `regionNameMap` (mirror of armies); cross-world history falls back to ULIDs. The "more" hint at the bottom is only printed when `offset + len(list) < total_count` |
| `battle show <id>` | `ShowBattle` | No name → ULID resolver — battle IDs are ULIDs the user copies from `battles`. Tab completion is wired via `suggestBattleIDs`, which fetches the first page and returns each ID. The renderer marks the caller's side with `(you)`; optional fields (`attacker_title`, `defender_title`, `march_order_id`, `army_id`) collapse to `—` when absent. The round log is verbose (one block per round) and only prints the `walls:` line when walls took damage. 404 surfaces unchanged from the wrapper — both unknown IDs and someone-else's battles look the same to the caller |

`render.go` keeps the per-verb printers (`printBattleList`,
`printBattleDetail`) and a few helpers (`lootString`,
`compositionString`, `shortKingdom`, `formatOptFloat`) that other
files in the subpackage don't need.

### [render.go](../../internal/tui/verbs/render.go) — shared

Tiny shared printers — currently just `printProfileRead`,
`printProfileWrite`, `printStats`, and the `nilOrEmpty(gen.NilString)
string` helper.

---

## Backend co-evolution

The verbs package is the place where most "the backend's shape is
awkward for this UI" notes accrete. Per
[CLAUDE.md](../../CLAUDE.md) "Backend co-evolution", **default to
surfacing**:

- State the symptom in a code comment at the point of friction.
- Propose the minimal backend change.
- Let the user decide whether to open a backend issue/PR or accept
  the CLI-side workaround.

The existing list of flagged candidates as of Phase 8:

- `WorldSummary` lacks `my_kingdom` → `worlds` can't split
  member/eligible.
- No `listServerPlayers` endpoint → `player show` has no completer.
- No server-side `--owner` filter on `listNodes` → client-side
  filtering in `nodes`.
- No `(worldID, otherHandle) → kingdomID` resolver → later phases
  pull IDs out of list payloads.
- Phase 8: the auto-managed Garrison army has no `is_garrison: true`
  field — the CLI can't special-case it without guessing by name.
- Phase 8: `Army` lacks an embedded `current_march` pointer; `army
  show` cannot render march detail without a separate
  `march`/`armies` lookup.
- Phase 8: no `march preview` endpoint — `march` can 422 with
  `unreachable` only after commit. A `POST /armies/{id}/march/preview`
  mirroring `previewBuildUpgrade` would close the loop.
- Phase 8: `ErrorEnvelope.code` is constrained to a small enum
  (`invalid`, `not_found`, `forbidden`, `handle_locked`, …). The
  backend's documented Phase 8 codes (`incompatible_armies`,
  `name_taken`, `army_not_home`, `insufficient_units`, …) are not in
  the enum, so they arrive as `code: "invalid"` with the real code in
  `message` — the CLI can't switch on the specific cause.
- Phase 9: `Battle.defender_kingdom_id` is `type: string` and listed
  in `required:`, but a wilderness / ruin-claim battle has no
  defending kingdom — the backend has been emitting empty string in
  practice. Either marking the field `nullable: true` (and dropping
  it from `required`) or replacing it with a typed `battle_type`
  discriminator would let the CLI render wilderness vs PvP without
  the "empty string means wilderness" workaround.
- Phase 9: no `(world_id, kingdom_id) → owner_handle` resolver
  exists. `battles` and `battle show` render the opponent kingdom as
  a truncated ULID. Embedding `attacker_handle` / `defender_handle`
  on `Battle` (mirroring how `World.my_kingdom` was added) would let
  us print `vs IronFist`. Same gap applies to
  `BattleParticipant.kingdom_id`.
- Phase 9: `Battle.region_id` only resolves to a region name when
  the battle's world matches the in-scope one (we cache
  `ShowWorldMap` per world). For cross-world history — e.g. a
  player browsing an archived world they're no longer in —
  embedding `region_name` on `Battle` would close the gap.

Don't ship workarounds quietly.

---

## Adding a new verb

1. Decide the file based on logical area
   ([servers.go](../../internal/tui/verbs/servers.go),
   [worlds.go](../../internal/tui/verbs/worlds.go), …). If it's a new
   area, add a new file — the package is intentionally split by
   feature.
2. Add the wrapper method(s) to
   [internal/api/client.go](../../internal/api/client.go) per
   [02-api-client.md](02-api-client.md) "Adding a wrapper".
3. In the verbs file, write `init()` to call `shell.Register` with
   the verb's `Name`, `Summary`, `Usage`, `Run`, optional `Complete`
   / `Flags` / `Sub`.
4. Write the handler following the canonical shape above. Reuse
   `requireWorldID` / `requireKingdomID` for scope guards; reuse
   `Resolve<Entity>` for name→ID; reuse `shell.Strong` / `Section` /
   `Success` / `Info` for rendering.
5. Add a print helper if the result is non-trivial. Conventions:
   line-per-field, narrow, easy to copy/paste.
6. Write tests next to the file. The shell stubs out via a fake
   `*api.Client` (built against `httptest.Server`); the `StateStore`
   is a stub that records the snapshot.
7. Update [docs/tutorial.md](../tutorial.md) — same commit.
8. Update [commands-reference.md](commands-reference.md) — same
   commit.
