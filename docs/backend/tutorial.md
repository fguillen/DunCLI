# Tutorial

A hands-on walkthrough of the Dun API from the perspectives of the two roles that talk to it: **Admins** (who run servers and host worlds) and **Players** (who join worlds and play the game). Each section walks through one task end-to-end with the exact requests you need.

For the full request/response contract, see [openapi.yaml](openapi.yaml). For game rules, see the [Game Design Document](dun%20Game%20Design%20Document.v3.md). For how the backend is built, see [architecture/](architecture/).

---

## 1. Before You Start

### 1.1 Prerequisites

You'll need:

- **A running Dun backend.** Locally, that's `bin/dev` from the repo root — it starts the web server on `http://localhost:3000` and the Solid Queue worker that processes jobs (mail, ticks, march arrivals). Hosted instances substitute their own base URL.
- **An HTTP client.** All examples in this guide are `curl`, but `httpie`, Postman, or your language's preferred client will do.
- **An email inbox you can read.** Magic-link tokens are mailed; you'll paste them back into the API. In development, `letter_opener` opens mailed messages in your browser instead of sending them.
- **Optional but recommended:** a JSON pretty-printer like `jq` for reading responses.

### 1.2 Conventions used in this guide

- **Base URL.** Every example below uses `http://localhost:3000/v1`. Swap in your environment's host as needed.
- **JSON in, JSON out.** Send `Content-Type: application/json` on any request with a body. Responses are always JSON, including errors.
- **Auth.** Every authenticated request carries `Authorization: Bearer <api_key>`. Player keys work only on player routes, admin keys only on admin routes — sending one to the other returns `401`.
- **Error envelope.** Failures look like `{ "error": { "code": "string_code", "message": "...", "retry_after": 30 } }`. `retry_after` appears only on rate-limit (`429`) responses.
- **Request IDs.** Every response echoes `X-Request-Id` (auto-generated if absent). The same id surfaces in lograge JSON logs and OpenTelemetry traces — quote it when reporting bugs.
- **Timestamps and dates.** All API timestamps are ISO 8601 UTC (`2026-05-17T14:30:00Z`). Dates in prose use `YYYY-MM-DD`.
- **Placeholders.** Examples use copy-paste-ready values like `k_live_abc123...` for keys and `01HK...` for IDs. Substitute the real ones from the previous step's response.

### 1.3 Core concepts at a glance

A one-page glossary; the rest of the tutorial assumes these.

- **Admin.** Runs servers, hosts worlds, configures access. Magic-link auth at `/v1/admin/auth/...`.
- **Player.** Joins worlds, plays the game. Magic-link auth at `/v1/auth/...`. A single human can be both — they sign in once on each surface.
- **Server.** Top-level tenant. Owns admins, members, and worlds. Players cannot see across server boundaries — non-members get `404`, not `403`.
- **ServerMembership.** A player's link to a server. Carries their per-server `handle` and `real_name`.
- **ServerAccess.** Admission rule: either a `domain` glob match (e.g. `*@acme.test`) or an `invite` email entry. The API surface currently exposes invite-kind rows only.
- **World.** A single round of play hosted on a server. Has a lifecycle: `proposed → grace → active → (archived | cancelled)`.
- **Round.** Synonym for a world's full lifetime from T0 to archive. Stats and Hall of Fame are per-server, summed across rounds.
- **Kingdom.** A player's instance inside one world. Holds resources, buildings, armies, and (eventually) a Wonder.
- **Region.** One tile on the world map. Has terrain, may contain nodes and ruins, and may host armies and Wonders.
- **Node.** A capturable resource-production point inside a region. Wilderness nodes have NPC garrisons; home-hoard nodes produce passively.
- **Army.** A group of units belonging to one kingdom. Lives at a region; can march, split, merge, recall.
- **Caravan.** A march with intent `caravan` — moves resources between kingdoms. Public ledger records every dispatch.
- **Wonder.** Endgame mega-build. Foundation → Construction (90h) → Consecration (24h). Surviving Consecration ends the round.
- **Archive.** Frozen, read-only snapshot of a world's final state. Created at round end.

---

## 2. Admin Walkthrough

The journey of someone who wants to host games for others — from zero to a running world with players in it.

### 2.1 Get authorized

Every admin endpoint needs an admin-scope Bearer API key. You get one by exchanging a single-use magic-link token that the backend mails to your address.

**Step 1 — Ask for the magic link.**

```bash
curl -X POST http://localhost:3000/v1/admin/auth/magic_link \
  -H 'Content-Type: application/json' \
  -d '{"email": "boss@acme.test"}'
```

`202 Accepted`, empty body. The backend enqueues `MagicLinkMailer#send_link` with `scope: "admin"`. The link is single-use and expires 15 minutes after it's mailed.

Open the email, grab the token from the link, and copy it.

**Step 2 — Exchange the token for an ApiKey.**

```bash
curl -X POST http://localhost:3000/v1/admin/auth/exchange \
  -H 'Content-Type: application/json' \
  -d '{"token": "PASTE_TOKEN_FROM_EMAIL"}'
```

`201 Created`. The fields you care about:

```json
{
  "api_key": "k_live_abc123...",
  "expires_at": "2026-08-15T10:00:00Z",
  "owner": { "type": "Admin", "...": "..." }
}
```

The `api_key` is shown **once** — the database stores only its SHA-256 digest. Copy it now; if you lose it, request a fresh magic link.

**Step 3 — Use it on every subsequent request.**

```bash
curl http://localhost:3000/v1/admin/servers \
  -H 'Authorization: Bearer k_live_abc123...'
```

Each authenticated request slides `expires_at` forward by 90 days, so an active admin's key effectively never expires — an untouched one does.

**Gotchas**
- Tokens are single-use. Re-clicking a consumed link returns `401`.
- An *admin*-scope token sent to the *player* exchange endpoint (and vice versa) returns `401`. Wrong scope = wrong endpoint.
- Missing header, expired key, or revoked key all return `401` with the standard error envelope.

