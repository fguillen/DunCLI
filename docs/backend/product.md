# dun — Product Overview

`dun` (from Gaelic for an ancient or medieval fort) is an async multiplayer, console-native, medieval-fantasy strategy game. It is designed to fit a developer's workday — short check-ins of 30 seconds to 5 minutes, 1 to 10 times a day — and to function as a continuous, low-effort team-building mechanism for coworkers in the same company.

This document is a high-level product summary. The full design lives in [docs/dun Game Design Document.v3.md](docs/dun%20Game%20Design%20Document.v3.md); section references like `§17.1` point at that doc. The implementation roadmap is in [TODO.md](TODO.md). The HTTP API surface is in [docs/openapi.yaml](docs/openapi.yaml).

---

## Why dun exists

**Problem.** Developers experience frequent micro-idle moments at work (builds, CI, deploys, LLM responses, package installs). Filling those gaps with social media or news is unsatisfying and hostile to flow. Separately, remote and hybrid work have eroded informal team bonding — Slack helps but is shallow, and scheduled team-building events feel forced.

**Opportunity.** Build a game that fits developer micro-idle time **and** functions as a passive team-building tool for coworkers.

**Success criteria.** Developers play voluntarily during idle moments instead of switching to social media; coworkers reference in-game events in real-life conversation (Slack, coffee, standup); companies see it as an organic alternative to scheduled team-building.

## Target user

Software developers and adjacent technical roles (SRE, data engineers, DevOps) who spend their day in a terminal or IDE, have multiple small idle windows per day, work in companies with at least a handful of technical colleagues, and appreciate text-based, command-driven interfaces.

## Core design principles

- **Short session friendly** — any meaningful action takes seconds, not minutes.
- **Persistent world** — progress continues while offline; check-ins feel rewarding.
- **Coworker-bound worlds** — each server maps to a company or office. The team-building engine.
- **Console-native** — the game lives where developers already are.
- **Asynchronous social loop** — actions take real time to complete, creating natural conversation hooks.

---

## Game shape

| Aspect | Value |
| :---- | :---- |
| Session length | 30 seconds to 5 minutes (sweet spot 1–3 min) |
| Sessions per day | 1 to 10 |
| Round length (typical) | 2 to 4 weeks |
| Round end trigger | A Wonder completes its 24-hour Consecration phase with HP > 0 |
| Attack travel time | Hours to days |
| Absence tolerance | Moderate (a weekend off is survivable; a week off hurts) |
| Newbie protection | None; late joiners get a stockpile bonus (§16.8) |
| Win condition | Wonder victory only — no score fallback, no time cap |

## Core gameplay loop

Conquest / raiding spine (Ogame-inspired) with a light diplomacy layer:

1. Build your home kingdom (12 buildings, single upgrade slot, linear levels 1–20 with exponential cost).
2. Train armies (8 unit types across three tiers, with a rock-paper-scissors layer).
3. Capture resource nodes on a shared region map; raid coworkers; trade via interceptable caravans.
4. Eventually start a Wonder in your home kingdom. Pay 25% up-front, build to 10,000 HP over ~90h, survive 24h of Consecration. If destroyed at any point, you lose all paid resources and may restart.

## Theme

Medieval fantasy kingdoms. Chosen for broad appeal, massive creative latitude, and easy "your kingdom vs your coworker's kingdom" characterization — and to be distinct from Ogame's space theme.

---

## Key systems

### Economy

- Four resources: **Gold** (currency), **Wood** (renewable), **Stone** (defensive / Wonder-critical), **Iron** (military).
- Hybrid production: four resource buildings at home plus capturable map nodes (1.2 nodes per player; Stone overweighted to 35% to feed the Wonder).
- Stockpile cap from Warehouse, quadratic scaling (~1M per resource at L20).
- Loot up to 25% of defender stockpile per raid, capped by attacker carrying capacity.

### Map

