# 04 — REPL Shell

Phase 3 of [TODO.md](../../TODO.md). The long-running `dun>` prompt
that replaces the Phase 0 splash: the dispatcher, the verb registry,
tab completion, per-session `Context`, the `~/.dun/state.json`
persistence, and the built-in verbs.

If you're debugging "where does typed input go?", "why isn't my new
verb showing up?", or "why did the prompt lose my scope?", this is
the file.

---

## Shape

```
shell.Run(ctx, baseURL, email, *api.Client, Options)
  │
  ├─► build Session{ API, Cfg, Creds, Context, Out, State }
  ├─► register built-in verbs (help, quit, exit, clear, version, whoami, where)
  ├─► API.Health(ctx)            (connectivity probe; fail-loud)
  ├─► postLoginPicker hook        (optional, only after `dun login` w/ ≥2 servers)
  ├─► newReadline(sess)           (history at ~/.dun/history, completer wired)
  └─► for {                       prompt loop
        line := rl.Readline()
        Dispatch(ctx, sess, line) → leaf Verb.Run → render to scrollback
        sess.State.Save(scope.Snapshot())
      }
```

The shell does **not** use `tea.WithAltScreen()` — scrollback is
preserved so users can copy/paste lines, search backwards through
output, etc. Individual verbs MAY open transient Bubble Tea programs
in alt-screen mode (see [06-tui-primitives.md](06-tui-primitives.md))
and return control here when done.

There is no background goroutine. No `tea.Tick`, no async refresh, no
toast queue. Every render is the response to a user-typed verb — see
[PRODUCT.md](../../PRODUCT.md) anti-goals and
[README.md](README.md) "The mental model in one minute".

---

## The verb registry

[verb.go](../../internal/tui/shell/verb.go) defines the registry and
the `Verb` type:

```go
type Verb struct {
    Name     string
    Sub      map[string]*Verb
    Summary  string
    Usage    string
    Run      Handler                  // leaf verb has Run set
    Complete Suggester                // positional-arg completion
    Flags    []FlagSpec               // --flag specs
}

type Handler func(ctx, sess, args, flags) error
```

`Verb.IsLeaf()` reports whether `Run` is non-nil. A non-leaf Verb
delegates via `Sub` — `server` is a branch, `server.join` is a leaf.

### Package-level registry

```go
var defaultRegistry = &registry{verbs: map[string]*Verb{}}

func Register(v *Verb)       // panic on duplicate name
func Resolve(name string)    (*Verb, bool)
func Verbs() []*Verb         // every top-level verb, sorted
```

Init order does not matter: every file under
[internal/tui/verbs/](../../internal/tui/verbs/) calls
`shell.Register(...)` from `init()`, and `cmd/dun/main.go` brings them
all in with a blank `_ "…/verbs"` import. By the time `shell.Run`
reads the registry, every verb is present.

`Register` panics on duplicate — that's a programming bug (two files
registering the same name), never a runtime condition. Aliases like
`quit` / `exit` are intentionally registered as two distinct verbs
that share a Handler closure.

### Why a registry, not direct imports?

If `shell` imported `verbs` directly, every game change would force a
recompile of the shell package. The registry keeps the shell free of
game-specific imports — it only knows the `Verb` shape. Concretely,
this also breaks an import cycle: verbs need `shell.Session`,
`shell.Register`, `shell.Err`, etc., so they import `shell`; if
`shell` also imported `verbs` we'd be stuck.

The same indirection lets the shell expose **hooks** (like
`SetPostLoginPicker`) that the verbs package can register without
introducing a back-reference.

---

## Parse + Dispatch

[dispatcher.go](../../internal/tui/shell/dispatcher.go) is the
two-phase pipeline.

### Parse