See [openapi.yaml](openapi.yaml) — operations `requestAdminMagicLink` and `exchangeAdminMagicLink`.

### 2.2 Manage your API keys

Once authorized you can inspect the keys issued to you and revoke any you suspect are leaked.

**List your keys.**

```bash
curl http://localhost:3000/v1/admin/auth/keys \
  -H 'Authorization: Bearer k_live_abc123...'
```

`200 OK`:

```json
{
  "keys": [
    { "id": "01HJ...", "name": "laptop", "current": true,  "expires_at": "...", "revoked_at": null },
    { "id": "01HK...", "name": null,     "current": false, "expires_at": "...", "revoked_at": null }
  ]
}
```

Fields worth looking at: `current` (true iff this is the key you just authenticated with), `last_used_at` (to spot dormant keys), `revoked_at` (non-null means already killed).

**Revoke a key.**

```bash
curl -X DELETE http://localhost:3000/v1/admin/auth/keys/01HK... \
  -H 'Authorization: Bearer k_live_abc123...'
```

`204 No Content`. The revoked key is rejected on its next use. Revoking the key you're *currently* using is allowed — you'll just need a fresh magic link to get back in.

**The 90-day rolling window in one paragraph.** Every successful authenticated request resets that key's `expires_at` to *now + 90 days*. A key untouched for 90 days expires silently and starts returning `401` on the next call. There is no refresh-token flow: the key *is* the credential, and activity is the refresh signal.

**When to revoke vs. let expire.** Revoke if you suspect a leak — the effect is immediate. Let it expire if the key just stopped being useful (old laptop, finished automation) — no action required.

See [openapi.yaml](openapi.yaml) — operations `listAdminApiKeys` and `revokeAdminApiKey`.

### 2.3 Create a server

A server is the top-level tenant: it owns admins, members, and the worlds you'll host. Creating one in a single call makes you its owner-admin.

```bash
curl -X POST http://localhost:3000/v1/admin/servers \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{
    "name": "Acme Co",
    "slug": "acme"
  }'
```

`201 Created`:

```json
{
  "id": "01HX...",
  "slug": "acme",
  "name": "Acme Co",
  "max_concurrent_worlds": 2,
  "max_worlds_per_account": 2,
  "owner_admin_id": "01HA..."
}
```

`slug` is optional — when omitted it's auto-derived from `name`. It must match `^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`. `max_concurrent_worlds` and `max_worlds_per_account` default to `2` each and can be tuned later.

**Update the limits.**

```bash
curl -X PATCH http://localhost:3000/v1/admin/servers/01HX... \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{ "max_concurrent_worlds": 4, "max_worlds_per_account": 2 }'
```

Whitelisted PATCH fields: `name`, `max_concurrent_worlds`, `max_worlds_per_account`. Anything else (slug, owner) is silently ignored. Per §16.7, limit changes apply **at join time only** — existing memberships are never retroactively pruned.

**List the servers you administer.**

```bash
curl http://localhost:3000/v1/admin/servers \
  -H 'Authorization: Bearer k_live_abc123...'
```

See [openapi.yaml](openapi.yaml) — `createServer`, `updateServer`, `listAdminServers`.

### 2.4 Configure access to the server

Admission is governed by `ServerAccess` rows of two kinds: `domain` glob match (e.g. `*@acme.test`) **or** `invite` (specific email). The two are unioned — a player gets in if either matches.

> **Today only the invite list is exposed via the API.** Domain glob rows live in the data model but aren't yet manageable through admin endpoints; if you need them, seed them directly. The rest of this section covers the invite list.

**Invite a player.**

```bash
curl -X POST http://localhost:3000/v1/admin/servers/01HX.../invitations \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{ "email": "guest@personal.com" }'
```

`201 Created`:

```json
{ "id": "01HV...", "email": "guest@personal.com", "created_at": "2026-05-17T10:00:00Z" }
```

Idempotent on `(server, email)` — re-inviting the same address returns the existing row.

**List or remove invitations.**

```bash
curl http://localhost:3000/v1/admin/servers/01HX.../invitations \
  -H 'Authorization: Bearer k_live_abc123...'

curl -X DELETE http://localhost:3000/v1/admin/servers/01HX.../invitations/01HV... \
  -H 'Authorization: Bearer k_live_abc123...'
```

**Admission is not retroactive (§16.7).** Removing an invitation does **not** remove any `ServerMembership` rows the invitation previously granted. To eject a member you must currently do it out-of-band. Similarly, tightening rules later won't kick existing members.

See [openapi.yaml](openapi.yaml) — `createServerInvitation`, `listServerInvitations`, `deleteServerInvitation`.

### 2.5 Invite co-admins

Once a server is yours, you can share operational duties with other admins.

**Invite a co-admin.**

```bash
curl -X POST http://localhost:3000/v1/admin/servers/01HX.../admins \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{ "email": "coadmin@example.com" }'
```

`201 Created`:

```json
{
  "adminship_id": "01HM...",
  "admin": { "id": "01HA2...", "email": "coadmin@example.com", "name": null },
  "role": "admin",
  "granted_by_admin_id": "01HA...",
  "joined_at": "2026-05-17T10:05:00Z"
}
```

