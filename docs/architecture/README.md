# dun-cli — Client Architecture

This directory explains **how dun-cli is built** — the layers under the
`dun>` prompt, where each persistent file lives, and how a typed verb
travels from the keyboard to a backend HTTP call and back.

It is _not_:

- A product description — see [PRODUCT.md](../../PRODUCT.md).
- A user walkthrough — see [docs/tutorial.md](../tutorial.md).
- A request/response contract — see
  [docs/backend/openapi.yaml](../backend/openapi.yaml) and
  [docs/backend/api-endpoints.md](../backend/api-endpoints.md).
- A roadmap — see [TODO.md](../../TODO.md).

The intended reader is a developer about to **change** dun-cli: a new
verb, a new persistence concern, a bug fix in the dispatcher. The docs
answer "what already exists, why is it shaped this way, and where do I
plug in?"

---

## How to read these docs

The docs go from **outside-in**: each file zooms one level deeper.

1. **System overview** (this file) — actors, the layered shape of the
   client, and what one keystroke triggers.
2. **Foundations** ([01-foundations.md](01-foundations.md)) — Phase 0
   ground rules: the Go stack, project layout, `~/.dun/` storage
   contract, build commands, logging.
3. **API client wrapper** ([02-api-client.md](02-api-client.md)) —
   Phase 1: how `internal/api` wraps the ogen-generated client (auth,
   error envelope, `X-Request-Id` propagation, 429 handling, the
   resolver cache).
4. **Auth, config & credentials**
   ([03-auth-and-config.md](03-auth-and-config.md)) — Phase 2: magic-
   link login, `~/.dun/credentials`, `~/.dun/config.toml`, the
   `FileProvider` that lets the api client re-read tokens lazily, and
   the `dun login | logout | keys | account` Cobra commands.
5. **REPL shell** ([04-repl-shell.md](04-repl-shell.md)) — Phase 3:
   the `dun>` prompt, the verb dispatcher and registry, tab
   completion, per-session `Context`, the `~/.dun/state.json`
   persistence, and the built-in verbs (`help`, `whoami`, `where`,
   `clear`, `version`, `quit`, `join`).
6. **Game verbs** ([05-verbs.md](05-verbs.md)) — Phases 4-7: how
   `internal/tui/verbs/*.go` files register into the shell, the
   require-scope helpers (`requireWorldID`, `requireKingdomID`), the
   resolve-name-then-call pattern, dynamic Suggesters, and a per-file
   tour (servers, profile, player, worlds, regions, kingdom).
7. **TUI primitives** ([06-tui-primitives.md](06-tui-primitives.md)) —
   the transient Bubble Tea programs verbs reach for: `selector.Pick`
   (list picker), `selector.Confirm` (y/N), `selector.Form` (huh
   multi-field), the shared Lipgloss `theme` palette, and the rules
   that keep scrollback intact.
8. **Command reference** ([commands-reference.md](commands-reference.md)) —
   every shipped verb and Cobra command, cross-linked to the
   `operationId` it calls.

Phases 8–14 of [TODO.md](../../TODO.md) are not yet shipped; their
slots in this doc set will fill in as the work lands.

---

## The mental model in one minute

```
Terminal
   │  keystroke
   ▼
cmd/dun (cobra root)
   │  bare `dun`  ─────────►  shell.Run (REPL)
   │  `dun login`/`logout`/`keys`/`account`  (one-shot)
   ▼
internal/tui/shell
   │  prompt (readline) → Parse → Dispatch
   ▼
internal/tui/verbs/*           ◄── registered via init()
   │  business logic for one verb
   │  reads session.Context (server/world/kingdom scope)
   ▼
internal/api (Client wrapper)
   │  ResolveServer / ResolveWorld / ResolveKingdom (cached)
   │  call(): respMeta + error envelope + X-Request-Id
   ▼
internal/api/gen (ogen-generated)
   │  typed request + response unions
   ▼
HTTP  (Authorization: Bearer <api_key>)
```

The arrows are all synchronous. **There is no background goroutine
polling the backend** — every render is the response to a user-typed
verb. See [PRODUCT.md](../../PRODUCT.md) anti-goals and
[CLAUDE.md](../../CLAUDE.md) "Interaction model — REPL, not full-screen
TUI".

---

## The layers, from outside-in