- Procedural per round, seeded planar graph. Region count = `clamp(2.5 × players + 6, 16, 64)`.
- Five terrain types (Plains, Forest, Hills, Mountain, Marsh) with march and combat modifiers (§16.10).
- Three node tiers (Rich +500/h, Standard +250/h, Poor +120/h) with static one-time garrisons.
- Static **Ruins** (one-time resource caches) and recurring **Weather windows** (24h terrain modifiers, 12h telegraph) layer onto the map (§16.11).
- Spawns placed by Poisson-disk: ≥2 hops between kingdoms, Plains or Hills only, ≥2 adjacent wilderness regions, away from Rich nodes (§16.5, §16.8).

### Military

- Eight units: Levy, Archer, Pikeman, Knight, Catapult, Royal Guard, Scout, Trebuchet (§16.3).
- RPS: Knights > Archers, Pikemen > Knights, Archers > Pikemen (1.4–1.6× multipliers).
- Specialist roles: Catapults capture nodes and damage walls; Trebuchets damage Wonders (50 HP per surviving unit); Royal Guard is the no-RPS anchor.
- 6-round combat simulation with ±8% variance; defender bonus +20% home, terrain capped at +25%.

### Buildings

12 buildings: Town Hall, Gold Mint, Lumber Camp, Quarry, Iron Mine, Warehouse, Barracks, Stable, Siege Workshop, Walls, Watchtower, Stone Mason. Linear levels 1–20, cost ×1.75 per level, time ×1.55 per level capped at 24h. Single upgrade slot (Town Hall unlocks +1 at L10 and L20). 75% cancel refund on resources.

### Wonder

- Total cost: 800k Gold, 600k Wood, **2.4M Stone**, 800k Iron. Stone-heavy to make Quarry nodes matter all round.
- Three phases: **Foundation** (instant, 25% payment) → **Construction** (90h at 100 HP/h to 10,000 HP, milestone payments at 25/50/75%) → **Consecration** (24h at maximum vulnerability).
- Repair: 1 HP per 8 Stone, cap 2,000 HP per phase. Build queue locked during construction; unit training continues.
- Destruction loses all paid resources; builder may restart.

### Diplomacy and social layer

Trade only. Resources sent via caravans, interceptable en route, recorded in a public world-scoped ledger. No formal pacts, alliances, or in-game intel sharing — all coordination happens out of game (Slack, voice, in person). The game provides the *state* worth talking about; the office provides the social layer.

### Onboarding

Company-scheduled rounds with an on-demand organizer trigger and a minimum player count. 72-hour grace window after T0 for joining, then closed. Late joiners get a stockpile bonus of `floor(hours_since_T0 / 12) × 1000` per resource, capped at +4000 (§16.8). Pre-built starter kingdom: each resource building at L1, Barracks L1, Walls L1, Watchtower L1, 500 of each resource, 20 Levy.

### Round end

Instant freeze on Wonder Consecration survival — all armies halt, all queues freeze, world enters read-only archive. No mandatory cooldown. The next round is manually proposed; no auto-rollover. Gameplay state resets to zero; **persistent profile** carries forward per-server (rounds played/won, Wonders completed/destroyed, peak nodes, raid counts, resources looted). Round winners receive a permanent inline title `[Champion of <World> ×N]` displayed beside their handle in every gameplay surface (§17.4).

---

## Identity and multi-tenancy

- **Email** is the identity unit. Verified via 15-minute single-use magic link. Subsequent CLI sessions authenticate via a 90-day rolling Bearer `ApiKey`. Two human kinds share the same magic-link + ApiKey substrate:
  - **Player** keys at `/v1/auth/...` for game actions.
  - **Admin** keys at `/v1/admin/auth/...` for server configuration, world creation, co-admin management, invitations.
- **Server-scoped identity** (§16.7). Each server is owned and configured by a company. Profiles, hall-of-fame, world memberships are fully scoped to a server; same email on two servers has two independent identities.
- **Access** is union of domain whitelist + explicit invite list (`ServerAccess` rows). Limit changes are not retroactive — existing memberships are never pruned when settings tighten.
- **Display name** is split: a per-server **handle** drives every gameplay surface (battle reports, leaderboards, announcements); a **real name** is visible only via `player show` to other members of the same server.

