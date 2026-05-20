# 02 — API Client Wrapper

Phase 1 of [TODO.md](../../TODO.md). The hand-rolled wrapper at
[internal/api/](../../internal/api/) around the ogen-generated client.
Everything above this layer (verbs, shell, cobra commands) talks to
this package — nobody ever imports `internal/api/gen` directly.

If you're debugging "why does this HTTP call look like this?" or
"where do I add a wrapper for a new operation?", this is the file.

---

## Why a wrapper at all?

ogen emits a strongly-typed client where each operation returns a
**discriminated-union response interface** — e.g. `JoinWorld` returns
something matching `(*gen.Kingdom | *gen.JoinWorldForbidden |
*gen.JoinWorldNotFound | *gen.JoinWorldUnauthorized |
*gen.JoinWorldUnprocessableEntity)`. That's correct but inconvenient
for UI code, and it leaves five cross-cutting concerns to solve in one
place rather than scattered through every caller:

| Concern | Where it lives |
|---|---|
| Bearer token, lazily loaded per request | `bearerSource` in [security.go](../../internal/api/security.go) — adapts a `TokenProvider` to ogen's `gen.SecuritySource` |
| Error envelope decoding (`{error: {code, message, retry_after?}}` → `*api.Error`) | [error.go](../../internal/api/error.go) |
| `X-Request-Id` capture from every response | `requestIDTransport` in [transport.go](../../internal/api/transport.go) |
| 429 detection (the spec does not declare `TooManyRequests`) | also `transport.go` — produces `*api.RateLimitError` |
| Per-session name/slug → ULID memoization | [resolve.go](../../internal/api/resolve.go) |

The wrapper's public surface is the `Client` type
([client.go](../../internal/api/client.go)) with one method per
operation the CLI uses. Callers see `[]gen.ServerSummary`,
`*gen.World`, etc. (the ogen-generated schema types), but never the
per-operation `*…OK` / `*…NotFound` union variants.

---

## The `Client` struct

```go
type Client struct {
    gen    *gen.Client
    logger *slog.Logger
    cache  *resolverCache
}
```