| Ring | Package | Owns | Read for context |
|------|---------|------|------------------|
| **Entrypoint** | `cmd/dun` | cobra root, `version`, `login`, `logout`, `keys`, `account`, the `enterShell` handoff | [01-foundations.md](01-foundations.md), [03-auth-and-config.md](03-auth-and-config.md) |
| **REPL shell** | `internal/tui/shell` | prompt loop, verb registry, dispatcher, tab completion, session `Context`, `~/.dun/state.json` | [04-repl-shell.md](04-repl-shell.md) |
| **Verbs** | `internal/tui/verbs` | game commands (`servers`, `world`, `map`, `kingdom`, `build`, …) registered via `init()` | [05-verbs.md](05-verbs.md) |
| **TUI primitives** | `internal/tui/selector`, `internal/tui/theme` | `Pick` / `Confirm` / `Form` transient Bubble Tea programs, Lipgloss palette | [06-tui-primitives.md](06-tui-primitives.md) |
| **API wrapper** | `internal/api` | bearer auth, error envelope, `X-Request-Id`, 429, resolver cache | [02-api-client.md](02-api-client.md) |
| **Generated client** | `internal/api/gen` | ogen-emitted typed client, schemas, response unions | regenerated by `make generate` — never hand-edit |
| **Storage** | `internal/config`, `internal/auth`, `internal/log` | `~/.dun/{config.toml, credentials, history, state.json, dun-cli.log}` | [01-foundations.md](01-foundations.md), [03-auth-and-config.md](03-auth-and-config.md) |

Each ring depends only on rings inside it. In particular, `shell`
never imports `verbs` — verbs register themselves via `init()` and the
shell discovers them through the package-level registry. `cmd/dun`
imports `verbs` only as a blank `_ "…/verbs"` so the init side-effects
fire.

---

## What one verb does, end-to-end

The journey of `world join spring-2026` typed at the prompt:

1. **Read line.** `readline` returns the string; `shell.Run`
   forwards it to `shell.Dispatch`.
2. **Parse.** `shell.Parse` uses `shlex.Split`, walks the verb tree
   (`world` → `join`), and packages `args=["spring-2026"]` plus a
   `ParsedCommand` pointing at the leaf `Verb`.
3. **Run.** `cmd.Verb.Run(ctx, sess, args, flags)` calls
   `verbs.runWorldJoin`.
4. **Scope guard.** The verb reads `sess.Context.ServerSlug()` — bails
   with `"not in a server scope — try `server join <slug>` first"` if
   empty. (No HTTP call burned.)
5. **Resolve names → IDs.** `sess.API.ResolveServer("acme")` and
   `sess.API.ResolveWorld(serverID, "spring-2026")` consult the
   per-session `resolverCache`. On miss they call `listPlayerServers`
   / `listServerWorlds` and memoize.
6. **API call.** `sess.API.JoinWorld(ctx, worldID, serverID)` invokes
   the ogen-generated `JoinWorld`. The wrapper's `call()` helper
   stashes a `respMeta` in the context, the transport captures
   `X-Request-Id` and the status code, and the response union is
   matched to either the success type (`*gen.Kingdom`), a per-status
   error (`*gen.JoinWorldForbidden`, …), or `unexpectedRes`.
7. **Cache invalidation.** `JoinWorld` calls
   `c.InvalidateWorlds(serverID)` on success so a stale slug→ID
   mapping isn't served the next time.
8. **State mutation.** The verb calls `sess.Context.SetWorld(slug)`,
   then `sess.State.Save(sess.Context.Snapshot())` to write
   `~/.dun/state.json` atomically.
9. **Render.** `shell.Success(...)` styles the line via the theme
   palette and writes to scrollback. Errors flow through `shell.Err`
   which formats `*api.Error` as `error: <msg> (code=<code>,
   request_id=<id>)`.
10. **Persist.** After every successful dispatch (regardless of
    whether the verb mutated `Context`), `shell.Run` saves
    `~/.dun/state.json` so a crash mid-session doesn't lose scope.

Every later phase plugs into this loop the same way: a new file under
`internal/tui/verbs/`, an `init()` that calls `shell.Register`, a
handler that resolves names → IDs, calls one wrapped API method, and
renders.

---

## What is _not_ here

- **Phases 8–14** — military, combat, nodes/ruins, trade, wonders,
  archive/hall-of-fame, packaging. None shipped yet; their docs will
  land with the work.
- **Admin surface** — explicitly out of scope for v1. The
  `bearerSource.AdminBearer` security source returns an error rather
  than sending an empty token.
- **Backend internals** — see the upstream
  [docs/backend/](../backend/) mirror and the dun backend repo.

When a phase ships, add a new section file or extend the existing one
and update both this README and
[commands-reference.md](commands-reference.md).