## Anti-abuse posture

- **Public trade ledger** turns collusion into a visible social act (cap-free; the office handles it).
- **Reports** are non-anonymous, scoped to server admins.
- **Rate limits**: 60 writes/minute, 1000 writes/hour per account; reads unlimited; admin-overridable per server.
- **Repeat-raid cap**: default 3 raids per attacker-target pair per 24h, configurable per server. Wonder assaults exempt.
- **Multi-account audit view** surfaces IP/device clusters for admin review — no auto-action.

---

## Technical shape (high level)

- **Backend**: Rails 8 API-only monolith. PostgreSQL 18+. Solid Queue / Solid Cache / Solid Cable (no Redis dependency in v1).
- **Tick model**: 5-second discrete event tick (build/training/march/battle/Wonder/caravan/weather edges), 1-minute production checkpoint, 5-minute stats refresh, 1-hour housekeeping.
- **Protocol**: HTTP/JSON only, no push or streaming in v1. Versioned under `/v1/...`. Specced in [docs/openapi.yaml](docs/openapi.yaml).
- **Client**: RubyGem distribution, single `dun` binary, Ruby 3.3+. Stateful REPL by default; one-shot mode for scripting; `--json` for tooling.
- **Hosting**: self-hosted single-tenant per company. Reference deployment is a Hetzner CX22/CPX21 running docker-compose (dun-web, dun-worker, postgres, Caddy for TLS). Nightly `pg_dump` backups.
- **Observability**: OpenTelemetry instrumentation throughout. Opt-in bundled Grafana stack (Tempo, Loki, VictoriaMetrics) via a docker-compose profile.

Full architecture rationale in §17.5.

---

## Status and roadmap

The implementation roadmap lives in [TODO.md](TODO.md), broken into phases. Each phase ends with tests and a git commit.

| Phase | Scope | Status |
| :---- | :---- | :---- |
| 0 | Bootstrap & conventions (Rails 8, Solid Queue/Cache, lograge, OTel, pagy, data_migrate) | Complete |
| 1 | Identity, Auth & Server Membership (§17.1, §16.7) | Complete |
| 2 | World Lifecycle & Map Generation (§13, §16.5, §16.8, §16.10, §16.11 placement) | Next |
| 3 | Resources, Buildings & Build Queue (§6, §7, §10, §16.4) | Planned |
| 4 | Tick Engine & Time Model (§17.5 cadences) | Planned |
| 5 | Military: Units, Training, March (§9, §16.3, §16.10) | Planned |
| 6 | Combat Resolution & Battle Reports (§9, §16.3) | Planned |
| 7 | Nodes, Capture, Ruins (§7, §16.5, §16.11) | Planned |
| 8 | Trade, Caravans & Ledger (§12, §17.2) | Planned |
| 9 | Wonder Mechanics (§14, §16.2) | Planned |
| 10 | Round End, Archive & Persistent Profiles (§16.6, §17.4) | Planned |
| 11 | Anti-Abuse: Reports, Rate Limits, Raid Cap (§17.2) | Planned |
| 12 | Weather Windows (§16.11) | Planned |
| 13 | Fog of War & Scouting (§16.9, v1.1) | Deferred |
| 14 | Observability, Deployment & Ops (§17.5) | Planned |

## Out of v1 scope

Recorded so the design surface doesn't silently absorb them:

- Slack / email digest / calendar / webhook / push integrations (§17.3).
- SSO beyond magic link (§17.1, v1.1 candidate).
- Marketplace order-book trading (§12).
- Specialized units, Heroes, Quests, additional cosmetics (§19).
- Managed hosting / SaaS surface (§17.5, §18.1).
- Multi-tenant infrastructure — single-tenant per server is the v1 commitment.