The backend find-or-creates the `Admin` row — no password setup is needed. They sign in via [§2.1](#21-get-authorized) using the same magic-link flow you used. Idempotent on `(server, admin)`.

**List the current adminships.**

```bash
curl http://localhost:3000/v1/admin/servers/01HX.../admins \
  -H 'Authorization: Bearer k_live_abc123...'
```

Rows are ordered by `joined_at` and include the original owner (`role: "owner"`) alongside any co-admins (`role: "admin"`).

**Revoke a co-admin.** The path id is the **Admin id**, not the adminship id.

```bash
curl -X DELETE http://localhost:3000/v1/admin/servers/01HX.../admins/01HA2... \
  -H 'Authorization: Bearer k_live_abc123...'
```

`204 No Content`.

**The last-admin guard (§17.1).** A server must always have at least one admin. Removing the final admin returns:

```json
{ "error": { "code": "last_admin", "message": "Cannot remove the only remaining admin" } }
```

with `422`. Add a co-admin first if you want to step down.

See [openapi.yaml](openapi.yaml) — `inviteServerAdmin`, `listServerAdmins`, `revokeServerAdmin`.

### 2.6 Inspect server membership

Once players start joining, you can see who's on the server. Real names are visible to admins (§17.1).

```bash
curl http://localhost:3000/v1/admin/servers/01HX.../members \
  -H 'Authorization: Bearer k_live_abc123...'
```

`200 OK`:

```json
{
  "members": [
    {
      "membership_id": "01HN...",
      "player": { "id": "01HP...", "email": "alice@acme.test", "name": "Alice Liang" },
      "joined_at": "2026-05-17T11:00:00Z"
    }
  ]
}
```

Pair this with the invitations list ([§2.4](#24-configure-access-to-the-server)) when auditing: invitations show who you've *let in*, members show who actually *joined*.

See [openapi.yaml](openapi.yaml) — `listServerMembers`.

### 2.7 Propose a new world

A world is one round of play. You create it in `proposed` state with a future T0 (round start) and a `min_players` threshold; once both are met it auto-starts.

```bash
curl -X POST http://localhost:3000/v1/admin/servers/01HX.../worlds \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{
    "name": "Spring 2026",
    "slug": "spring-2026",
    "min_players": 4,
    "t0_at": "2026-05-24T18:00:00Z",
    "auto_cancel_after_hours": 168
  }'
```

`201 Created`:

```json
{
  "id": "01HW...",
  "server_id": "01HX...",
  "name": "Spring 2026",
  "slug": "spring-2026",
  "seed": "0x1f...",
  "status": "proposed",
  "min_players": 4,
  "auto_cancel_after_hours": 168,
  "t0_at": "2026-05-24T18:00:00Z",
  "grace_closes_at": null,
  "archived_at": null,
  "cancelled_at": null,
  "wonder_name": null
}
```

Key fields:

- `seed` is the deterministic RNG seed for map generation — same seed always produces the same map.
- `auto_cancel_after_hours` (default `168` = 7 days) caps how long a `proposed` world can sit empty before auto-cancelling.

**Concurrent world limit.** You can have at most `max_concurrent_worlds` worlds in non-terminal state per server. Exceed that and creation returns `422` with `code: "concurrent_world_limit_reached"`.

**List worlds on a server.**

```bash
curl http://localhost:3000/v1/admin/servers/01HX.../worlds \
  -H 'Authorization: Bearer k_live_abc123...'
```

Returns past and present worlds (any status).

See [openapi.yaml](openapi.yaml) — `proposeWorld`, `listAdminWorlds`.

### 2.8 Edit or cancel a proposed world

While the world is still `proposed` you can adjust `name`, `t0_at`, `min_players`, and `auto_cancel_after_hours`. Once it transitions past `proposed`, the configuration is frozen.

```bash
curl -X PATCH http://localhost:3000/v1/admin/worlds/01HW... \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{ "t0_at": "2026-05-25T18:00:00Z", "min_players": 6 }'
```

`200 OK` — returns the updated world. Changing `t0_at` reschedules the auto-start; changing `min_players` recomputes the gate at start time.

**Trying to PATCH a non-proposed world** returns `422` with `code: "world_not_configurable"`.

**Cancel a world that won't fill.**

```bash
curl -X POST http://localhost:3000/v1/admin/worlds/01HW.../cancel \
  -H 'Authorization: Bearer k_live_abc123...'
```

`200 OK` — world transitions to `cancelled` (terminal). Cancelling anything past `proposed` returns `422 world_not_cancellable`. An idle `proposed` world also self-cancels automatically `auto_cancel_after_hours` after creation.

See [openapi.yaml](openapi.yaml) — `configureWorld`, `cancelWorld`.

### 2.9 World-level invitations (optional)

`WorldInvitation` records are **informational only** — they do **not** gate join. Admission is still controlled by `ServerAccess` ([§2.4](#24-configure-access-to-the-server)). Use these for coordination and notifications when you want to flag a specific world to a player who already has server access.

```bash
curl -X POST http://localhost:3000/v1/admin/worlds/01HW.../invitations \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_abc123...' \
  -d '{ "email": "alice@acme.test" }'
```

`201 Created`:

```json
{
  "id": "01HI...",
  "email": "alice@acme.test",
  "invited_by_admin_id": "01HA...",
  "created_at": "2026-05-17T12:00:00Z"
}
```

Idempotent on duplicates. List with `GET` and remove with `DELETE` at the same path; both are no-ops on admission.

```bash
curl http://localhost:3000/v1/admin/worlds/01HW.../invitations \
  -H 'Authorization: Bearer k_live_abc123...'

curl -X DELETE http://localhost:3000/v1/admin/worlds/01HW.../invitations/01HI... \
  -H 'Authorization: Bearer k_live_abc123...'
```

See [openapi.yaml](openapi.yaml) — `createWorldInvitation`, `listWorldInvitations`, `deleteWorldInvitation`.

### 2.10 Watch a world go live

There's no `start` endpoint — worlds transition autonomously based on time and player count. The lifecycle:

1. **`proposed`** — created via [§2.7](#27-propose-a-new-world). Joinable.
2. **`grace`** — when `t0_at` is reached *and* `joined_count >= min_players`, the world auto-starts: map is generated from `seed`, spawn regions are assigned to anyone who joined during `proposed`, the 72-hour late-join window opens. `grace_closes_at` is now set.
3. **`active`** — when `grace_closes_at` passes. Late-join is closed; full game mechanics (combat, raids, Wonders) are live.
4. **`archived`** — terminal. Triggered when a Wonder survives Consecration. World is read-only; `archived_at` is set.
5. **`cancelled`** — terminal. Triggered by [§2.8](#28-edit-or-cancel-a-proposed-world) or by `auto_cancel_after_hours` elapsing while still empty.

If T0 fires but `joined_count < min_players`, the world stays `proposed` and re-checks each tick.

**Inspect current state.**

```bash
curl http://localhost:3000/v1/admin/worlds/01HW... \
  -H 'Authorization: Bearer k_live_abc123...'
```

The `status`, `t0_at`, `grace_closes_at`, `archived_at`, `cancelled_at`, and `wonder_name` fields together tell you where the round is in its arc.

See [openapi.yaml](openapi.yaml) — `showAdminWorld`.

### 2.11 Diagnostics while the world runs

The main admin observability endpoint is the per-world battle log. It surfaces every resolved fight on the server's worlds you administer.

```bash
curl 'http://localhost:3000/v1/admin/worlds/01HW.../battles?limit=25&offset=0' \
  -H 'Authorization: Bearer k_live_abc123...'
```

`200 OK`:

```json
{
  "battles": [ { "id": "01HB...", "ended_at": "...", "outcome": "...", "...": "..." } ],
  "total_count": 137
}
```

Ordered by `ended_at` desc, then `id` desc. Default `limit` is 25; max 100. Use `offset` to paginate.

**What admins can see.** Battle metadata across the world, server members and their profiles, every invitation and adminship, world configuration and seed.

**What admins cannot see.** Per-kingdom private state (resource stockpiles, build queues, march intents) is not exposed to admins — those endpoints are player-scope only. Admin observability is for hosting and moderation, not playing.

See [openapi.yaml](openapi.yaml) — `listWorldBattles`.

### 2.12 Decommissioning

When a server has outlived its purpose, you can delete it. This is destructive and cascades through every dependent row.

```bash
curl -X DELETE http://localhost:3000/v1/admin/servers/01HX... \
  -H 'Authorization: Bearer k_live_abc123...'
```

`204 No Content`. Cascades through adminships, memberships, accesses, player profiles, worlds, kingdoms, and everything those reference. Any admin on the server may delete it — no extra ownership check beyond adminship.

**When not to do this.**

- **A round is active.** Players will lose in-progress kingdoms with no archive. Wait for round end, or cancel the world first ([§2.8](#28-edit-or-cancel-a-proposed-world)).
- **You only want to slow new joins.** Reduce `max_concurrent_worlds` or stop creating worlds — deletion is a sledgehammer.
- **You only want to remove yourself.** Revoke your own adminship ([§2.5](#25-invite-co-admins)) instead, after adding a co-admin so the last-admin guard doesn't block you.

See [openapi.yaml](openapi.yaml) — `deleteServer`.

---

## 3. Player Walkthrough

The journey of someone who wants to play — from "I got an invite" to "the round is over and I'm on the leaderboard."

### 3.1 Get authorized

Every player endpoint needs a player-scope Bearer API key. The flow mirrors the admin flow ([§2.1](#21-get-authorized)) but lives on `/v1/auth/...` instead of `/v1/admin/auth/...`.

**Step 1 — Ask for the magic link.**

```bash
curl -X POST http://localhost:3000/v1/auth/magic_link \
  -H 'Content-Type: application/json' \
  -d '{"email": "alice@acme.test"}'
```

`202 Accepted`. The response is identical whether or not a `Player` already exists for that email — no enumeration leak.

**Step 2 — Exchange the token.**

```bash
curl -X POST http://localhost:3000/v1/auth/exchange \
  -H 'Content-Type: application/json' \
  -d '{"token": "PASTE_TOKEN_FROM_EMAIL"}'
```

`201 Created`:

```json
{
  "api_key": "k_live_player_xyz...",
  "expires_at": "2026-08-15T10:00:00Z",
  "owner": { "type": "Player", "...": "..." }
}
```

The exchange does more than mint a key. On first-time exchange the backend also:

1. Find-or-creates the `Player` row (default `name` is the email's local-part — fix it in [§3.2](#32-set-up-your-account)).
2. Iterates every `Server` and creates `ServerMembership` + `PlayerProfile` rows for any whose `ServerAccess` admits your email (§16.7 union rules).

So a single exchange can land you on multiple servers at once if you're admitted to them.

**Returning players** get a fresh key and no row churn — existing memberships and profiles are left alone.

See [openapi.yaml](openapi.yaml) — `requestPlayerMagicLink`, `exchangePlayerMagicLink`.

### 3.2 Set up your account

Identity on Dun is **per-server**, not global. The same human can have the handle `IronFist` on one server and `quiet_alice` on another. There is no global "player name" exposed beyond the email — only the per-server `handle` and `real_name`.

**Set or change your per-server profile.**

```bash
curl -X PATCH http://localhost:3000/v1/servers/01HX.../me \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "handle": "IronFist", "real_name": "Alice Liang" }'
```

`200 OK`:

```json
{
  "handle": "IronFist",
  "real_name": "Alice Liang",
  "stats": { "...": "..." },
  "title": null
}
```

`title` is auto-rendered from your stats (e.g. `[Champion of Aldermarch ×2]`) — it lights up once you've accumulated leaderboard credits ([§3.15](#315-read-the-archive-and-the-hall-of-fame)).

**Handle rules (§17.1).**

- Length 3–20.
- Must start with a letter.
- Allowed characters: `a-zA-Z0-9_` plus single internal spaces (no leading/trailing/consecutive).
- Case-preserved on display, case-insensitive for uniqueness within a server.
- Reserved (case-insensitive): `admin`, `system`, `dun`, `world`, `neutral`, `wilderness`, `server`, `anonymous`, `none`, `null`.

Either field can be omitted (the missing one is left alone). Phase 2 will activate a `handle_locked` guard during active rounds — once that lands, mid-round handle changes return `422 handle_locked`.

**Look up someone else's profile.**

```bash
curl http://localhost:3000/v1/servers/01HX.../players/IronFist \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Returns their `handle`, `real_name`, `stats`, `title`, and `joined_at`. Real names are visible only to server members; non-members get `403`.

See [openapi.yaml](openapi.yaml) — `updateOwnProfile`, `showPlayerProfile`.

### 3.3 Access a server

The token exchange in [§3.1](#31-get-authorized) already created memberships for every server you're admitted to. This section is about *finding* and *joining* the rest.

**List servers you can see.**

```bash
curl http://localhost:3000/v1/servers \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK`:

```json
{
  "servers": [
    { "id": "01HX...", "slug": "acme",    "name": "Acme Co",    "member": true  },
    { "id": "01HY...", "slug": "betagrp", "name": "Beta Group", "member": false }
  ]
}
```

The list is the union of: servers you're already a member of (`member: true`), and servers that would admit your email (`member: false`). Servers you can't see at all aren't in the response — existence isn't disclosed across boundaries.

**Join a server you're admitted to.**

```bash
curl -X POST http://localhost:3000/v1/servers/01HY.../join \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`201 Created`:

```json
{ "membership_id": "01HN...", "server": { "id": "01HY...", "...": "..." } }
```

Idempotent — calling `join` after you're already a member returns the existing membership. Trying to join a server that doesn't admit your email returns `403 forbidden`.

Right after joining, set your per-server handle and real name with [§3.2](#32-set-up-your-account).

See [openapi.yaml](openapi.yaml) — `listPlayerServers`, `joinServer`.

### 3.4 Browse available worlds

The player API today has **no "list worlds on this server" endpoint** — you look up worlds by ID. You'll get those IDs out-of-band: an admin shares them, a CLI client surfaces them, or you're notified via a `WorldInvitation` (informational only — see [§2.9](#29-world-level-invitations-optional)).

Once you have a world ID:

```bash
curl http://localhost:3000/v1/worlds/01HW... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK` — the world detail. Key fields to read before deciding to join:

- `status` — `proposed` (joinable, T0 not yet reached), `grace` (joinable, late-join window), `active` (closed), `archived` / `cancelled` (terminal).
- `t0_at` — when the round will start (if `proposed`).
- `grace_closes_at` — when late-join shuts off (if `grace`).
- `min_players` and current join count.
- Region count and any caller-side kingdom summary if you've already joined.

Non-members of the server get `404`, not `403` — world existence isn't disclosed across server boundaries.

See [openapi.yaml](openapi.yaml) — `showWorld`.

### 3.5 Join a world

Joining a world creates your kingdom in it. The same call works during `proposed` and `grace`; behavior differs slightly between the two.

```bash
curl -X POST http://localhost:3000/v1/worlds/01HW.../join \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`201 Created` — returns your `Kingdom` row.

**What you start with** (§16.8):

- All resource buildings (Town Hall, Gold Mint, Lumber Camp, Quarry, Iron Mine, Warehouse) at level 1.
- Barracks, Walls, Watchtower at level 1.
- 500 of each resource (`gold`, `wood`, `stone`, `iron`).
- 20 Levy in your home Garrison army.

**Proposed vs grace differs in spawn assignment.**

- During `proposed`, a *stub* kingdom is created — no home region yet. Home regions are assigned by `Worlds::Start` when the world transitions to `grace`.
- During `grace`, you get an immediate spawn region **plus** the late-joiner stockpile bonus: `floor(hours_since_T0 / 12) × 1000` per resource, capped at +4000.

**Errors you might hit:**

- `403 forbidden` — you're not admitted to the world's server.
- `422` — world status isn't `proposed` or `grace`, or you've hit `max_worlds_per_account` on this server.

See [openapi.yaml](openapi.yaml) — `joinWorld`.

### 3.6 Your first steps — Economy

The economy loop is: read your dashboard, queue building upgrades, wait, repeat. Resources accrue continuously against your Warehouse cap.

**Read your kingdom dashboard.**

```bash
curl http://localhost:3000/v1/kingdoms/01HK... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK` — materialized stockpiles, production rates, all buildings with their current level, and any in-progress build orders. The server lazily accrues stockpiles from the last checkpoint against current production and the Warehouse cap *before* serializing, so the numbers are current. Ripe build orders are resolved on the same call.

**Queue a building upgrade.**

```bash
curl -X POST http://localhost:3000/v1/kingdoms/01HK.../build \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "building": "warehouse", "target_level": 2 }'
```

`201 Created` — returns the `BuildOrder` with `completes_at`. Cost is deducted immediately. Time is `min(base × 1.55^(L-1), 24h) × stone_mason_discount`. `target_level` must equal *current level + 1* — a defensive concurrency check.

**Building catalog:**
`town_hall`, `gold_mint`, `lumber_camp`, `quarry`, `iron_mine`, `warehouse`, `barracks`, `stable`, `siege_workshop`, `walls`, `watchtower`, `stone_mason`.

**Single build slot.** Each kingdom has one active build order at a time. Queueing a second returns `422 queue_full`. Repeated identical orders are idempotent — same `building` + `target_level` returns the existing order without re-deducting.

**Cancel and refund.**

```bash
curl -X DELETE http://localhost:3000/v1/kingdoms/01HK.../build/01HB... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK`. Refunds 75% of resources (floored). Elapsed time is lost; the build slot frees immediately.

**Why upgrade the Warehouse early.** Stockpile cap scales quadratically with Warehouse level (roughly 1M per resource at L20). Hitting the cap means production silently caps out — visible by comparing `production_rate` to actual stockpile growth in successive dashboard reads.

See [openapi.yaml](openapi.yaml) — `showKingdom`, `queueBuildOrder`, `cancelBuildOrder`.

### 3.7 Your first steps — Military

Each military building has its own independent FIFO training queue (§11): Barracks, Stable, Siege Workshop. They run in parallel — three orders, one per building, all training at once.

**Queue a training order.**

```bash
curl -X POST http://localhost:3000/v1/kingdoms/01HK.../train \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "building": "barracks", "unit": "pikeman", "count": 10 }'
```

`201 Created` — returns the `TrainingOrder`. Cost is per-unit × count, deducted immediately. Time per unit is `base × 0.95^(building_level - 1)`; total = per-unit × count.

**Unit catalog** (§16.3): `levy`, `archer`, `pikeman`, `knight`, `catapult`, `royal_guard`, `scout`, `trebuchet`. Each unit is trained at a specific building — sending `unit: knight` to `building: barracks` returns `422 unit_not_trainable_here`.

**Rock-paper-scissors at a glance:**

- **Knights** beat **Archers** (1.4–1.6×).
- **Archers** beat **Pikemen** (1.4–1.6×).
- **Pikemen** beat **Knights** (1.4–1.6×).
- **Catapults** are the only unit that can capture wilderness nodes ([§3.10](#310-capture-nodes-and-claim-ruins)).
- **Trebuchets** are the only unit that deals significant damage to Wonders ([§3.13](#313-build-a-wonder)).
- **Scouts** ignore terrain modifiers (with Knights), useful for reconnaissance.
- **Royal Guard** is elite defense.

**Cancel and refund.**

```bash
curl -X DELETE http://localhost:3000/v1/kingdoms/01HK.../train/01HT... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK`. Refunds 75% of total cost (per-unit × count, floored).

**See your armies.**

```bash
curl http://localhost:3000/v1/kingdoms/01HK.../armies \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Newly-trained units land in your home Garrison automatically. Split them out into named armies in [§3.9](#39-march-scout-reinforce).

See [openapi.yaml](openapi.yaml) — `queueTrainingOrder`, `cancelTrainingOrder`, `listKingdomArmies`.

### 3.8 Read the map

The mental model: the world is a graph of **regions**. Each region has terrain, may host **nodes** (capturable production), **ruins** (one-time caches), **armies**, and **Wonders**. You navigate by region adjacency.

**The whole map at once.**

```bash
curl http://localhost:3000/v1/worlds/01HW.../map \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK`:

```json
{ "regions": [ { "id": "01HR...", "name": "Ashenfield", "terrain": "plains", "...": "..." } ] }
```

Returns a `RegionSummary` for every region: id, name, terrain, node count, current governance.

**Drill into one region.**

```bash
curl http://localhost:3000/v1/worlds/01HW.../regions/01HR... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Full region detail — governance, units present, Wonders sited here, every node with its production rate.

**Walk the adjacency graph.**

```bash
curl http://localhost:3000/v1/worlds/01HW.../regions/01HR.../adjacent \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Returns the directly-connected regions with their terrain. Marches plan paths along this graph ([§3.9](#39-march-scout-reinforce)).

**List every node.**

```bash
curl http://localhost:3000/v1/worlds/01HW.../nodes \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Includes three flavors:

- **Wilderness nodes** — `owner_kingdom_id: null`, static NPC `garrison` populated. Capturable.
- **Captured nodes** — `owner_kingdom_id` set, empty `garrison`. Producing for their owner.
- **Home-hoard nodes** — `is_home_hoard: true`. Permanently bound to a kingdom's home region.

`GET /v1/worlds/{world_id}/nodes/{id}` returns a single node.

**List ruins** — one-time resource caches scattered on the map.

```bash
curl http://localhost:3000/v1/worlds/01HW.../ruins \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

**Terrain economics:** Stone is overrepresented on the map (~35% of node yield). The other three resources split the remainder roughly evenly. Plan early Wonder ambitions around this.

See [openapi.yaml](openapi.yaml) — `showWorldMap`, `showRegion`, `showRegionAdjacent`, `listNodes`, `showNode`, `listRuins`.

### 3.9 March, scout, reinforce

Armies live at a region. They can be split, merged, renamed, and dispatched on marches. There are six march intents — pick the one that matches what you're trying to do (attack vs scout vs capture etc.).

**Inspect an army.**

```bash
curl http://localhost:3000/v1/armies/01HA... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Returns units by kind, current region, status (`home` or `marching`), carrying capacity, active march order if any.

**Split a home army.**

```bash
curl -X POST http://localhost:3000/v1/armies/01HG.../split \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "units": { "pikeman": 5, "archer": 3 }, "name": "Frontier 1st" }'
```

`201 Created` — returns `{ source, new }`. The source army shrinks; if empty after the split it's destroyed (unless it's the kingdom's auto-managed Garrison, which always sticks around). Source must be `status: home`.

**Rename and merge.**

```bash
curl -X POST http://localhost:3000/v1/armies/01HN.../rename \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "name": "Vanguard" }'

curl -X POST http://localhost:3000/v1/armies/01HN.../merge \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "from_id": "01HG..." }'
```

Merge requires both armies to belong to the same kingdom, share a region, and be `status: home`.

**Dispatch a march.**

```bash
curl -X POST http://localhost:3000/v1/armies/01HN.../march \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "target_region_id": "01HR2...", "intent": "reinforce" }'
```

`201 Created`. The backend plans the shortest path on the adjacency graph and computes per-leg time using the slowest unit's speed and the average terrain modifier of both endpoints (§16.10). Knight/Scout-only armies ignore terrain. A `march_arrival` event is scheduled at `arrives_at`.

**The six intents:**

- `attack` — fight whoever's there ([§3.11](#311-attack-and-raid)).
- `reinforce` — join your own forces at the target (or a friendly's, if reinforcing a same-kingdom army).
- `scout` — gather intel without engaging.
- `capture` — take a wilderness node ([§3.10](#310-capture-nodes-and-claim-ruins)).
- `claim_ruin` — pick up a ruin's resource cache ([§3.10](#310-capture-nodes-and-claim-ruins)).
- `caravan` — deliver trade payload ([§3.12](#312-trade-with-caravans)). Dispatched via the caravan endpoint, not directly.

**Recall an in-flight march.**

```bash
curl -X POST http://localhost:3000/v1/armies/01HN.../recall \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK`. Cancels the pending `march_arrival`, schedules a return march (intent `reinforce`) with the path reversed. v1 charges no unit losses on recall.

See [openapi.yaml](openapi.yaml) — `showArmy`, `splitArmy`, `renameArmy`, `mergeArmy`, `dispatchMarch`, `recallMarch`.

### 3.10 Capture nodes and claim ruins

Capturing nodes is how you scale production beyond your home buildings. Wilderness nodes have static NPC garrisons; defeat them and you own the node.

**Capture a wilderness node.**

```bash
curl -X POST http://localhost:3000/v1/armies/01HN.../march \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "target_region_id": "01HR3...", "intent": "capture" }'
```

The march resolves combat against the wilderness garrison on arrival. **Catapults are required for capture** — an army with no Catapults will defeat the garrison but cannot flip ownership. Catapults are trained at Siege Workshop ([§3.7](#37-your-first-steps--military)).

Once captured, the node's `owner_kingdom_id` is set to yours, its garrison empties, and it starts contributing to your stockpile at its `base_rate`.

**Claim a ruin.**

```bash
curl -X POST http://localhost:3000/v1/armies/01HN.../march \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "target_region_id": "01HR4...", "intent": "claim_ruin" }'
```

Ruins are one-time caches — the resources transfer to your stockpile (capped by your Warehouse), the ruin is consumed. No garrison fight; first-come-first-served.

Track wilderness and ruin opportunities via the map endpoints from [§3.8](#38-read-the-map).

### 3.11 Attack and raid

Attacking another kingdom is a march with `intent: attack` against a region they hold (their home, a captured node's region, a wonder site).

```bash
curl -X POST http://localhost:3000/v1/armies/01HN.../march \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "target_region_id": "01HR5...", "intent": "attack" }'
```

On arrival, combat resolves automatically.

**Combat at a glance (§16.4).**

- 6 rounds of simultaneous resolution.
- Per-round damage uses unit RPS multipliers with ±8% variance.
- Defender gets +20% home bonus (capped at +25% with terrain).
- Loot: up to 25% of defender's resource stockpile per raid, capped by attacker carrying capacity.

**Read battle reports.**

```bash
curl 'http://localhost:3000/v1/kingdoms/01HK.../battles?limit=25&offset=0' \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Lists battles where this kingdom was attacker or defender, ordered by `ended_at` desc.

```bash
curl http://localhost:3000/v1/battles/01HB... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK`:

```json
{
  "battle": { "id": "01HB...", "outcome": "attacker_won", "loot": { "...": "..." }, "...": "..." },
  "participants": [ { "side": "attacker", "kingdom_id": "...", "...": "..." } ]
}
```

Includes the round-by-round log. Only the attacker and defender kingdoms' owners can read a given battle — other players get `404`.

**The raid cap.** A given attacker–defender pair can resolve at most a configurable number of `attack` battles per 24h window (default 3 per server; admin-overridable). Excess marches still arrive but generate no loot. Wonder assaults are exempt.

See [openapi.yaml](openapi.yaml) — `dispatchMarch` (intent `attack`), `listKingdomBattles`, `showBattle`.

### 3.12 Trade with caravans

A caravan is a march with `intent: caravan` that moves resources from your kingdom to another's. The escort split off your army carries it; if a hostile third party is camped at the destination, they intercept it.

**Dispatch a caravan.**

```bash
curl -X POST http://localhost:3000/v1/kingdoms/01HK.../caravans \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{
    "receiver_handle": "BobTheBuilder",
    "source_army_id": "01HG...",
    "payload":       { "wood": 5000, "stone": 5000 },
    "escort_units":  { "levy": 20, "knight": 5 }
  }'
```

`201 Created` — returns the `Caravan` row with the underlying march details.

**What happens under the hood:**

1. The `escort_units` are peeled off `source_army_id` (must be `status: home` at your home region).
2. The `payload` is deducted from your stockpile immediately.
3. A march is dispatched with `intent: caravan` to the receiver's home region.
4. On arrival, the caravan either:
   - **Delivers** — resources land in receiver's stockpile (capped by their Warehouse).
   - **Intercepted** — if a hostile third-party army is camped at the destination region, the strongest hostile fights the escort. Outcome depends on combat; surviving escorts can still deliver.

**Constraints worth knowing:**

- Escort total carrying capacity must be ≥ sum of `payload`. Otherwise `422 insufficient_capacity`.
- Sender and receiver must be in the same world (`422 cross_world` if not).
- Self-trade is rejected (`422 self_trade`).

**Read the public trade ledger.** Every dispatch, delivery, and interception is recorded and visible to every server member.

```bash
curl 'http://localhost:3000/v1/worlds/01HW.../trade-ledger?player=IronFist&since=24h&limit=25&page=1' \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Filters: `player` (matches sender, receiver, or attacker handle), `since` (e.g. `24h`, `7d`), `limit` (max 100), `page` (1-based). Handles are snapshotted at dispatch — they stay stable even if the player later renames.

See [openapi.yaml](openapi.yaml) — `dispatchCaravan`, `listTradeLedger`.

### 3.13 Build a Wonder

The Wonder is the endgame. Building one starts a 90-hour Construction phase followed by a 24-hour Consecration phase — surviving Consecration ends the round.

**Prerequisites (§14, §16.2).**

- ≥3 owned nodes.
- Specific building level gates per Wonder kind.
- No live Wonder already on your kingdom.
- Stockpile ≥ foundation cost (25% of total: 200k Gold, 150k Wood, 600k Stone, 200k Iron).

**Check current state.**

```bash
curl http://localhost:3000/v1/kingdoms/01HK.../wonder \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Returns the kingdom's Wonder (or `{ "wonder": null }`). The server runs `Wonders::ApplyConstruction` before serializing, so HP and milestone state are always current.

**Start construction.**

```bash
curl -X POST http://localhost:3000/v1/kingdoms/01HK.../wonder \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "name": "sky_tower" }'
```

`201 Created`. Wonder names: `sky_tower`, `eternal_citadel`, `cathedral_of_ages`, `library_of_worlds`, `crown_of_kings`, `black_spire`.

Effects: 25% foundation cost deducted, build queue locked (no new building upgrades until the Wonder resolves), Construction phase scheduled for 90h.

**The phases.**

1. **Foundation** — instant, 25% paid up-front.
2. **Construction** — 90h. HP accrues at 100 HP/hour up to 10 000 HP. Construction auto-pauses at HP thresholds 2500/5000/7500 — you must pay the corresponding milestone (10% of total cost each) to resume.
3. **Consecration** — 24h at max vulnerability. Survive with HP > 0 and the round ends in your favor ([§3.14](#314-end-of-round)).

**Pay a milestone.**

```bash
curl -X POST http://localhost:3000/v1/kingdoms/01HK.../wonder/milestone \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "percent": 25 }'
```

Pay 25/50/75 in order. Wrong order or amount returns `422 wrong_milestone_percent`.

**Repair damage.**

```bash
curl -X POST http://localhost:3000/v1/kingdoms/01HK.../wonder/repair \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer k_live_player_xyz...' \
  -d '{ "hp": 1000 }'
```

Costs 8 Stone per HP. Phase cap is 2000 HP/phase (foundation/construction/consecration are independent budgets). Each 500 HP repaired pauses construction by 30 minutes (stacking) — over-repair burns your timeline.

**Abandon a Wonder.**

```bash
curl -X DELETE http://localhost:3000/v1/kingdoms/01HK.../wonder \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Marks it `destroyed`, unlocks your build queue. All paid resources are lost.

**Trebuchets are how Wonders die.** Only Trebuchets deal meaningful HP damage to a Wonder — other units chip ineffectively. When a rival starts Consecration, you need Trebuchets at their region or your Wonder loses the race.

**Watch the field.**

```bash
curl http://localhost:3000/v1/worlds/01HW.../wonders \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Public list of every Wonder in the world, ordered by creation time.

See [openapi.yaml](openapi.yaml) — `getWonder`, `startWonder`, `repairWonder`, `payWonderMilestone`, `cancelWonder`, `listWorldWonders`.

### 3.14 End of round

A round ends the moment any Wonder survives the full 24h Consecration phase with HP > 0. There's no manual trigger — the consecration handler atomically:

1. Transitions the world `active → archived` and stamps `archived_at`.
2. Builds a `RoundArchive` snapshot of the final state.
3. Recomputes per-server `PlayerStats` (rounds played, won, Wonders built/destroyed, peak nodes, raid count, resources looted).
4. Rebuilds the four `Leaderboard` snapshots (Champions, Wreckers, Warlords, Veterans) and reassigns Champion titles.

From the player's side, the world simply becomes read-only. All write endpoints on an archived world return `422 world_not_active` or similar.

**If no Wonder consecrates** — the round can in principle run forever. There's no time cap on a `proposed → grace → active` lifecycle; it ends when someone wins. Admins can intervene by abandoning their own world (no admin "force-end" endpoint exists in v1).

### 3.15 Read the archive and the Hall of Fame

Once a world is archived, the snapshot and updated leaderboards are available.

**Read the archive.**

```bash
curl http://localhost:3000/v1/worlds/01HW.../archive \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`200 OK` — the immutable `RoundArchive`: final region governance, kingdoms with final resource totals, Wonder state, aggregate counts. Returns `404` until the world is archived. Caller must be a member of the hosting server.

**Read the Hall of Fame.**

```bash
curl http://localhost:3000/v1/servers/01HX.../hall-of-fame \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Or filter to one kind:

```bash
curl 'http://localhost:3000/v1/servers/01HX.../hall-of-fame?kind=champions' \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

The four leaderboards (§17.4):

- **Champions** — most Wonders completed.
- **Wreckers** — most Wonders destroyed.
- **Warlords** — highest raid count.
- **Veterans** — most rounds played.

Snapshots only refresh at round end, so reading mid-round returns the previous round's standings. Caller must be a server member; non-members get `403`.

**Persistent identity per server (§17.4).** Your handle, real name, stats counters, and Champion titles are stored against your `PlayerProfile`, which is scoped per server. They survive every round on that server until you delete your account ([§3.16](#316-account-hygiene)).

See [openapi.yaml](openapi.yaml) — `getWorldArchive`, `getHallOfFame`.

### 3.16 Account hygiene

The same patterns as the admin side ([§2.2](#22-manage-your-api-keys)), plus account deletion.

**Rotate keys.**

```bash
curl http://localhost:3000/v1/auth/keys \
  -H 'Authorization: Bearer k_live_player_xyz...'

curl -X DELETE http://localhost:3000/v1/auth/keys/01HK... \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

Same shape as the admin endpoints, same 90-day rolling expiry, same single-key-is-the-credential model.

**Delete your account.**

```bash
curl -X DELETE http://localhost:3000/v1/auth/account \
  -H 'Authorization: Bearer k_live_player_xyz...'
```

`204 No Content`. The calling key is now invalid.

**What gets anonymized.**

- Your **real name** is purged immediately on every server.
- Per-server **handles** are anonymized to `[deleted player #<id>]`. Original handles are reserved for 30 days, then released for reuse.
- **Stats counters** are zeroed across all servers.
- **Champion titles** are removed from leaderboard snapshots.
- All your **ApiKeys** are revoked.
- The `Player` row is **tombstoned** (email replaced, name `[deleted]`) — the row stays so archive references remain intact.

**What stays.** Battle reports and trade ledger entries keep the **handle-at-time-of-event snapshot**. A battle from three months ago still reads `IronFist defeated alice@acme.test` rather than retroactively redacting. This is intentional — the historical record stays coherent for everyone else.

**Irreversible.** There's no undo. Once the call returns `204`, your participation history is anonymized everywhere and the calling key won't work.

See [openapi.yaml](openapi.yaml) — `listPlayerApiKeys`, `revokePlayerApiKey`, `deleteAccount`.

---

## 4. Appendix

### 4.1 Error codes you'll actually hit
The most common 4xx codes from the envelope, what they mean, and the typical fix.

### 4.2 Rate limits
Per-minute and per-hour write limits, the `429` response, `retry_after`, admin overrides.

### 4.3 Time and ticks
How discrete events, production checkpoints, and march arrivals interact with wall-clock time.

### 4.4 Further reading
Pointers back to [openapi.yaml](openapi.yaml), the [Game Design Document](dun%20Game%20Design%20Document.v3.md), and the architecture phase chapters.
