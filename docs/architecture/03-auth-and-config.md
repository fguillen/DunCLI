# 03 — Auth, Config & Credentials

Phase 2 of [TODO.md](../../TODO.md). How dun-cli authenticates the
player, persists credentials, finds the backend URL, and exposes the
operational commands (`login`, `logout`, `keys`, `account`).

If you're debugging "where did the API key go?" or "why did my next
command pick up a stale token?", this is the file.

---

## The shape, in one minute

```
~/.dun/config.toml       (base URL — TOML, mode 0644)
        │
        ▼
internal/config.Load → *Config

~/.dun/credentials       (api keys — TOML, mode 0600)
        │
        ▼
internal/auth.LoadStore → *Store
                              │
                              ▼
                       auth.NewFileProvider(store) implements api.TokenProvider
                              │
                              ▼
                       api.New(baseURL, fp, hc) → *api.Client
```

Every cobra subcommand (and the bare-`dun` shell entry) starts with
`loadSession()` ([cmd/dun/client.go](../../cmd/dun/client.go)), which
wires those three calls together. The returned `*session` is the only
thing each command's `RunE` needs.

---

## `internal/config` — `~/.dun/config.toml`

[internal/config/config.go](../../internal/config/config.go) loads the
TOML file via viper. The file is optional — a missing file returns
defaults with no error so first-run users don't need to bootstrap one.
A present-but-malformed file is a typed error so the user gets a
clear "your config is broken" message instead of a silent fallback.

Today's `Config` struct has exactly one field:

```go
type Config struct {
    BaseURL string
}
```

`DefaultBaseURL` is `http://localhost:3000/v1` — matches the Rails
backend's local dev port. Later phases may grow this struct (theme
override, log level default, etc.); the empty TOML key falls back to
`DefaultBaseURL` via `v.SetDefault(...)`.

The path resolution rule from [01-foundations.md](01-foundations.md) is
firm here: `os.UserHomeDir() + "/.dun/" + FileName`. No XDG env var,
no `$DUN_HOME` override.

`Stat()` is exported so tests can assert presence/absence without
re-deriving the path.

---

## `internal/auth` — `~/.dun/credentials`

[internal/auth/store.go](../../internal/auth/store.go) is the
file-backed credentials store. One file holds every `(base_url,
email, scope)` credential the user has logged in with, plus two
pointer tables — `[current]` for the player surface and
`[current_admin]` for the admin surface — that each name the active
credential for their scope.

```toml
[current]
base_url = "http://localhost:3000/v1"
email = "fg@example.com"

[current_admin]
base_url = "http://localhost:3000/v1"
email = "fg@example.com"

[[credentials]]
base_url = "http://localhost:3000/v1"
email = "fg@example.com"
api_key = "k_abc…"
expires_at = 2026-08-18T12:00:00Z
# scope omitted ⇒ player

[[credentials]]
base_url = "http://localhost:3000/v1"
email = "fg@example.com"
api_key = "k_adm…"
expires_at = 2026-08-18T12:00:00Z
scope = "admin"
```

This shape supports a single user who works against multiple
backends (dev, prod), multiple identities per backend, **and** a
player + an admin key for the same email. The `scope` field (Phase
14; empty normalizes to `"player"`) makes the last case two distinct
entries that never clobber. `[current]` and `[current_admin]` are
independent, so the `dun>` and `dun-admin>` shells never overwrite
each other's active credential.

### `Store` API

```go
type Store struct {
    Current      currentRef // player [current]
    CurrentAdmin currentRef // admin  [current_admin]
    Credentials  []Credential
}

func LoadStore() (*Store, error)
func (s *Store) Save() error
func (s *Store) Upsert(c Credential)                          // keyed (base, email, scope)
func (s *Store) Get(baseURL, email, scope string) (Credential, bool)
func (s *Store) CurrentCredential() (Credential, bool)        // player
func (s *Store) CurrentAdminCredential() (Credential, bool)   // admin
func (s *Store) SetCurrent(baseURL, email string) error       // player
func (s *Store) SetCurrentAdmin(baseURL, email string) error  // admin
func (s *Store) Delete(baseURL, email, scope string)
func (s *Store) Clear()
```

- `LoadStore` returns an empty `Store` (no error) when the file
  doesn't exist; that's the first-run state.
- `Save` writes atomically: temp file in the same directory at mode
  0600, then `os.Rename`. The directory is created at mode 0700.
  Mid-write crashes leave the previous file intact.
- `Get` / `Upsert` / `Delete` key on `(base_url, email, scope)`; an
  empty scope normalizes to `"player"`. `SetCurrent` /
  `SetCurrentAdmin` reject an unknown `(baseURL, email)` for their
  scope — the credential must already exist via `Upsert`.
- `Delete` clears the matching pointer (`[current]` or
  `[current_admin]`) if it was pointing at the removed row.
- `Clear` wipes everything — every credential and both pointer
  tables; used by `dun account delete` after the backend has already
  revoked every key.