`New(baseURL, tp TokenProvider, hc *http.Client)`
([client.go:32](../../internal/api/client.go#L32)) wires the three:

1. Clones `hc` (or makes a default one with a 30 s timeout) and wraps
   its `Transport` with `requestIDTransport`. Caller's `http.Client`
   is never mutated.
2. Builds a `gen.Client` against `baseURL` with a `bearerSource{tp:
   tp}` as the security source. Per-request bearer-token lookup is
   delegated to `tp.Token(ctx)` — see "Lazy bearer tokens" below.
3. Allocates a fresh `resolverCache`.

`baseURL` includes the version prefix (e.g.
`http://localhost:3000/v1`). The default in [internal/config](../../internal/config/config.go)
matches the Rails backend's dev port.

---

## The `call()` helper

Every wrapped operation goes through one private helper, `Client.call`
([client.go:74](../../internal/api/client.go#L74)). It is the single
place the five cross-cutting concerns live. Signature:

```go
func (c *Client) call(
    ctx context.Context,
    op string,
    run    func(ctx context.Context) (any, error),
    decode func(res any, requestID string) error,
) error
```

Flow per call:

1. **Stash respMeta in context.** `withMeta(ctx)` returns a derived
   context carrying a fresh `*respMeta`. The transport will write
   `X-Request-Id`, the HTTP status, and (on 429) a `*RateLimitError`
   into that pointer.
2. **Invoke `run`.** The caller's closure calls the ogen operation and
   returns its `any`-typed response union plus an error.
3. **Check for 429.** If the transport saw a `429`, it parsed the
   envelope and populated `meta.rateLimit`. That's surfaced as the
   error verbatim, **regardless of whether ogen also returned an
   `UnexpectedStatusCode` error for the same response** — the
   OpenAPI spec does not declare 429 responses, so ogen treats them
   as unexpected, but we want the typed form either way.
4. **Check the transport error.** Any other low-level error
   (connection refused, TLS, etc.) is wrapped as `fmt.Errorf("api %s:
   %w", op, err)` and logged at warn level.
5. **Decode the union.** The caller's `decode` closure type-switches
   on the response union. The convention is: success cases set the
   caller's captured output variable and return `nil`; per-status
   envelope types are passed through `fromEnvelope(...)` → `*Error`;
   the default branch returns `unexpectedRes` (see "Spec drift" below).
6. **Log + return.** A successful call emits a `debug` "api ok" line
   with `op`, `request_id`, `http_status`, and elapsed; failures emit
   a `warn` "api error" / "api transport error" line.

A representative wrapper method
([client.go:166](../../internal/api/client.go#L166)):

```go
func (c *Client) ListPlayerServers(ctx context.Context) ([]gen.ServerSummary, error) {
    var out []gen.ServerSummary
    err := c.call(ctx, gen.ListPlayerServersOperation,
        func(ctx context.Context) (any, error) { return c.gen.ListPlayerServers(ctx) },
        func(res any, rid string) error {
            switch v := res.(type) {
            case *gen.ListPlayerServersOK:
                out = v.Servers
                return nil
            case *gen.ErrorEnvelope:
                return fromEnvelope(v, rid)
            }
            return unexpectedRes(gen.ListPlayerServersOperation, res)
        },
    )
    return out, err
}
```

The pattern is: closures capture the typed output variable, the
type-switch covers exactly the response variants ogen generates, and
the wrapper returns `(output, error)` to the caller. The CLAUDE.md
"no generics" comment in `call()` calls this out — every operation's
response union is a different unrelated type, so explicit switching
beats a generic helper.

### Spec drift detection

`unexpectedRes` ([client.go:144](../../internal/api/client.go#L144))
returns `api $op: unexpected response type %T`. Hitting this in
practice means the OpenAPI spec gained a new response status and the
wrapper's switch is out of date. Run `make generate` and add the
missing case.

---

## Lazy bearer tokens

`TokenProvider` ([token.go](../../internal/api/token.go)) is the
abstract seam between this package and the credentials file:

```go
type TokenProvider interface {
    Token(ctx context.Context) (string, error)
}
```

The wrapper does **not** capture a token at `New(...)` time. Instead,
`bearerSource` ([security.go](../../internal/api/security.go))
implements ogen's `gen.SecuritySource` and delegates to
`tp.Token(ctx)` on every outgoing operation:

```go
func (s bearerSource) PlayerBearer(ctx, _ gen.OperationName) (gen.PlayerBearer, error) {
    if s.tp == nil { return gen.PlayerBearer{}, nil }
    tok, err := s.tp.Token(ctx)
    return gen.PlayerBearer{Token: tok}, err
}
```

The Phase 2 [auth.FileProvider](../../internal/auth/provider.go) is
backed by an `*auth.Store` that re-reads `Current` on every `Token()`
call. That is what lets `dun login` mutate the credentials file and
then immediately hand off to the shell without rebuilding the
`*api.Client` — the next outbound request picks up the fresh key.

Returning `("", nil)` from `Token` is **valid**, per the doc comment
on the interface. It means "no credentials available"; the backend
will respond with 401 and the caller can handle that as an
unauthenticated state.

`AdminBearer` returns an error: v1 is player-surface only, and we want
to fail loudly if a future spec change ever routes us through an admin
operation.

---

## The transport: `X-Request-Id` and 429

[transport.go](../../internal/api/transport.go) defines a custom
`http.RoundTripper` that wraps the caller's transport with one job:
capture per-response metadata so wrapper methods can read it after the
ogen call returns.

```go
type respMeta struct {
    requestID string
    status    int
    rateLimit *RateLimitError
}
```

`call()` stashes a fresh `*respMeta` in the request context via
`withMeta(ctx)`. The transport reads it back via
`metaFromCtx(req.Context())` and writes:

- `X-Request-Id` header into `requestID` (empty if the response didn't
  carry one).
- `StatusCode` into `status`.
- On a 429, the parsed `*RateLimitError` into `rateLimit`.

The transport never mutates the request. Authentication is handled
**upstream** by `bearerSource`; keeping the two concerns split means
each is testable in isolation and the transport stays trivial.

### Why parse 429 in the transport

The OpenAPI spec does not declare 429 responses for any operation. If
the backend returns one anyway, ogen treats the response as an
unexpected status and returns an `UnexpectedStatusCode` error. By
parsing the 429 body **before** ogen sees it, we:

- Always surface a typed `*RateLimitError` regardless of whether ogen
  also bubbled the unexpected-status error.
- Centralize body-reading and the `Retry-After` header fallback in one
  place (see `parseRateLimit` at
  [transport.go:83](../../internal/api/transport.go#L83)).
- Replace the body with `http.NoBody` so the downstream decoder
  doesn't observe a half-read stream.

There is **no automatic retry**. The CLI surfaces the rate-limit error
verbatim — the user decides what to do.

---

## Error envelope

[error.go](../../internal/api/error.go) defines the typed errors every
caller sees:

```go
type Error struct {
    Code       string  // stable machine-readable token from the backend
    Message    string  // human-readable; format is unstable
    RequestID  string  // X-Request-Id captured by the transport
    HTTPStatus int     // populated for rate-limit errors today
}

type RateLimitError struct {
    Err        *Error
    RetryAfter time.Duration
}
```

`fromEnvelope` converts the ogen-generated `*gen.ErrorEnvelope` into
the typed form, attaching the captured `requestID`. When the envelope
includes `retry_after`, the returned error is a `*RateLimitError`
wrapping the `*Error`.

`AsError(err)` is the helper UI code calls to extract a typed
`*Error` from any error this package might return, whether it's
wrapped in a `*RateLimitError` or not. It works through
`errors.As` so `fmt.Errorf("api %s: %w", op, …)` wrapping is
transparent.

The shell's error renderer
([shell/render.go](../../internal/tui/shell/render.go)) uses
`AsError`:

```
error: <message> (code=<code>, request_id=<id>)
```

When the error is not from this package (e.g. a transport failure
that didn't reach the response decoder), only the `Error()` string is
printed — code and request_id are omitted.

---

## The resolver cache

Users always type names — `acme`, `spring-2026`, `Greyhollow`,
`Vanguard`, `IronFist`. The backend uses ULIDs internally. The
resolvers in [resolve.go](../../internal/api/resolve.go) bridge the
two with a per-shell-session memoization cache.

```go
type resolverCache struct {
    mu       sync.RWMutex
    servers  map[string]string            // slug → ULID
    worlds   map[string]map[string]string // serverID → slug → ULID
    players  map[string]map[string]string // serverID → handle → ID-like
    regions  map[string]map[string]string // worldID → name → ULID
    armies   map[string]map[string]string // kingdomID → name → ULID
    kingdoms map[string]string            // worldID → caller's kingdom ULID
}
```

Each `Resolve<Entity>` method:

1. Read-locks the cache and returns the cached value on hit.
2. On miss, calls the relevant `list*` / `show*` wrapper method.
3. Write-locks, populates the entire returned slice (so the first map
   fetch pays for every subsequent name lookup in the session), and
   returns the requested ID or `notFound(kind, key)`.

`notFound` returns an `*Error` with `code: "not_found"` so callers can
treat client-side misses identically to a backend 404 — no special
case in render code, no separate `errors.Is(...)` check.

### The kingdom resolver is special

`ResolveKingdom(ctx, worldID)` returns the **caller's own** kingdom
ULID, pulled from `ShowWorld(...).MyKingdom`. There is no public
endpoint that maps `(worldID, otherHandle) → kingdomID` in v1; later
phases that need that mapping pull the ID out of `list*`/`show*`
payloads directly.

If the caller has no kingdom in that world, `MyKingdom` is unset and
the resolver returns `not_found`. Verbs translate this to a
user-facing `you have no kingdom in this world — try \`world join
<slug>\` first` (see `requireKingdomID` in
[verbs/kingdom.go](../../internal/tui/verbs/kingdom.go)).

### Invalidation

Mutating operations must invalidate the relevant cache slice **on
success**:

| Wrapper method | Invalidates |
|---|---|
| `JoinServer` | `InvalidateServers()` — caller is now a member, server cache is stale |
| `JoinWorld` | `InvalidateWorlds(serverID)` |
| `QueueBuildOrder`, `CancelBuildOrder` | `InvalidateKingdom(kingdomID)` (no-op today; reserved seam) |
| Phase 8+ army mutations | `InvalidateArmies(kingdomID)` |

The hooks are wired even when they are no-ops (`InvalidateKingdom`
today) so callers don't need to remember to add them when a later
phase extends what the cache holds for that entity.

### Concurrency

The shell is single-threaded today: readline blocks the main goroutine
while a verb runs. The cache still guards with `sync.RWMutex` for two
reasons:

- Tab completion runs Suggesters under a short timeout in a derived
  context (see [04-repl-shell.md](04-repl-shell.md)). A dynamic
  Suggester that consults a resolver runs on the readline goroutine
  too, but the mutex makes the contract explicit.
- Later phases may run a connectivity probe in parallel with the
  prompt. Guarding once is cheaper than retrofitting.

No resolver is on a hot path — at most one `list*` call per entity
kind per session.

---

## Adding a wrapper for a new operation

For each new player operation:

1. **Confirm it's in the generated client.** After `make generate`,
   `grep <OperationId> internal/api/gen/oas_operations_gen.go` should
   show the operation. If not, the spec mirror is stale — run
   `./scripts/sync-backend-docs.sh && make generate`.
2. **Add the wrapper method to [client.go](../../internal/api/client.go).**
   Use the existing methods as templates. Type-switch on the response
   union and cover every variant ogen emits; close with
   `unexpectedRes`.
3. **Pass the typed output back to the caller.** Schemas from
   `internal/api/gen` are fine to expose (`gen.ServerSummary`,
   `gen.World`, …); per-operation `*…OK` / `*…NotFound` types are
   not.
4. **Invalidate cache slices on success** if the mutation makes a
   memoized ID stale. Skip this for read-only operations.
5. **Add a test** under `internal/api/`. The pattern is an
   `httptest.Server` returning canned JSON; existing tests in
   [client_test.go](../../internal/api/client_test.go),
   [auth_ops_test.go](../../internal/api/auth_ops_test.go),
   [membership_ops_test.go](../../internal/api/membership_ops_test.go),
   [world_kingdom_ops_test.go](../../internal/api/world_kingdom_ops_test.go),
   and [resolve_test.go](../../internal/api/resolve_test.go) cover the
   shape.

The verbs that consume the new method are added under
[internal/tui/verbs/](../../internal/tui/verbs/) per
[05-verbs.md](05-verbs.md).