`Parse(line)` ([dispatcher.go:30](../../internal/tui/shell/dispatcher.go#L30))
splits the line via `shlex.Split` (so `--real-name "Foo Bar"` works),
walks the verb tree following each token, then drains the remaining
tokens into positional `Args` and `--flag value` pairs.

```go
type ParsedCommand struct {
    Verb  *Verb              // resolved leaf
    Path  []string           // ["server", "join"]
    Args  []string           // positional after the verb path
    Flags map[string]string  // --name → value, "true" for boolean
}
```

Flag handling:

- `--name=value` and `--name value` are both accepted.
- An unknown `--name` is rejected with `unknown flag --X for ...`.
- A `--flag` declared with `HasValue: false` rejects an inline value
  and is recorded as `"true"`.
- A `--flag` declared with `HasValue: true` consumes the next token
  if no inline `=` was supplied.

Errors from `Parse` are surfaced through `shell.Err` and the loop
continues — a typo doesn't drop the user out of the shell.

### Dispatch

`Dispatch(ctx, sess, line)`
([dispatcher.go:119](../../internal/tui/shell/dispatcher.go#L119))
calls `Parse`, then `cmd.Verb.Run(ctx, sess, cmd.Args, cmd.Flags)`.
Two return cases matter:

- `ErrExit` (from `quit` / `exit`) — propagates up; `shell.Run`
  persists state and returns nil.
- Any other error — rendered via `shell.Err`, dispatcher returns
  `nil` so the loop continues.

The render layer ([render.go](../../internal/tui/shell/render.go))
formats `*api.Error` as `error: <msg> (code=<code>, request_id=<id>)`
via `api.AsError`. Non-API errors print only the `Error()` message.

---

## Tab completion

[completion.go](../../internal/tui/shell/completion.go) adapts the
verb registry to `readline.AutoCompleter`. Completion runs
**synchronously on every Tab keystroke**, so an unbounded Suggester
would freeze the prompt — `runSuggester` enforces an 800 ms timeout
on dynamic candidates and swallows errors (best-effort by design).

```
const completeTimeout = 800 * time.Millisecond
```

The completer figures out which "slot" the cursor is in by walking
the partial token list:

1. **Empty prefix** → top-level verb names.
2. **Branch verb, no further token** → its sub-verb names.
3. **Leaf verb + last token starts with `--flagName`** → flag-value
   candidates from `FlagSpec.Suggest` (if `HasValue=true`).
4. **Leaf verb + cur word starts with `--`** → list of `--flagName`s.
5. **Otherwise** → positional candidates from `Verb.Complete`.

`tokenizePartial` uses `strings.Fields` rather than `shlex.Split`:
while typing, an unterminated quote is normal (`profile set
--real-name "F`) and `shlex` would error. We need the completion
engine to keep working through that state.

### Suggester contract

```go
type Suggester interface {
    Suggest(ctx context.Context, sess *Session, prefix string) ([]string, error)
}

type SuggestFunc func(ctx, sess, prefix) ([]string, error)
```

Dynamic Suggesters that hit the network MUST honor `ctx` — readline
calls completion synchronously, so a wedged Suggester freezes the
prompt. `runSuggester` enforces the timeout but the Suggester is on
the hook for actually returning when cancelled.

Static Suggesters return the same fixed list every time:

```go
shell.SuggestFunc(func(_, _, _) ([]string, error) {
    return []string{"mine", "wild", "captured", "home-hoard"}, nil
})
```

Dynamic Suggesters consult the api client — typically via a resolver:

```go
shell.SuggestFunc(func(ctx, sess, _) ([]string, error) {
    list, err := sess.API.ListPlayerServers(ctx)
    // … return slugs
})
```

The api client's resolver cache absorbs repeated calls within the
800 ms timeout — the first Tab does the HTTP round-trip, subsequent
Tabs in the same session are instant.

---

## Per-session `Context` — server/world/kingdom scope

[context.go](../../internal/tui/shell/context.go) holds the in-memory
mutable scope tuple every verb reads to decide which entity to act
on when the user omits an explicit reference:

```go
type Context struct {
    mu            sync.RWMutex
    serverSlug    string
    worldSlug     string
    kingdomHandle string
}
```

Verbs read it via the getter methods (`ServerSlug()`, `WorldSlug()`,
`KingdomHandle()`). Verbs MUST mutate through the setters (`SetServer`,
`SetWorld`, `SetKingdomHandle`) — the setters embed the
narrower-scope-invalidation rules:

- `SetServer(newSlug)` clears `worldSlug` and `kingdomHandle` when the
  server changes. They belong to the previous server.
- `SetWorld(newSlug)` clears `kingdomHandle` when the world changes.
  Each world has its own handle.

That keeps stale references from leaking across scope changes
silently. The mutex is overkill today (the shell is single-threaded)
but makes the contract explicit for the connectivity-probe goroutine
later phases may add.

### `Snapshot` + `Apply`

`Snapshot()` returns the value-type `ContextSnapshot` used for
persistence and `Apply(snap)` overwrites the in-memory state from
one. Both go through the same mutex.

---

## State persistence — `~/.dun/state.json`

[state.go](../../internal/tui/shell/state.go) writes the
`ContextSnapshot` to disk after every successful dispatch so a
mid-session crash doesn't lose the most recent scope change.

The on-disk shape carries the active credential too:

```go
type stateFile struct {
    BaseURL       string
    Email         string
    ServerSlug    string
    WorldSlug     string
    KingdomHandle string
}
```

On `Load`, if the persisted `(BaseURL, Email)` doesn't match the
caller's, the snapshot is silently discarded. That's the
multi-account safety net: a `dun login` as a different identity (or a
manual credentials edit) doesn't re-apply someone else's scope on
shell startup.

The file is written atomically: temp file in the same dir, mode 0644,
then `os.Rename`. Same pattern as `auth.Store.Save`.

### When `Save` runs

- On every successful `Dispatch` in `shell.Run` — line 130 of
  [shell.go](../../internal/tui/shell/shell.go).
- Inside any verb that mutates `Context` and wants the state to
  survive a crash before the dispatcher's catch-all save can run.
  These look like:
  ```go
  sess.Context.SetServer(...)
  if err := sess.State.Save(sess.Context.Snapshot()); err != nil {
      shell.Err(sess.Out, fmt.Errorf("persist state: %w", err))
  }
  ```
- On Ctrl-D (`io.EOF` from readline) and on `ErrExit` (the `quit` /
  `exit` verbs).

A persistence failure inside a verb is non-fatal — the in-memory state
is still correct; the error is surfaced so the user knows the next
session may not reload this scope.

### Why is `State` an interface on `Session`?

```go
type StateStore interface {
    Save(ContextSnapshot) error
}
```

Verb tests pass a stub `StateStore` so they don't have to touch the
real filesystem. The shell hands the real `*FileStore` in via
`shell.Run`.

---

## The `Session` struct

```go
type Session struct {
    API     *api.Client
    Cfg     ConfigSnapshot       // { BaseURL string }
    Creds   CredentialSnapshot   // { Email string }
    Context *Context
    Out     io.Writer            // os.Stdout in prod; *bytes.Buffer in tests
    State   StateStore
}
```

Every verb handler receives a `*Session`. The intent is:

- `sess.API` is the wrapped client from
  [02-api-client.md](02-api-client.md).
- `sess.Cfg` and `sess.Creds` are read-only views — verbs never
  rewrite the config or credentials directly.
- `sess.Context` is the mutable scope.
- `sess.Out` is the rendering destination — never `os.Stdout`
  directly. Tests inject a `*bytes.Buffer`.
- `sess.State` is the persistence interface.

---

## Built-in verbs

[builtin.go](../../internal/tui/shell/builtin.go) registers the
verbs that have **no API dependencies** and are part of the shell's
contract regardless of which game verbs are linked:

| Verb | Purpose |
|---|---|
| `help [verb]` | List verbs (sorted) or print one verb's usage/flags |
| `quit` / `exit` | Returns `ErrExit` so the loop terminates |
| `clear` | ANSI clear (`\033[H\033[2J`) |
| `version` | Prints the `Options.Version` string |
| `whoami` | Prints the active credential's email + base URL |
| `where` | Prints the active (server, world, kingdom) scope |

`whoami` and `where` are deliberately built-ins rather than game
verbs: they read from `sess.Creds` / `sess.Context`, no HTTP call, no
API client dependency. They live here so the shell remains usable
even if the verbs package is empty.

The `join` verb is registered by the verbs package, not built-in,
because its sugar form (`join world <slug>`) calls into a game verb.

### Idempotent registration

```go
var builtinsRegistered bool
func registerBuiltinsOnce(version string) { ... }
```

A test that invokes `shell.Run` multiple times in-process would
otherwise trip the duplicate-name panic in `Register`. The guard
makes the shell test-friendly without weakening the registration
contract.

---

## Connectivity probe — fail fast and loud

```go
if err := client.Health(ctx); err != nil {
    Err(sess.Out, fmt.Errorf("backend unreachable: %s: %w", cfgBaseURL, err))
    return err
}
```

Before the prompt loop starts, `shell.Run` calls
`client.Health(ctx)` (i.e. `GET /v1/health`). If the backend is
unreachable, the user sees:

```
error: backend unreachable: http://localhost:3000/v1: ... (code=..., request_id=...)
```

…and the process exits non-zero. That's intentional: the alternative
is the user typing `servers` at the prompt and getting a confusing
generic API error for the **second** failed call. Surfacing the first
failure with full context — base URL, code, request id — saves a
support round-trip.

---

## The post-login picker hook

`shell.SetPostLoginPicker(f)` is the registration hook that lets the
verbs package install a function the shell can call without
importing verbs:

```go
var postLoginPicker func(ctx, *Session) error

func SetPostLoginPicker(f func(ctx, *Session) error) {
    postLoginPicker = f
}
```

The verbs package calls `SetPostLoginPicker(postLoginServerPicker)`
from `init()` ([verbs/servers.go](../../internal/tui/verbs/servers.go)).
`shell.Run` calls it when `Options.PostLoginPicker == true` — set by
`cmd/dun/login.go` after a successful magic-link exchange where the
player has 2+ server memberships. A cancellation from the picker is
soft (the user can `server join …` later); other errors are rendered
to scrollback and the shell continues.

Same pattern can be used for any other "shell needs to call game
code" callback — keep the hook tiny, declare a single variable, and
let the verbs package set it from `init`.

---

## Tests

- **Parse** — [dispatcher_test.go](../../internal/tui/shell/dispatcher_test.go)
  table-drives the parser through verbs, sub-verbs, positional args,
  inline / spaced flags, unknown verbs, unknown flags, missing
  values, and unclosed quotes.
- **Completion** — [completion_test.go](../../internal/tui/shell/completion_test.go)
  covers each of the five "slot" cases above; uses static Suggesters
  to keep the test deterministic.
- **State** — [state_test.go](../../internal/tui/shell/state_test.go)
  exercises the round-trip and the foreign-credential isolation rule.

The teatest snapshot tests for the readline prompt itself are
deferred (see [TODO.md](../../TODO.md) "Phase 3 — Interactive shell"
final bullet) — the dispatcher, parser, and completion engine are
covered without a real terminal, which is the high-value surface.