### `FileProvider` — the bridge to `internal/api`

[internal/auth/provider.go](../../internal/auth/provider.go) adapts a
`*Store` to `api.TokenProvider`. A `FileProvider` carries the scope it
serves: `NewFileProvider(store)` is player-scope, `NewAdminFileProvider(store)`
is admin-scope. `Token()` follows the matching pointer:

```go
func (p *FileProvider) Token(_ context.Context) (string, error) {
    if p == nil || p.store == nil { return "", nil }
    var c Credential
    var ok bool
    if p.scope == ScopeAdmin {
        c, ok = p.store.CurrentAdminCredential()
    } else {
        c, ok = p.store.CurrentCredential()
    }
    if !ok { return "", nil }
    return c.APIKey, nil
}
```

`loadSession()` builds a player client (`NewFileProvider`);
`loadAdminSession()` builds an admin client (`NewAdminFileProvider`).
The api.Client is otherwise identical — scope is decided entirely by
which provider it was handed (see [02-api-client.md](02-api-client.md)
"The two bearer seams").

Two implications:

1. **Token is read fresh on every API call.** `loadSession` builds an
   `*api.Client` once at the top of every command; the provider
   re-reads `store.Current` each time the client makes a request.
   That's what lets `dun login` mutate the in-memory store and then
   hand off to `enterShell` — the post-login HTTP calls pick up the
   new key without rebuilding the client.
2. **Empty token is a valid state.** `("", nil)` means "no
   credentials" and the backend responds 401. The CLI handles that
   exactly like any other API error — through `*api.Error` with code
   `unauthorized`.

---

## The magic-link login flow

[cmd/dun/login.go](../../cmd/dun/login.go) implements `dun login`.
There is no password anywhere — auth is magic-link only.

```
$ dun login
Email: user@example.com
Magic-link email sent. Check your inbox.
Token: <pasted from email>
Logged in as user@example.com (expires 2026-08-18).
```

Step by step:

1. **`loadSession()`** — picks up `BaseURL` from
   `~/.dun/config.toml` and an empty (or stale) `Store` from
   `~/.dun/credentials`.
2. **Prompt for email**, write to the running `cmd.InOrStdin()`. Empty
   email is a hard error.
3. **`client.RequestPlayerMagicLink(ctx, email)`** — calls
   `requestPlayerMagicLink`. The backend response is identical
   whether or not a Player already exists (no enumeration leak), so
   success here just means the mailer was enqueued.
4. **Prompt for token** from the email body. Empty token is a hard
   error.
5. **`client.ExchangePlayerMagicLink(ctx, token)`** — calls
   `exchangePlayerMagicLink` which returns `{api_key, expires_at,
   owner}`.
6. **Persist.** `store.Upsert(cred)` + `store.SetCurrent(...)` +
   `store.Save()`. The local entry only lands after the server-side
   exchange succeeds.
7. **Print expiry** and return to cobra.

### Post-login handoff

After `runLogin` succeeds, the cobra `RunE` doesn't exit — it calls
`postLoginShellOptions(ctx)` and then `enterShell(ctx, opts)`:

```go
func postLoginShellOptions(ctx) (shell.Options, error) {
    sess, _ := loadSession()
    list, _ := sess.client.ListPlayerServers(ctx)
    members := <slugs where Member == true>
    switch len(members) {
    case 0:  // no flags — user runs `servers` then `server join …`
    case 1:  opts.SingleMemberSlug = members[0]
    default: opts.PostLoginPicker = true
    }
}
```

The shell's `Options` (see [04-repl-shell.md](04-repl-shell.md))
either auto-scopes to the single membership or opens the post-login
picker for 2+ memberships. If the membership probe fails, the user
falls back to a bare shell with a one-line warning rather than being
locked out.

This is the only place in the CLI where one command (`login`) flows
into another long-running command (the REPL). All other Cobra
subcommands exit when they're done — except `dun admin login`, which
mirrors this handoff into the admin shell.

---

## Admin scope (Phase 14)

The admin surface reuses every piece above with two differences: a
different `scope` on the credential and a different pointer table.

- **`dun admin login`** ([admin_login.go](../../cmd/dun/admin_login.go))
  is `runLogin` with `requestAdminMagicLink` / `exchangeAdminMagicLink`
  in place of the player operations. It additionally checks the
  exchange response's `owner.type` is `admin` and refuses to persist
  the key otherwise — a guard against pasting a player token at the
  admin prompt. The credential is `Upsert`-ed with `Scope: ScopeAdmin`
  and `SetCurrentAdmin` points `[current_admin]` at it. On success it
  hands off to `enterAdminShell` (no membership picker — admin server
  verbs arrive in Phase 15).
- **`dun admin logout`** ([admin_logout.go](../../cmd/dun/admin_logout.go))
  is `runLogout` against `listAdminApiKeys` / `revokeAdminApiKey`,
  deleting the `ScopeAdmin` entry.
- **`loadAdminSession()`** is `loadSession()` built with
  `auth.NewAdminFileProvider`. Both go through a shared
  `loadSessionWith(newProvider)` helper in
  [cmd/dun/client.go](../../cmd/dun/client.go).

Key management for the admin surface is **not** a Cobra command — it
is the `keys` verb *inside* the `dun-admin>` shell (see
[05-verbs.md](05-verbs.md) "admin/"). The admin `keys revoke` of the
session's own current key clears the local admin credential the same
way `dun keys revoke` does for the player.

---

## Logout, key management, account deletion

The remaining Phase 2 commands all follow the same shape: load the
session, call one or two API methods, mutate the local store, save.

### `dun logout` — [logout.go](../../cmd/dun/logout.go)

1. `LoadStore` → must have a `CurrentCredential` (otherwise "nothing
   to log out of").
2. `ListPlayerAPIKeys` to discover the **id** of the current key. The
   `exchange` response does not include the id, so we have to list
   and find `current: true`.
3. `RevokePlayerAPIKey(currentID)` server-side.
4. Only on success: `store.Delete(...)` + `store.Save()`. If the
   server revoke fails, the local entry is preserved — a partial
   logout is worse than no logout.

### `dun keys list | revoke <id>` — [keys.go](../../cmd/dun/keys.go)

`list` is a one-shot tabwriter dump of `ListPlayerAPIKeys` with the
columns `ID | NAME | LAST USED | EXPIRES | STATUS`. Status is derived:
`revoked` (revoked_at set), `current` (current=true), or `active`.

`revoke <id>` is the manual form of logout: revoke a specific key by
ULID. The implementation lists first to detect whether the user is
revoking **their own** current key — if so, it also clears the local
credential after the server-side revoke succeeds. That matters for
the "I want to invalidate this machine's session" workflow.

### `dun account delete` — [account.go](../../cmd/dun/account.go)

A single y/N prompt
([deviation from "confirm twice"](../../TODO.md#phase-2--auth--account))
guards an irreversible action. On confirmation:

1. `DeleteAccount` — backend purges real name, anonymizes the
   per-server handle, zeros stats, revokes every ApiKey.
2. `store.Clear()` + `store.Save()` — local credentials are moot
   anyway since the server has already revoked them.

The prompt includes the last 4 chars of the API key
(`lastFour(cur.APIKey)`) so the user can sanity-check which account
they're about to nuke if they have multiple credentials configured.

---

## Why no OS keychain?

[CLAUDE.md](../../CLAUDE.md) "Storage layout" calls this out as a hard
rule. The trade-offs:

- **Pro keychain**: encrypted-at-rest by the OS, syncs with the
  user's existing credential manager, harder to exfiltrate via a
  read-only filesystem leak.
- **Pro file**: no platform-specific glue (`zalando/go-keyring` would
  drag in dbus on Linux), easy to inspect with `cat`, trivially
  scriptable (`DUN_API_KEY=$(grep api_key ~/.dun/credentials …)`),
  matches the dotfile conventions of `~/.aws/credentials`,
  `~/.kube/config`, `~/.netrc`.

We picked **file** because:

- Players running a strategy game in a terminal already trust their
  shell and home directory.
- The key is a 90-day rolling Bearer — losing one to a `~/.dun/`
  exfiltration is a defined incident (revoke + re-login), not a
  permanent compromise.
- Tests run against the real code path (`t.Setenv("HOME", tempDir)`),
  not a keychain stub.

The decision is reversible: a future `KeychainProvider` could
implement `api.TokenProvider` and slot in next to `FileProvider`
without changing the rest of the codebase.

---

## Tests

[internal/config/config_test.go](../../internal/config/config_test.go),
[internal/auth/store_test.go](../../internal/auth/store_test.go), and
[internal/auth/provider_test.go](../../internal/auth/provider_test.go)
cover:

- Missing-file behavior (no error, defaults applied).
- TOML round-trip — write, reload, compare (including the `scope`
  field and `[current_admin]`).
- File permissions — credentials file is mode 0600 after Save.
- `[current]` cleared when the active credential is deleted.
- `FileProvider` re-reads on every call (mutate `store.Current`, then
  call `Token()` again — fresh value comes back).
- Player and admin credentials for the same `(base_url, email)`
  coexist as two entries; `NewAdminFileProvider` yields the admin key,
  `NewFileProvider` the player key.

[cmd/dun/login_test.go](../../cmd/dun/login_test.go),
[cmd/dun/logout_test.go](../../cmd/dun/logout_test.go) and
[cmd/dun/admin_test.go](../../cmd/dun/admin_test.go) use an
`httptest.Server` to stub the backend and feed canned input via
`cmd.SetIn(strings.NewReader(...))` — the magic-link prompts (player
and admin) are exercised without a real terminal.

Filesystem isolation is `t.Setenv("HOME", t.TempDir())` for every
test that touches `~/.dun/`; no test ever pollutes the developer's
real dotfolder.
