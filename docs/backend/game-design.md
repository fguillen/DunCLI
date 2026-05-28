# dun — Game Design Document

**Status**: Living design document. Captures all committed decisions, the reasoning behind them, alternatives considered, and pending items. Single-source context for continuing the design conversation.

**About the game**: `dun` is a console-based, async multiplayer medieval fantasy strategy game. From Dun (Gaelic), an ancient or medieval fort. Designed for developers during workday micro-idle moments. Functions as a passive team-building tool for coworkers.

**How to use this document**: The document is organized into four parts.

- **Part I (Sections 1–15)** is the foundational design. Every section is a committed decision with reasoning and alternatives.  
- **Part II (Section 16\)** is the extended mechanics layer. Each subsection started as a pending decision and has since been locked.  
- **Part III (Section 17\)** is the platform layer: identity, anti-abuse, integrations, persistence, infrastructure.  
- **Part IV (Sections 18–20)** holds business model and rollout (still pending), future enhancements (deferred), and a quick-reference summary.

When a subsection is still pending, it carries a **(Pending)** tag and lists the open questions instead of committed decisions.

---

## Table of Contents

### Part I — Foundations

1. [Project Motivation and Objectives](#1-project-motivation-and-objectives)  
2. [Theme](#2-theme)  
3. [Core Gameplay Loop](#3-core-gameplay-loop)  
4. [Session Shape and Round Structure](#4-session-shape-and-round-structure)  
5. [Win Condition](#5-win-condition)  
6. [Resources](#6-resources)  
7. [Resource Generation](#7-resource-generation)  
8. [Map / World Structure](#8-map--world-structure)  
9. [Military System](#9-military-system)  
10. [Buildings and Progression](#10-buildings-and-progression)  
11. [Timing Model](#11-timing-model)  
12. [Diplomacy / Social Layer](#12-diplomacy--social-layer)  
13. [Player Onboarding](#13-player-onboarding)  
14. [Wonder Mechanics](#14-wonder-mechanics)  
15. [Console Interaction Sketches](#15-console-interaction-sketches)

### Part II — Extended Mechanics

16. [Extended Mechanics](#16-extended-mechanics)  
    - 16.1 [Console Command Grammar](#161-console-command-grammar)  
    - 16.2 [Wonder Cost Balancing](#162-wonder-cost-balancing)  
    - 16.3 [Combat Balancing](#163-combat-balancing)  
    - 16.4 [Building Cost and Time Curves](#164-building-cost-and-time-curves)  
    - 16.5 [Map Generation](#165-map-generation)  
    - 16.6 [Round-Over and Reset](#166-round-over-and-reset)  
    - 16.7 [Multi-World Membership](#167-multi-world-membership)  
    - 16.8 [Spawn Placement and Balancing](#168-spawn-placement-and-balancing)  
    - 16.9 [Fog of War](#169-fog-of-war)  
    - 16.10 [Terrain Effects](#1610-terrain-effects)  
    - 16.11 [Special Map Features](#1611-special-map-features)

### Part III — Platform

17. [Account, Identity, and Infrastructure](#17-account-identity-and-infrastructure)  
    - 17.1 [Authentication and Identity](#171-authentication-and-identity)  
    - 17.2 [Anti-Cheating and Griefing](#172-anti-cheating-and-griefing)  
    - 17.3 [Out-of-Game Integrations](#173-out-of-game-integrations)  
    - 17.4 [Persistence Model](#174-persistence-model)  
    - 17.5 [Tech Architecture](#175-tech-architecture)

### Part IV — Business and Future

18. [Business Model and Rollout](#18-business-model-and-rollout) (Pending)  
    - 18.1 [Monetization](#181-monetization-pending)  
    - 18.2 [Launch and Growth Strategy](#182-launch-and-growth-strategy-pending)  
19. [Future Enhancements](#19-future-enhancements)  
    - 19.1 [Specialized Unit Types](#191-specialized-unit-types)  
    - 19.2 [Heroes and Champions](#192-heroes-and-champions)  
    - 19.3 [Quest and Event System](#193-quest-and-event-system)  
    - 19.4 [Cosmetics and Personalization](#194-cosmetics-and-personalization)  
20. [Quick-Reference Summary](#20-quick-reference-summary)

---

# Part I — Foundations

The committed core design. Sections 1–15 define the game's identity, loop, and base mechanics. Everything in Parts II and III layers on top of this foundation. Changes here cascade widely and should be made deliberately.

---

# 1\. Project Motivation and Objectives

### Problem

Developers experience frequent micro-idle moments during the workday (waiting for builds, tests, deployments, CI pipelines, package installs, LLM responses). Current options for filling these gaps (social media, news, mobile games) are unsatisfying or hostile to flow.

Separately, remote and hybrid work have eroded informal team bonding. Slack helps but is shallow. Scheduled team-building events feel forced.

### Opportunity

Build a game that fits developer micro-idle time **and** functions as a continuous low-effort team-building mechanism.

### Project objective

An online multiplayer persistent game that:

- Fits naturally into a developer's workflow as a context-switch activity  
- Creates ongoing social engagement between coworkers in the same company or office  
- Rewards consistent short interactions over long play sessions  
- Feels native to the developer environment via console commands

### Target user

Software developers and adjacent technical roles (SRE, data engineers, DevOps, etc.) who:

- Spend their day in a terminal or IDE  
- Have multiple small idle windows per day  
- Work in companies with at least a handful of technical colleagues  
- Appreciate text-based, command-driven interfaces

### Core design principles

- **Short session friendly**: any meaningful action takes seconds, not minutes  
- **Persistent world**: progress continues while offline, check-ins feel rewarding  
- **Coworker-bound worlds**: each "world" maps to a company or office. The team-building engine.  
- **Console-native**: the game lives where developers already are  
- **Asynchronous social loop**: actions take real time to complete, creating natural conversation hooks

### Inspiration

Ogame (browser-based persistent strategy MMO).

### Success criteria

- Developers play voluntarily during idle moments instead of switching to social media  
- Coworkers reference in-game events in real-life conversation (Slack, coffee, standup)  
- Companies see it as a low-cost organic alternative to scheduled team-building  
- High DAU relative to session length (many short sessions, not few long ones)

### Non-goals

- Replacing focused leisure gaming  
- Competing with AAA or mobile games on production value  
- Requiring dedicated play time outside the workday  
- Building a general-audience game

---

# 2\. Theme

### Decision

**Medieval fantasy kingdoms.**

### Reasoning

Selected from the most popular board game themes (BoardGameGeek research). Chosen for:

- Broadly appealing, instantly readable  
- Massive creative latitude (factions, units, geography, lore)  
- Easy to characterize ("your kingdom vs your coworker's kingdom")  
- Distinct from Ogame's space theme (avoiding being a clone)

### Alternatives considered

- **Sci-fi / space**: too on-the-nose for an Ogame-inspired game  
- **Historical / civilization**: less creative latitude, real-world friction  
- **Mythological** (Greek, Norse, Egyptian): strong but narrower than fantasy  
- **Pirates / nautical**: smaller scope, less variety in factions  
- **Post-apocalyptic / horror**: doesn't fit team-building tone  
- **Dev-themed** (distributed systems, open source): rejected by user. The game does not need to be related to dev work

---

# 3\. Core Gameplay Loop

### Decision

**Spine A (conquest / raiding, Ogame model) with light E (diplomacy) elements layered on.**

### Reasoning

- Async raids generate the best office conversation ("you attacked me at 3am?\!")  
- Ogame proves the loop works with this exact session pattern  
- Console commands map naturally to military orders  
- Easy to balance attacker vs defender so nobody feels griefed into quitting  
- Diplomacy layer adds team-building potential without dominating the design

### Alternatives considered

- **B. Territorial expansion** (Risk/Diplomacy spine): considered but conquest covers it  
- **C. Economic dominance** (Brass/Catan spine): too low-conflict for office drama  
- **D. Hero/quest progression** (RPG spine): doesn't suit coworker conflict dynamics  
- **E. Pure diplomacy/intrigue** (GoT spine): too dependent on simultaneous activity  
- **F. Hybrid all-of-the-above**: too complex to design and balance

---

# 4\. Session Shape and Round Structure

### Decisions

| Aspect | Value |
| :---- | :---- |
| Session length range | 30 seconds to 5 minutes |
| Session length sweet spot | 1 to 3 minutes |
| Sessions per day | 1 to 10 |
| Absence tolerance | Moderate (offline for hours fine, days hurts) |
| Vacation mode | None |
| Newbie protection | None |
| Attack travel time | Hours to days |
| Round length (typical) | 2 to 4 weeks |
| Round end trigger | Wonder completed (no fallback, no time cap) |
| Round restart | Full reset, all players from zero |

### Reasoning

- Session length must fit a build-wait gap (typical CI runs are 30s-5min)  
- Multi-session days reward regular check-ins without demanding constant attention  
- Moderate absence tolerance: a weekend off is survivable, a week off hurts  
- No vacation mode keeps the game competitive and avoids exploits (people declaring vacation strategically)  
- No newbie protection forces the design to handle late joiners through other means (see onboarding)  
- Long attack travel times create defender warning windows, the foundation of async drama  
- Round-based structure (vs persistent) avoids snowballing and lets new cohorts start fresh  
- 2-4 week rounds match workplace rhythms and avoid fatigue

### Alternatives considered

- **Ultra-short sessions (\< 30s)**: too shallow, no decisions  
- **Medium sessions (5-10min)**: doesn't fit build-wait pattern  
- **Permanent universe** (Ogame style): too long for round-based drama, snowballing problem  
- **Punishing absence model**: hostile to workplace reality (vacations, sick days, crunch)  
- **Hard time cap on rounds (e.g. fixed 14 days)**: rejected. Round must end with a decisive event, not a whimper

---

# 5\. Win Condition

### Decision

**Wonder victory only.** No score fallback. No time cap. Round ends when a Wonder is completed and survives Consecration (see Section 14).

### Reasoning

- Decisive, dramatic ending creates the "final boss" moment of the round  
- Forces dominant players to expose themselves (cannot win by hiding)  
- Round-end becomes a watercooler event, not an anticlimactic leaderboard  
- Even a long round (3+ months) is acceptable if it ends with drama  
- Single condition is easier to teach and reason about than multiple paths

### Alternatives considered

- **A. Conquest victory** (eliminate rivals or hold X% of map): can drag indefinitely  
- **B. Score victory** (highest at end of fixed time): anticlimactic  
- **D. Throne / king victory** (capture and hold central location): exhausting for the holder, requires availability  
- **E. Alliance victory** (coalition meets condition): kingmaker problems, "everyone joins winner" cascades  
- **F. Multiple paths** (any of A-E): too complex to balance  
- **C. Wonder \+ score fallback after N days**: rejected. User wanted no fallback

---

# 6\. Resources

### Decision

**Four resources: Gold, Wood, Stone, Iron.**

| Resource | Role |
| :---- | :---- |
| Gold | Universal currency, trade, mercenaries, upkeep. The liquid resource. |
| Wood | Early buildings, basic units, ships. The renewable resource. |
| Stone | Fortifications, advanced buildings, the Wonder. The defensive / endgame resource. |
| Iron | Weapons, elite military units. The military resource. |

### Reasoning

- Four distinct resources support strategic specialization without overwhelming new players  
- Stone as the Wonder-critical resource creates a clear late-game bottleneck to fight over  
- Iron as a dedicated military resource gives combat its own economy  
- Fits in a single terminal status line (UI constraint)  
- Avoids complexity of population, food, mana systems

### Alternatives considered

- **One resource (Gold only)**: too shallow  
- **Two resources** (Gold \+ Iron): no strategic specialization  
- **Three resources** (Ogame style): solid but lacks distinct military resource  
- **Five+ with Population**: adds bookkeeping, deferred to later if needed  
- **Five+ with Food**: forces upkeep mechanics, scope creep  
- **Five+ with Mana**: implies separate magic system, scope creep  
- **Many resources (7+)** (Brass style): too heavy for short sessions

---

# 7\. Resource Generation

### Decision

**Hybrid model: production buildings at home \+ capturable nodes on a shared map.**

### Mechanics

- Each player has a kingdom with 4 production buildings (one per resource), upgradable  
- Resource nodes (e.g. Ancient Forest, Iron Vein, Marble Quarry, Gold Hoard) scattered across the map  
- Each node provides bonus income to its owner  
- Nodes can be captured by dispatching armies (requires Catapults)  
- Nodes can be contested: another player can attack and seize them — **except a kingdom's Home Hoard node, which is permanently reserved for its home kingdom and can never be seized** (§16.5)  
- Kingdom stockpiles can be raided (steal a percentage) — this includes raiding a rival's home region, which loots resources but never takes the Home Hoard node  
- Kingdoms cannot be permanently conquered (home base survives the round)

### Reasoning

- Buildings provide reliable baseline income for short check-in sessions  
- Nodes tie combat directly to economic damage (lasting consequence, not just one-time loot)  
- Geography matters: who is near which node creates natural rivalries and alliances  
- Forces Wonder-builders to expose themselves on the map (need to control nodes)  
- Provides ongoing combat interest beyond just raids

### Alternatives considered

- **A. Pure building production** (Ogame): combat feels disconnected from economy  
- **B. Worker assignment** (Settlers/Anno): too many clicks per session  
- **C. Pure territory control**: removes the "home base" feeling, more chaotic  
- **D. Card draw**: bad fit for persistent async play  
- **E. Hybrid (chosen)**: best of both worlds

---

# 8\. Map / World Structure

### Decision

**Region/province map with named regions and adjacency graph. Full visibility in v1 (fog of war designed in Section 16.9, ships in v1.1).**

### Mechanics

- World divided into \~20 to 60 named regions, scaling with player count (formula in 16.5)  
- Each region contains 0-2 resource nodes of varying quality  
- Each player's kingdom occupies one region at spawn  
- Regions connected by adjacency (graph)  
- Travel time depends on path length and slowest unit in army  
- Some regions are neutral wilderness (no kingdom, just nodes or empty)  
- Full map visibility: knowing where everyone is supports the diplomacy layer

### Reasoning

- Region maps render cleanly in console (named list with adjacencies, no per-tile rendering)  
- Named regions ("Ironwood", "Pale Coast", "Dragon's Tooth Pass") give world flavor  
- Natural neighborhoods create clear neighbors and rivals (coworker drama)  
- Choke points emerge organically from graph structure  
- Scales with player count (8 players → 26-region map; 24 players → 64-region map per 16.5)  
- Fresh map per round keeps things interesting

### Alternatives considered

- **A. Coordinate grid** (Ogame): less character, no built-in choke points  
- **B. Hex/tile map** (Civ): hard to render in console, terrain complexity  
- **D. Graph network** (subway-like): less geographic intuition  
- **E. Abstract distance** (no map): loses neighborhood/social structure

### Resolved deferrals

These map-related deferrals from earlier drafts have since been committed in Section 16:

- **Fog of war**: designed in 16.9, ships v1.1  
- **Terrain effects**: defined in 16.10  
- **Procedural generation**: committed in 16.5  
- **Spawn placement and respawn rules**: committed in 16.8

---

# 9\. Military System

### Unit Decision

**6-8 tiered units across three tiers, with light rock-paper-scissors layer.** Exact stats and costs in 16.3.

| Tier | Unit | Role | Cost profile |
| :---- | :---- | :---- | :---- |
| Light | Levy / Militia | Cheap chaff, fast to produce | Low Wood, Low Iron |
| Light | Archer | Ranged, weak in melee | Wood-heavy |
| Medium | Knight | Strong cavalry, fast travel | Iron-heavy, Gold |
| Medium | Pikeman | Anti-cavalry, solid defender | Wood, Iron |
| Heavy | Catapult | Siege, **needed for node capture** | Wood, Stone, Iron |
| Heavy | Royal Guard | Elite defender, expensive | Iron, Gold, Stone |
| Optional | Scout | Cheap, fast, gathers intel | Gold |
| Optional | Trebuchet | **Anti-Wonder siege weapon** | Heavy Stone, Iron |

**Rock-paper-scissors layer**: Knights crush Archers, Pikemen crush Knights, Archers crush Pikemen. Catapults break fortifications. Trebuchets damage Wonders.

### Combat Resolution Decision

**Multi-round simulation (6 rounds) with detailed battle log.** Full formulas in 16.3.

- Both sides exchange attacks each round, casualties accumulate  
- Mild randomness (5-10% variance)  
- Defender bonus \+20% in home region  
- Battle log: round-by-round casualties, MVP unit, loot captured  
- Both sides take casualties  
- Attackers can retreat early (reduced loot)

### Loot Rules

- Successful raid steals up to a cap (\~25% of stockpile, or carrying capacity)  
- Successful node attack transfers ownership  
- Successful Wonder attack damages or destroys construction

### Reasoning

- Tiered units give late-game progression: a Royal Guard army signals power  
- RPS prevents snowballing: 1000 Knights can be beaten by coordinated Pikeman alliance  
- Catapults gate node capture: prevents trivial early land grabs  
- Trebuchets dedicated to Wonder destruction: clear strategic purpose  
- Multi-round combat with detailed logs creates shareable office content  
- Defender bonus \+ travel times keep attacking risky and decision-heavy

### Alternatives considered

- **A. Single unit type**: no strategic depth  
- **B. Pure 3-4 RPS**: less progression feel  
- **C. Full Ogame scale (10-12 units)**: too heavy for short sessions  
- **D. Class system with upgrades**: complex to implement, not necessary  
- **E. Hero-led armies**: scope creep  
- **Combat: instant deterministic**: boring, no battle drama  
- **Combat: instant with variance**: too random for player commitment  
- **Combat: real-time/live**: bad fit for async play

---

# 10\. Buildings and Progression

### Building Set Decision

**12 buildings, single upgrade slot, linear levels with tier gates.** Final building list and base costs in 16.4 (note: Town Hall replaces the earlier Vault concept, Warehouse handles storage).

| Category | Building | Purpose |
| :---- | :---- | :---- |
| Town | Town Hall | Unlocks tiers, grants extra build queue at L10/L20 |
| Resource | Gold Mint | Produces gold |
| Resource | Lumber Camp | Produces wood |
| Resource | Quarry | Produces stone |
| Resource | Iron Mine | Produces iron |
| Storage | Warehouse | Increases stockpile cap, raid loot scales with cap |
| Military | Barracks | Trains Levy, Archer, Pikeman |
| Military | Stable | Trains Knights, Scouts, Royal Guard |
| Military | Siege Workshop | Trains Catapult and Trebuchet |
| Defensive | Walls | Boosts defender bonus in your kingdom |
| Defensive | Watchtower | Detects incoming attacks earlier, more lead time |
| Support | Stone Mason | \-2% build time per level, cap \-30% |

The Wonder is a special endgame structure (Section 14), not counted in the 12 standard buildings. Marketplace and Library from earlier drafts were folded into Stone Mason (build discount) and Town Hall (tier unlocks); see 16.4 for the final set.

### Upgrade Mechanics

- Linear levels 1-20 (soft cap for round-based play)  
- Cost scales exponentially: \~1.75x per level (committed in 16.4)  
- Build time scales exponentially with a 24h cap (committed in 16.4)  
- **Single upgrade slot**: only one building upgrade in progress at a time (Town Hall unlocks extra slots at L10/L20)

### Tier Gates (rough sketch, see 16.4 for finalized version)

- Stable requires Barracks 3  
- Siege Workshop requires Barracks 5 and Iron Mine 5  
- Wonder requires building prerequisites, 3 controlled nodes, and unlock cost (see 14 and 16.2)

### Storage Caps (Warehouse)

- Warehouse sets per-resource cap. Production stops at cap (forces spending)  
- Higher Warehouse levels grow cap quadratically (see 16.4)  
- Cap size shapes raid economics: bigger cap \= bigger loot pool

### Reasoning

- 12 buildings is enough variety for daily decisions without overwhelming new players  
- Single slot keeps sessions short: log in, evaluate, queue, log out (no queue management)  
- Single slot also limits compulsive checking: queued 6h build \= nothing to do for hours  
- Linear levels with exponential cost is a proven, satisfying progression curve  
- Tier gates create memorable milestones (unlocking Siege Workshop, unlocking Wonder)  
- Warehouse cap as raid target ties combat to economy meaningfully  
- Stone Mason as build-time discount replaces the earlier Library/research concept, kept lean

### Alternatives considered

- **A. Minimal building set (5-8)**: too shallow, runs out of decisions  
- **C. Rich set (20+ Ogame scale)**: too heavy for round-based play  
- **Tiered tech tree separate from buildings**: adds complexity  
- **Modular slot-based**: less progression feel  
- **Multiple concurrent upgrades**: invites pay-to-win or compulsive checking

---

# 11\. Timing Model

This section sets target ranges. Concrete formulas are in 16.4 (buildings), 16.3 (units, march), 16.2 (Wonder).

### Build Time Curve

| Level range | Typical build time | Player experience |
| :---- | :---- | :---- |
| 1 to 3 | 2 to 20 min | Day 1, lots of decisions, fast feedback |
| 4 to 6 | 1 to 4 h | Day 1-2, multi-check-in pacing |
| 7 to 10 | 4 to 16 h | Day 2-7, "queue and come back later" |
| 11 to 15 | 16 to 24 h (capped) | Day 5-12, overnight builds |
| 16 to 20 | 24 h (capped, exponential cost) | Day 10+, major commitments |

Cost scaling: 1.75x per level. Time scaling: 1.55x per level with 24h cap (see 16.4).

### Unit Training Times

- Light units (Levy, Archer): 45-90s per unit, scales with Barracks level  
- Medium units (Knight, Pikeman): 3-4min per unit  
- Heavy units (Catapult, Royal Guard): 20-25min per unit  
- Trebuchet: 45min per unit  
- Each military building has independent training queue

### March Times

- Adjacent region: 30min to 2h  
- 2 hops: 2-5h  
- Cross-map (5+ hops): 12-36h  
- Army moves at slowest unit's speed  
- Knights and Scouts ignore terrain penalties (see 16.10)

### Resource Production

- Level 1 building: 25-40 units / hour  
- Level 10: \~250-400 / hour  
- Level 20: \~500-800 / hour  
- Captured nodes add flat bonuses (+120 to \+500 / hour by tier, see 16.5)

### Wonder Construction (see Section 14 and 16.2)

- Total cost: 800K Gold, 600K Wood, 2.4M Stone, 800K Iron  
- Construction phase: 90 hours at fixed 100 HP/h rate  
- Plus 24h Consecration phase  
- Total Wonder time: \~84-120 hours from Foundation to round-end

### Round Shape (typical 2-4 weeks)

- **Day 1-4**: setup phase  
- **Day 5-10**: mid-game, node grabs, first raids  
- **Day 10-16**: heavy mid-game, advanced units, stockpiling stone  
- **Day 14-28**: Wonder phase, repeated attempts, round ends  
- **Tail beyond 28 days possible** if Wonders repeatedly destroyed

### Concurrency Rules

- 1 building upgrade at a time (single slot, Town Hall unlocks more)  
- Each military building has its own training queue (Barracks, Stable, Siege Workshop independent)  
- Multiple armies can march simultaneously  
- Multiple incoming attacks can hit the same target

### Reasoning

- Early-game fast pacing makes day 1 rewarding  
- Mid-game stretches into "queue and forget" so game doesn't demand constant attention  
- Late-game serious commitments (24+ hour builds, day-long marches) create real strategic tension  
- Travel times scale with distance: geography matters, defenders get meaningful warning windows  
- Curve targets a typical 2-4 week round

---

# 12\. Diplomacy / Social Layer

### Decision

**Resource trade only.** No formal pacts, alliances, or intel sharing in the game.

### Rationale

- All coordination, alliances, betrayals, and intel sharing happen out of game (Slack, voice, in person)  
- Office context already provides chat channels. The game doesn't need to replicate them  
- Forces real conversation between coworkers (the team-building goal)  
- Significantly reduces design and build effort  
- The game provides the *state* worth talking about, the social layer is provided by the office

### Trade Mechanics

- Resources sent to another player at any time  
- Send command: `send <player> <amount> <resource>`  
- Resources travel as **caravans** that march like armies  
- Travel time \= same as army march between regions  
- Caravans can be **intercepted en route** by other players' armies  
- Caravans need **escort units**. Carrying capacity \= sum of escort capacity  
- No fees, no taxes, no caps (gated naturally by carrying capacity and march time)  
- All trades recorded in a public ledger (see 17.2)

### Marketplace

Player-listed offers and order-book trading are deferred. The Marketplace building from earlier drafts was folded into other buildings (see 10). Direct caravan trade is the only v1 mechanic.

### What is NOT in the game

- No formal pacts or non-aggression agreements  
- No alliance entities, ranks, or shared resources  
- No alliance chat  
- No formal intel sharing (scout reports cannot be officially shared, though screenshots and Slack work)  
- No diplomatic actions like marriage, vassalage, tribute

### Reasoning

- Lightweight design: less to build, less to balance  
- Coworker bonding is *strengthened* by forcing communication outside the game  
- The "single Slack message coordinating an attack" is the team-building moment  
- Avoids the alliance-mega-block problem (everyone joins the winning alliance) common in MMOs

### Alternatives considered

- **A. No formal mechanics at all** (pure social, no trade): rejected. Trade adds meaningful interaction  
- **B. Lightweight pacts** (NAP with 24h notice to break): considered, rejected to keep game lean  
- **C. Full alliance system**: rejected as too heavy and risks rigid teams  
- **E. Combined pacts \+ trade \+ intel**: rejected to keep game lean

---

# 13\. Player Onboarding

### World Creation Decision

**Company-scheduled rounds with on-demand player triggering.**

- Designated organizer (or any player) at a company can propose a new round  
- Sends invitations to coworkers via company channel (Slack, email, etc.)  
- Round starts when both conditions are met:  
  - Minimum player count is reached (e.g. 4 players), AND  
  - A scheduled start time arrives (e.g. "starts Monday 9am if 4+ players")  
- Multiple concurrent worlds per company allowed (see 16.7 for limits)  
- Empty world auto-cancels if minimum signups not reached in N days

### Joining Decision

**Closed start with 72-hour grace window.**

- Round officially starts at scheduled time T0  
- New players join freely until T0 \+ 72 hours  
- After 72 hours, round closed to new joiners  
- Grace-window joiners spawn with stockpile bonus to absorb time gap (see 16.8)  
- Late joiners after grace window must wait for next round

### First-Session Decision

**Pre-built starter kingdom \+ contextual hints.**

New player spawns with:

- Each of 4 resource buildings at level 1  
- Barracks at level 1  
- Walls and Watchtower at level 1  
- 500 of each resource (plus grace-window bonus per 16.8)  
- 20 Levy soldiers at home

Plus contextual help in command output:

- Hints in command output (fade after first few uses)  
- `tutorial` command for opt-in guided walkthrough  
- No separate sandbox world: learn in the real round  
- No forced tutorial

### Reasoning

- Company-scheduled with on-demand trigger matches workplace reality  
- Multiple worlds per company let large companies run parallel rounds (by team, by floor, by interest)  
- 72-hour grace window absorbs missed Slack notifications without permanent imbalance  
- Pre-built starter balances "no protection" stance with reality: fresh players need something to do  
- Opt-in tutorial preserves developer aesthetic without forcing hand-holding

### Alternatives considered

- **B. Continuous overlapping rounds** (auto-spawn): less workplace control  
- **C. Fixed monthly cadence**: too rigid for workplace reality  
- **A. Closed start, no late joiners**: punishing for missed signups, new hires  
- **B. Open joining anytime**: fresh kingdom on day 10 is unwinnable  
- **D. Late joiners get scaled state**: rejected for simplicity. Stockpile bump in 16.8 is the chosen compromise  
- **A. Empty kingdom (figure it out)**: too brutal even for developers  
- **B. Forced tutorial quest line**: violates developer aesthetic  
- **D. Sandbox tutorial world**: scope creep, deferred

### Resolved follow-ups

These onboarding follow-ups from earlier drafts have since been committed:

- Spawn placement for late joiners: 16.8  
- Round announcement integration: 17.3 (deferred to post-v1 by decision)  
- Minimum player count configurability: 16.7  
- Multi-world membership: 16.7  
- Round-over reset: 16.6  
- Cosmetic carry-over: titles committed in 17.4, broader cosmetics in 19.4

---

# 14\. Wonder Mechanics

This section defines the Wonder as the round-end mechanic. Full cost balancing, HP, and damage formulas are in 16.2.

### Thematic Identity Decision

**Player chooses from a menu of Wonders, all mechanically identical.**

- Options: Sky Tower, Eternal Citadel, Cathedral of Ages, Library of Worlds, Crown of Kings, Black Spire (final list TBD)  
- Mechanically identical (just flavor)  
- Builder declares choice when starting construction  
- Battle reports and notifications use chosen name

### Location Decision

**Built in player's home kingdom.**

- Wonder occupies a special slot in the kingdom (like a 13th building)  
- Defended by player's normal home garrison \+ walls and watchtower bonuses  
- No race for a "Sacred Site"  
- Reinforces "your home is your fortress"

### Prerequisites Decision

**Building levels \+ node control \+ unlock cost.** Exact prerequisites finalized alongside the building set in 16.4. Current target:

- Specific building level gates (e.g. Quarry 10, Siege Workshop 5\)  
- Control at least 3 resource nodes on the map  
- One-time unlock cost matching the Foundation payment in 16.2

### Construction Model Decision

**Three phases: Foundation → Construction → Consecration.** Exact resource amounts in 16.2.

| Phase | What happens | Duration |
| :---- | :---- | :---- |
| **Foundation** | Pay 25% of total Wonder cost. Construction begins. Wonder visible at HP 1000\. | Instant |
| **Construction** | Builds at fixed 100 HP/h to 10,000 HP. Milestone payments at 25%, 50%, 75%. Missing a milestone pauses construction. | 90 hours |
| **Consecration** | Final 24 hours. Pay final 5%. Maximum vulnerability (no construction crew to repair full-speed). | 24 hours |

**Total Wonder time: \~84-120 hours from Foundation to round-end.**

### Attack and Damage Decision

**Multiple attack vectors, each with a role.**

| Unit | Role |
| :---- | :---- |
| Trebuchets | Damage Wonder HP (primary anti-Wonder weapon, 50 HP per surviving unit) |
| Catapults | Damage Walls (reduces defender bonus) |
| Knights/Royal Guards/Levies | Kill garrison, steal Wonder-construction resources |

Full damage and repair mechanics in 16.2:

- Wonder destroyed at HP 0: construction lost, all paid resources lost, builder must restart from scratch  
- Wonder damaged but not destroyed: builder can spend Stone to repair (1 HP per 8 Stone, with per-phase caps)

### Defense and Repair

- During construction, kingdom's normal building queue is **locked** to the Wonder (no other upgrades)  
- Unit training continues normally (defender keeps training Royal Guards)  
- Walls and Watchtower can still be upgraded  
- Wonder repair pauses construction briefly (see 16.2 for exact timing)  
- A "Wonder Garrison" specialized unit deferred to 19.1

### Round End Decision

**Wonder must survive the 24-hour Consecration phase.**

- Consecration *is* the survival timer  
- Maximum vulnerability during Consecration, world-wide notification  
- The dramatic moment of the round: final 24-hour push by everyone vs the builder  
- If Wonder survives Consecration: builder wins, round ends, world resets (see 16.6)  
- If destroyed during Consecration: builder must restart Wonder, round continues

### Reasoning

- Public construction with multiple visible phases maximizes office drama (each phase \= Slack moment)  
- Multi-vector attacks make stopping the Wonder a *team* effort (the team-building goal)  
- Locked build queue during construction \= real sacrifice (builder bets everything on the push)  
- Restart penalty (lose paid resources) makes destruction devastating but not round-ending  
- Consecration timer is the drama amplifier: round ends with last stand, not quiet click  
- Choosing a Wonder name personalizes identity ("Marta's Sky Tower")  
- No race for Sacred Site keeps focus on builder vs world

### Resolved follow-ups

These Wonder follow-ups from earlier drafts have since been committed:

- Exact total Wonder cost: 16.2  
- Exact Trebuchet damage per surviving unit: 16.2 (50 HP)  
- Wonder repair cap per phase: 16.2 (2000 HP)  
- Multiple simultaneous Wonders: 16.2 (independent, allowed)  
- Winner cosmetic reward: 17.4 (titles)

---

# 15\. Console Interaction Sketches

Not committed as the final command surface. These are illustrative mockups of how interactions feel. The full command grammar is committed in 16.1.

### Status / Kingdom Overview

\> status

Pale Coast \- Day 4, 18h elapsed

Resources: 12,400 / 50,000 G   18,200 / 50,000 W   8,400 / 50,000 S   9,800 / 50,000 I

Production: \+480 G/h  \+620 W/h  \+280 S/h  \+340 I/h

Build queue:

  Barracks lvl 6        2h 14m remaining

Training:

  Barracks: 40 Levy (12m), 15 Archer (1h 30m)

  Stable:   8 Knight  (32m)

Armies:

  1st Host (home)  \- 80 Levy, 30 Archer, 15 Knight

  2nd Host (marching to Dragon's Tooth, ETA 1h 12m) \- 5 Knight (scouting)

Incoming:

  Bob   \-\> Pale Coast  (180 Levy, 40 Knight, 8 Catapult)  ETA 3h 45m

  Marta \-\> Iron Vein (your node)  (30 Knight)  ETA 1h 8m

### Map

\> map

World: Aldermarch, 24 regions, 8 kingdoms

  \[1\] Ironwood        (Marta)        nodes: Ancient Forest

  \[2\] Pale Coast      (you)          nodes: Gold Hoard

  \[3\] Dragon's Tooth  (neutral)      nodes: Iron Vein, Marble Quarry

  \[4\] Mistmarsh       (Bob)          nodes: \-

  ...

\> map adjacent

Pale Coast borders:

  \- Ironwood       (Marta)     \~2h march

  \- Mistmarsh      (Bob)       \~3h march

  \- Dragon's Tooth (neutral)   \~1h30m march

### Attack

\> attack ironwood with 200 levy 50 archer 30 knight 10 catapult

March order issued.

Target: Ironwood (Marta)

ETA: 2h 14m

Carrying capacity: 45,000

### Battle Report

\> battle 47

\=== BATTLE OF PALE COAST \===

Attacker: Bob (180 levy, 60 knight, 5 catapult)

Defender: you (200 levy, 80 pikeman, 20 archer) \[+20% defense\]

Round 1: ...

Round 2: ...

...

Result: DEFENDER VICTORY

### Trade

\> send marta 5000 stone

Caravan dispatched: 5000 stone to Marta.

Escort required: minimum capacity 5000\.

ETA: 2h 14m.

\> marketplace

Pale Coast Marketplace (lvl 4):

  1000 W  \-\>  720 G

  1000 S  \-\>  1100 G

  1000 I  \-\>  900 G

  1000 G  \-\>  650 W / 480 S / 580 I

### Wonder

\> wonder

You have not started a Wonder.

Prerequisites:

  Library lvl 10        ✓ (lvl 11\)

  Quarry lvl 10         ✓ (lvl 10\)

  Smithy lvl 5          ✓ (lvl 6\)

  Resource nodes ≥ 3    ✓ (4 controlled)

  Unlock cost           need 50,000 G / 50,000 W / 100,000 S / 50,000 I

                        have 62,000 / 48,000 / 87,000 / 51,000

\> wonder start "Sky Tower"

WARNING: Starting a Wonder pays 25% of total cost upfront and locks your build queue.

Confirm? \[y/N\]

\> wonder

\=== THE SKY TOWER (you) \===

Phase: Construction (45% complete)

HP: 920 / 1000

Next milestone: 50% (cost: 100,000 S)  in 4h 12m

Estimated completion: Day 18, 14:30

Garrison: 80 Royal Guard, 200 Pikeman, 40 Archer

Recent attacks: 2 (last: 6h ago, 80 HP damage, repaired)

\> wonder repair 200

Repairing 200 HP for 2,000 Stone.

Construction paused for 30 minutes during repairs.

### World Announcements

WORLD ANNOUNCEMENT (visible to all):

  \>\>\> Marta has begun building THE SKY TOWER in Ironwood. \<\<\<

  \>\>\> Construction will complete in approximately 96 hours. \<\<\<

WORLD ANNOUNCEMENT:

  \>\>\> Marta's SKY TOWER has entered Consecration phase. \<\<\<

  \>\>\> Round will end in 24 hours unless the Wonder is destroyed. \<\<\<

---

# Part II — Extended Mechanics

Section 16 collects mechanics that were originally queued as pending decisions and have since been committed. Each subsection is a focused design pass: console grammar, Wonder costs, combat numbers, building curves, map generation, and the round lifecycle.

These layer on top of Part I rather than replacing it. Where 16.x and earlier sections disagree, 16.x is the authoritative source for the specific numbers involved (Part I provides ranges and intent, Part II provides formulas).

---

# 16\. Extended Mechanics

## 16.1 Console Command Grammar

The interface design for the `dun` CLI: transport, session model, command surface, output formatting, and discovery. The shape committed here is what every other section's commands plug into.

### Committed

**Transport and session**

- Transport: Local CLI binary (`dun`) over authenticated HTTP/JSON API. SSH-hosted shared instance deferred to post-v1.  
- Session model: Stateful REPL by default (`dun` drops to `dun>` prompt). One-shot mode for scripting (`dun status` runs and exits).  
- REPL features: persistent prompt, login banner with urgent state summary, tab completion, server-side command history, `\?` help, confirmations for destructive actions.

**Grammar (mixed)**

- Frequent actions are top-level verbs with natural-order arguments.  
- Noun-heavy domains use subcommands.  
- Meta commands use backslash prefix (psql-style).

Top-level verbs: `status`, `map [region]`, `attack <region> with <units>`, `march <region> with <units>`, `build <building>`, `train <unit> <count> [in <building>]`, `send <player> <amount> <resource>`, `scout <region>`, `recall <army>`, `report [id]`.

Noun subcommands: `army list | show | split | rename`, `wonder [start | repair | cancel]`, `node list | show`, `market | market trade`, `player list | show`, `world | world info`.

Meta commands: `\?`, `\h <command>`, `\q`, `\history`, `\clear`, `\json on|off`, `\color on|off`, `\hints on|off`.

**Output format**

- Color: default on, auto-detect TTY, respect `NO_COLOR`. Override with `--color=always|never|auto` or `\color`.  
- Palette: ANSI 16 only. Red (warnings), yellow (in-progress), green (success), cyan/blue (self), dim (secondary), bold (headers/current values). No backgrounds.  
- Structured mode: plain text default, JSON via `--json` or `\json on`. Format strings deferred to v1.1.  
- Tables: aligned columns, no borders. Headers bold. Default 4-6 columns, `--wide` shows all. Truncate with ellipsis.

**Help and discovery**

- Help: `\?`, `\h <command>`, bare `help`, `--help`, `-h` all work.  
- Tab completion: server-driven, covers commands, regions, players, armies, buildings, units, resources. 30s cache per session.  
- Contextual hints: one-line next-step suggestions appended to command output. Fade after 3-5 uses per command. Toggle with `\hints`.  
- Errors: did-you-mean for typos. Validation before destructive confirms. Every error suggests a fix.

**Async notifications (v1 scope)**

- In-session banner only: login MOTD shows urgent state (incoming attacks, completed builds, arriving caravans, Wonder events).  
- Slack and email notifications deferred to post-v1 (see 17.3). API surface designed to support them when added.

### Deferred to post-v1

- SSH-hosted shared instance  
- Slack bot (per-company app, DMs, world announce channel)  
- Email digests  
- Format strings (`--format` templates)  
- Webhooks  
- Contextual `\?` suggestions (currently static)  
- Mobile push, SMS, Discord, Teams (likely never)

---

## 16.2 Wonder Cost Balancing

Concrete cost, HP, damage, and repair numbers for the Wonder. Builds directly on Section 14, which defines the phases and intent.

### Committed

**Total Wonder cost**

| Resource | Total | Foundation (25%) | Each milestone @25/50/75% (10%) | Consecration (5%) |
| :---- | :---- | :---- | :---- | :---- |
| Gold | 800,000 | 200,000 | 80,000 | 40,000 |
| Wood | 600,000 | 150,000 | 60,000 | 30,000 |
| Stone | 2,400,000 | 600,000 | 240,000 | 120,000 |
| Iron | 800,000 | 200,000 | 80,000 | 40,000 |

Stone is 3x other resources. The Wonder is the dedicated late-game Stone sink that gives captured Quarry nodes lasting value.

**Wonder HP and construction**

- Wonder HP at Foundation: 1,000  
- Wonder HP at full Construction: 10,000  
- Construction rate: 100 HP per hour (fixed)  
- Construction phase duration: 90 hours from Foundation to full HP  
- Milestones at 25%, 50%, 75% completion gate progress. Missing a milestone payment freezes construction at that HP until paid.

**Trebuchet damage**

- Each surviving Trebuchet does 50 HP damage to the Wonder per successful attack  
- A full-HP Wonder (10,000 HP) requires 200 surviving Trebuchets in a single strike to one-shot  
- Trebuchet cost: 8,000 Stone, 4,000 Iron, 2,000 Wood, 1,500 Gold per unit  
- Trebuchet build time at Smithy level 10: 45 min per unit

A 200-Trebuchet anti-Wonder force costs 1.6M Stone and 800K Iron, roughly 65% of the Wonder's own Stone cost. Destruction is a coalition-scale investment, not a solo play.

**Repair**

- Repair rate: 1 HP restored per 8 Stone spent  
- Repair cap: 2,000 HP per phase (Foundation, Construction, Consecration each have independent caps)  
- Repairing pauses Construction for 30 minutes per 500 HP repaired  
- Consecration repair cap is the critical constraint: builder cannot infinitely tank damage in the final 24 hours

### Reasoning

- Stone at 3x ratio gives Quarry nodes lasting endgame value. Without Wonder demand, Stone becomes dead weight by day 14, since Walls and Smithy upgrades plateau by level 15\.  
- HP at 10,000 (not 1,000) forces sustained multi-attack pressure. At 1,000 HP a single 20-Trebuchet strike one-shots the Wonder, trivializing the multi-vector attack design and making Consecration a coin flip.  
- Trebuchet cost forces coordination: 3-5 attackers pooling resources to assemble a credible Wonder-killing force. Solo Wonder-killing would reduce the round finale to a duel and exclude the rest of the office.  
- Repair caps prevent stockpile-rich builders from becoming invulnerable. The 2,000 HP Consecration cap means a builder can absorb roughly 40 surviving Trebuchets of damage in the final 24h, no more. Forces real defense via garrison, not just Stone-pumping.  
- Cost calibrated against mid-game economy: at day 14 with all Resource buildings at level 12 and 2 captured nodes, Stone production is \~30,000/hour. The 2.4M Stone Wonder cost takes \~80 hours of pure stockpiling, realistically 5-7 days with raids and node captures. Matches the committed 84-120h Wonder timeline and forces builders to commit before they can fully fund completion, creating milestone-payment tension.

### Alternatives considered

- **Equal resource ratios (1:1:1:1)**: rejected. No late-game Stone sink, Quarry nodes become irrelevant.  
- **HP at 5,000**: rejected. Still vulnerable to a single elite army strike, undermines multi-vector design.  
- **HP at 20,000**: rejected. Pushes destruction into "impossible without full-server coalition" territory, kills the dramatic-but-achievable balance.  
- **Variable construction rate (faster early, slower late)**: rejected for v1 simplicity. Fixed rate is easier to communicate and balance.  
- **No repair cap, just Stone cost**: rejected. Rich builder becomes invulnerable, breaks Consecration drama.  
- **Trebuchets at 50% cost**: rejected. Solo Wonder-killing becomes viable, excludes most of the office.

### Open follow-ups

- Whether milestone payments scale linearly (10/10/10) or back-load (5/10/15) to increase pressure as completion nears  
- Whether destroyed Wonders return any resources to the builder (currently full loss, may be too punishing on a second attempt)  
- Whether a second Wonder attempt by the same player gets a discount (anti-tilt mechanism)  
- Whether Trebuchets take attrition damage themselves during a Wonder assault (siege attrition)  
- Interaction with Watchtower: does early warning let the defender pre-train enough Royal Guards to wipe Trebuchets before they fire  
- Whether multiple simultaneous Wonders share a global HP pool or each is independent (current design: independent)  
- Trebuchet combat resolution against the Wonder garrison: how many Trebuchets actually "survive" to deal damage is partly defined in 16.3 but exact garrison-vs-siege interaction needs playtest

---

## 16.3 Combat Balancing

Unit stats, RPS multipliers, combat resolution, and defender bonuses. The numerical foundation for every battle, raid, and march in the game.

### Committed

#### Unit stats

| Unit | Atk | Def | HP | Speed (regions/h) | Capacity | Cost (G/W/S/I) | Train time |
| :---- | :---- | :---- | :---- | :---- | :---- | :---- | :---- |
| Levy | 4 | 6 | 10 | 0.5 | 50 | 20 / 30 / 0 / 10 | 45s |
| Archer | 12 | 4 | 8 | 0.5 | 30 | 30 / 60 / 0 / 20 | 90s |
| Pikeman | 8 | 18 | 16 | 0.4 | 40 | 40 / 50 / 10 / 40 | 3m |
| Knight | 25 | 12 | 20 | 1.0 | 80 | 100 / 20 / 0 / 80 | 4m |
| Catapult | 40 | 8 | 30 | 0.25 | 200 | 150 / 300 / 200 / 150 | 20m |
| Royal Guard | 30 | 35 | 40 | 0.5 | 60 | 200 / 50 / 50 / 150 | 25m |
| Scout | 2 | 2 | 4 | 2.0 | 10 | 50 / 0 / 0 / 0 | 60s |
| Trebuchet | 20 | 6 | 50 | 0.2 | 250 | 1500 / 2000 / 8000 / 4000 | 45m |

Speed unit: regions per hour assuming standard adjacency. Army moves at slowest unit's speed.

#### RPS multipliers

Applied to attacker's effective damage when its target type matches:

| Attacker | Target | Multiplier |
| :---- | :---- | :---- |
| Knight | Archer | 1.5x |
| Pikeman | Knight | 1.6x |
| Archer | Pikeman | 1.4x |
| Catapult | Walls/Watchtower | 3.0x |
| Trebuchet | Wonder | 1.0x (50 HP/unit per 16.2) |
| Royal Guard | any | 1.0x (no RPS, raw stats win) |

#### Combat resolution (6 rounds)

Each round per side:

1. Compute total Atk (sum of unit\_count × Atk × RPS\_multiplier\_vs\_dominant\_enemy\_type)  
2. Compute total Def (sum of unit\_count × Def)  
3. Damage dealt to opponent \= max(0, Atk \- Def × 0.5) × variance  
4. Variance: uniform(0.92, 1.08)  
5. Damage distributed proportionally across opponent units, weighted by unit count and inverse-HP (chaff dies first)  
6. Defender bonus: defender's Def multiplied by 1.20 in home region, 1.10 in owned non-home region, 1.0 in neutral/enemy region
   - "Home region" is keyed off the kingdom's permanent spawn assignment (`home_region_id`), **not** off who owns the region's Home Hoard node. A kingdom therefore gets its 1.20 home bonus from T0 — even before it has captured its own (still-wilderness) Home Hoard — and never loses it, since the Home Hoard can never be seized (§16.5). The walls bonus is gated the same way.

Battle ends early if one side's total HP drops below 15% of starting HP (rout). Routed side loses additional 30% of remaining units fleeing.

#### Carrying capacity rules

- Each unit contributes its capacity stat to the army total  
- Loot drawn from defender's stockpile, capped at 25% per resource or carrying capacity, whichever is lower  
- Capacity also gates caravan escort sizing (Section 12\)

#### March speed

- Base: speed stat regions/hour  
- Knights and Scouts ignore terrain penalties (see 16.10)  
- Catapult/Trebuchet armies are 4-5x slower than Knight armies, creating a real tradeoff between hitting fast and hitting hard

#### Defender bonus tuning

- Home region: \+20% Def (committed in Section 9\)  
- Walls level scales home bonus: \+1% per Wall level, capped at \+40% total at Wall 20  
- Watchtower: adds 30 minutes warning per level to incoming-attack ETA visibility (no combat effect)

### Reasoning

- **Knight as glass cannon**: 25 Atk / 12 Def encourages using them offensively, not as static defense. Pairs with their 1.0 speed for raid-focused play.  
- **Pikeman counter is real**: 18 Def \+ 1.6x vs Knights makes a Pikeman-heavy garrison a credible answer to a Knight raid, even outnumbered. Defender player has a non-frustrating answer to "Bob spammed Knights."  
- **Archer fragility**: 4 Def, 8 HP. Without a Pikeman screen they evaporate. Forces combined-arms thinking, not Archer-spam.  
- **Royal Guard as no-RPS anchor**: 30/35/40 with no multipliers means it wins through raw stats. Expensive enough (200G/150I) that a Royal Guard army signals real commitment, exactly as Section 9 intended.  
- **Catapult Atk 40 \+ 3x vs Walls**: a stack of 20 Catapults punches through Wall 10 in two rounds. Node capture stays gated behind real investment.  
- **Defender bonus capped at \+40%**: avoids invulnerable turtles. A Wall-20 \+ Royal-Guard fortress is hard but not impossible to crack with a coordinated 3-attacker assault.  
- **Variance ±8%**: enough to make battle reports feel alive, small enough that a 2x material advantage still wins \~99% of the time. Players can't blame RNG for losses they earned.  
- **Rout threshold**: prevents pyrrhic 6-round slogs. Real battles end decisively, generating cleaner Slack-shareable battle reports.  
- **Capacity-gated loot**: a Knight raid (80 cap) can grab meaningful gold but not drain a stockpile. To truly cripple, attacker must bring slow chaff (Levy at 50 cap, scales by count), exposing them to interception.

### Alternatives considered

- **Single Atk stat, no Def**: rejected. Removes meaningful defender investment, makes Walls and Royal Guard feel pointless.  
- **Higher variance (±20%)**: rejected. Too many "I had 3x his army and lost" moments. Workplace context tolerates less RNG salt than typical MMOs.  
- **Fixed-damage-per-unit, no rounds**: rejected. Kills the multi-round battle log that generates office conversation.  
- **Pure deterministic (no variance)**: rejected. Removes the "battle report watercooler" element. Slight uncertainty makes reports worth reading.  
- **RPS as 2.0x multipliers**: rejected. Too punishing, makes mono-unit armies suicidal. 1.4-1.6x is enough to matter without dominating.  
- **Defender bonus \+50%**: rejected. Made attacking too risky, broke the conquest spine.

### Open follow-ups

- Whether Catapults take attrition damage from Walls during siege (currently no, may need a tax to prevent free Catapult carry-over)  
- Whether retreat-during-march has a unit-loss penalty or is free  
- How casualties from a battle interact with carrying-capacity loot (currently capacity recomputed post-battle on survivors)  
- Scout combat: do they fight in stack battles or auto-flee? Current stats imply they fight badly. Likely should auto-flee or be excluded from combat entirely  
- Trebuchet self-defense: 20 Atk / 6 Def means an unescorted Trebuchet army gets shredded by a small garrison. Intentional (forces escort cost), but needs playtest validation  
- Interaction with multi-side battles: if Bob and Marta both arrive at Carl's region in the same hour, do they fight together against Carl, or sequentially? Recommend sequential by ETA, but worth deciding  
- Whether attack/defense values should use additive or multiplicative defender bonus stacking (Walls \+ home \+ terrain)

---

## 16.4 Building Cost and Time Curves

Concrete cost, time, and production formulas for every building. Defines the day-by-day economic shape of a round.

### Committed

**Cost formula (all buildings, all levels)**

cost(L) \= round(base × 1.75^(L-1))

**Time formula (all buildings, all levels)**

time(L) \= min(base\_time × 1.55^(L-1), 24h)

Time cap of 24h kicks in around L14-15 depending on building. High-level upgrades remain expensive but never strand players offline.

**Per-building base costs and times**

| \# | Building | Role | Base G | Base W | Base S | Base I | Base time |
| :---- | :---- | :---- | :---- | :---- | :---- | :---- | :---- |
| 1 | Town Hall | Unlocks tier, \+1 build queue at L10/L20 | 200 | 200 | 200 | 100 | 5m |
| 2 | Gold Mint | Gold/h | 100 | 150 | 50 | 0 | 2m |
| 3 | Lumber Camp | Wood/h | 80 | 50 | 80 | 0 | 2m |
| 4 | Quarry | Stone/h | 80 | 100 | 50 | 0 | 2m |
| 5 | Iron Mine | Iron/h | 100 | 100 | 100 | 0 | 3m |
| 6 | Warehouse | Storage cap | 50 | 200 | 100 | 20 | 3m |
| 7 | Barracks | Trains Levy/Archer/Pikeman | 150 | 200 | 100 | 50 | 5m |
| 8 | Stable | Trains Knight/Scout | 200 | 100 | 50 | 150 | 8m |
| 9 | Siege Workshop | Trains Catapult/Trebuchet | 300 | 400 | 200 | 200 | 15m |
| 10 | Walls | \+Def home region per 16.3 | 100 | 50 | 300 | 50 | 8m |
| 11 | Watchtower | \+Attack ETA warning | 100 | 100 | 200 | 30 | 6m |
| 12 | Stone Mason | \-2% build time per level, cap \-30% at L15 | 200 | 100 | 400 | 100 | 10m |

**Resource production formula**

production(L) \= base\_rate × L  (linear-additive, units per hour)

| Building | L1 | L5 | L10 | L15 | L20 |
| :---- | :---- | :---- | :---- | :---- | :---- |
| Gold Mint | 30/h | 150/h | 300/h | 450/h | 600/h |
| Lumber Camp | 40/h | 200/h | 400/h | 600/h | 800/h |
| Quarry | 25/h | 125/h | 250/h | 375/h | 500/h |
| Iron Mine | 30/h | 150/h | 300/h | 450/h | 600/h |

Production stops when the resource hits the Warehouse cap.

**Storage cap formula (Warehouse)**

cap(L) \= 5,000 \+ 2,500 × L^2  (per resource, applied independently)

| Level | Cap per resource |
| :---- | :---- |
| 1 | 7,500 |
| 5 | 67,500 |
| 10 | 255,000 |
| 15 | 567,500 |
| 20 | 1,005,000 |

**Stone Mason mechanic**

- Discount: `time_multiplier = 1 - (0.02 × L)`, capped at \-30% at L15  
- Applies to in-progress and queued building upgrades, retroactive recalculation on Stone Mason completion  
- Does NOT affect unit training queues  
- Does NOT affect Wonder construction (per Section 14, fixed 100 HP/h)  
- Levels 16-20 retain value as Wonder prerequisite signaling

**Cancel refund**

- Cancelling an in-progress upgrade refunds 75% of resources  
- Elapsed time is lost (no time refund)  
- Build queue slot freed immediately

**Sanity-check curves**

Quarry costs at base 80 Stone, 1.75x:

- L1: 80, L5: 750, L10: 16,400, L15: 360,000, L20: 7,900,000

Quarry build times at 2m base, 1.55x, 24h cap:

- L1: 2m, L5: 11m, L10: 2h, L13: 8h, L15: 19h, L16-20: 24h (capped)

Cumulative L1-L15 build time ≈ 50 hours. Across a 2-3 week round with one queue slot (Town Hall L10 unlocks a second), a focused player gets one building to L15 and several to L10. Curve opens up mid-round.

### Reasoning

- **Cost 1.75x, time 1.55x**: time scales slower than cost so late-game upgrades are gated by economy (raid, capture nodes) rather than wall-clock. Matches the conquest spine.  
- **24h time cap**: preserves the "queue before bed, check tomorrow" async pattern. No upgrade strands a player offline for days.  
- **Linear production vs exponential cost**: ROI per level decelerates. Pushes players past L10-12 to pivot from levels to territory. Captured nodes (\~+120-500/h flat per 16.5) become competitive with high Quarry levels.  
- **Stone production deliberately lower**: keeps Wonder Stone bottleneck (2.4M per 16.2) economically meaningful and makes Quarry node capture matter all round.  
- **Quadratic Warehouse**: scales fast enough to hold Wonder milestone payments (80k-240k Stone) without dwarfing exponential building costs. L1 starter cap of 7,500 forces early check-ins (\~10h to fill at L5 production), reinforcing 1-10 sessions/day pattern.  
- **Warehouse as raid target**: higher cap \= bigger loot pool. Real tradeoff between stockpiling and exposing.  
- **Stone Mason retroactive**: turns it into a strategic catch-up tool, not just future pacing. Cap at \-30% prevents trivializing the 24h time cap.  
- **75% cancel refund**: typo recovery and strategy pivots stay non-punishing. Time loss is the real penalty, prevents queue-cycling exploits.

### Alternatives considered

- **Pure exponential time, no cap**: L20 builds reach 70+ days, breaks the round  
- **Piecewise three-tier curve (early fast, late slow)**: more tunable but more code, deferred unless playtest reveals problems  
- **Exponential production (1.2x/L)**: matches cost curve, kills node value, snowballs leaders  
- **Quadratic production (L²)**: too forgiving, removes raid pressure  
- **Logarithmic production**: flatlines too early, high levels feel dead beyond unlock gates  
- **Linear Warehouse cap**: mid-game becomes mandatory daily upgrade  
- **Exponential Warehouse cap**: late-game caps balloon to 100M+, raids economically pointless  
- **Multiple Warehouses**: violates single-slot principle and 12-building cap  
- **50% cancel refund**: too punishing for typo recovery  
- **0% cancel refund**: hostile to async play and fat-finger errors  
- **Stone Mason affects Wonder**: breaks Section 14 fixed-rate construction balance  
- **Stone Mason non-retroactive**: reduces tactical value, mostly future-pacing tool

### Open follow-ups

- Whether Town Hall \+1 queue unlock at L10/L20 needs a separate balance pass (likely yes, deferred to playtest)  
- Whether production should have small variance (±5%) to feel "alive" (leaning no, breaks predictability)  
- Whether starter kingdom gets a flat \-50% discount on first L1→L2 of each Resource building (per Section 13 onboarding intent)  
- Whether Stone Mason should have diminishing-returns curve instead of flat 2%/L (current flat curve gives clean \-30% cap, simpler)  
- Production behavior when Warehouse cap is hit: hard stop vs trickle. Current decision: hard stop

---

## 16.5 Map Generation

How regions, adjacencies, nodes, spawns, and wilderness garrisons are generated for each round. Procedural, seeded, scales with player count.

### Committed

**Generation model**

- Procedural generation per round from a logged seed  
- Seed displayed in `world info` output, enables rematch rounds on identical maps  
- Region names drawn from a curated pool of \~200 medieval-fantasy place names (Ironwood, Pale Coast, Dragon's Tooth Pass, Mistmarsh, etc.)  
- Adjacency graph generated as a planar graph with average degree 2.8-3.5, no isolated regions

**Map size**

regions \= clamp(2.5 × players \+ 6, 16, 64\)

| Players | Regions | Kingdoms | Wilderness |
| :---- | :---- | :---- | :---- |
| 4 | 16 | 4 | 12 |
| 8 | 26 | 8 | 18 |
| 12 | 36 | 12 | 24 |
| 16 | 46 | 16 | 30 |
| 20 | 56 | 20 | 36 |
| 24+ | 64 (cap) | 24 | 40 |

Floor of 16 prevents claustrophobic small rounds. Cap of 64 preserves terminal readability of `map` output.

**Node placement**

total\_nodes \= round(1.2 × players)

Quality tier distribution:

| Tier | Share | Bonus production |
| :---- | :---- | :---- |
| Rich | 20% | \+500/h |
| Standard | 50% | \+250/h |
| Poor | 30% | \+120/h |

Resource type distribution:

| Resource | Share |
| :---- | :---- |
| Stone | 35% |
| Iron | 25% |
| Wood | 20% |
| Gold | 20% |

Stone overweighted to match Wonder demand (3x other resources per 16.2). Each region holds 0-2 nodes maximum. Rich nodes never share a region with another node.

**Spawn placement**

Poisson-disk algorithm on the region graph:

1. Generate region graph and compute pairwise graph distance  
2. Reserve `ceil(players × 1.5)` spawn-eligible regions for grace-window absorption  
3. Place T0 players using max-min spacing (each new spawn maximizes minimum graph distance to existing spawns)  
4. Grace-window joiners (T0 to T0+72h per Section 13\) take next-best reserved slot  
5. Each spawn region gets a Home Hoard node: Standard tier, matched to that kingdom's weakest production type

Constraints:

- Minimum 2 hops between any two kingdoms  
- No kingdom spawns adjacent to a Rich node  
- No kingdom spawns on a hub region (5+ adjacencies)

Further spawn constraints (terrain, degree, wilderness adjacency) layer on in 16.8 and 16.10.

**Wilderness**

- No hostile mobs, no PvE  
- Each unclaimed node has a token garrison that must be defeated to claim it:

| Tier | Garrison |
| :---- | :---- |
| Poor | 15 Levy, 5 Archer |
| Standard | 25 Levy, 10 Archer, 5 Pikeman |
| Rich | 40 Levy, 20 Archer, 15 Pikeman, 5 Knight |

Garrisons are static, do not respawn after defeat, do not pursue. One-time speedbump per node.

### Reasoning

- Procedural with seed scales to "every company runs rounds" without staffing burden, while seeded reproducibility supports rematch culture and bug reproduction  
- 2.5 regions per player keeps neighbor density meaningful: every player has 1-3 close rivals, the foundation of office drama  
- Wilderness ratio (62-75%) shrinks slightly with player count so contested ground grows with population  
- 1.2 nodes per player makes the Wonder 3-node prerequisite contested but achievable (must fight for at least one)  
- Stone weighting at 35% prevents the Wonder Stone bottleneck from being unfightable: roughly 1 Stone node per 2.4 players ensures multiple credible Wonder builders  
- Poisson-disk spawn prevents the "surrounded ragequit" failure mode that random placement produces in \~30% of rounds  
- Home Hoard gives every kingdom a baseline economic anchor without requiring early aggression, supporting late grace-window joiners  
- No mobs keeps the design lean and focused on the team-building goal: players should fear coworkers, not bandits  
- Static node garrisons gate early land grabs without introducing a respawning PvE system

### Alternatives considered

- **Hand-crafted map pool (\~20 maps)**: more polish per map, but staffing burden and predictable meta as players memorize layouts  
- **Pure random no seed**: loses rematch and debugging value  
- **Player-drafted maps**: scope creep, deferred indefinitely  
- **Linear size formula `3 × players`**: 24-player rounds hit 72 regions, terminal output suffers  
- **Fixed 30 regions**: small rounds feel empty, large rounds feel cramped  
- **Equal resource split across nodes**: Stone too scarce relative to Wonder demand, breaks 16.2 economy  
- **Flat single-tier nodes**: removes "the Rich Quarry in Dragon's Tooth" as a persistent talking point  
- **Random spawn placement**: 30% of rounds produce a surrounded player, ragequit risk  
- **Symmetric/rotational spawn**: only works for specific player counts, breaks 72h grace window  
- **Player-chosen spawn regions**: meta-gameable, first picker dominates  
- **Full PvE wilderness with bandits**: scope creep, hostile to async absence tolerance  
- **Roaming mob armies**: punishes offline players, conflicts with moderate absence tolerance  
- **No node garrisons (free claims)**: trivial early land grabs, removes Catapult-gated capture intent

### Open follow-ups

- Curated name pool: source list (lean on public-domain medieval atlases) and whether players can submit names per company  
- Whether seed is exposed at round start or only after round end (current: visible from start)  
- Whether the planar graph generator should bias toward choke-point structures (peninsulas, narrow corridors) or stay uniform  
- Whether Rich node placement should cluster (creating contested zones) or spread (creating multiple flashpoints), current default: spread  
- ~~Whether Home Hoard nodes can be lost permanently or reset to neutral if the kingdom is heavily raided~~ **Resolved**: a Home Hoard is permanently reserved for its home kingdom. Only that kingdom may capture it (in any state); every other kingdom's capture attempt is rejected (`home_hoard_protected`). It can never be seized — the kingdom's home base survives the round (§7). Rivals can still raid the home region's stockpile via `attack`, but never take the node.  
- Garrison scaling if playtest shows Rich nodes are too easy to take with a Knight rush  
- Whether to expose map generation parameters as world creation options (configurable density, size override) or keep formula fixed for v1

---

## 16.6 Round-Over and Reset

What happens at the exact moment a Wonder survives Consecration: state freeze, archive, profile carry-over, and how the next round starts.

### Committed

**Winning moment**

- Consecration timer reaches zero with Wonder HP above 0: round ends instantly  
- All armies halt mid-march, all build and training queues freeze, world enters read-only archive mode  
- No wind-down period, no grace window for in-flight attacks  
- World-wide announcement fires simultaneously: `>>> [Player]'s [Wonder Name] has stood. The round is over. <<<`

**Cooldown**

- No mandatory cooldown between rounds  
- Next round can be proposed immediately after the previous one ends  
- Cooldown periods deferred to post-v1

**Next round trigger**

- Manual proposal only: organizer must explicitly start the flow (matching Section 13 world creation model)  
- No auto-rollover, no auto-invite  
- Previous round participants receive a notification that a new round is available to join (delivery channel deferred per 17.3)

**Carry-over**

- No gameplay carry-over: resources, buildings, armies, nodes all reset to zero  
- Persistent player profile carries forward indefinitely (full stat surface in 17.4):  
  - Rounds played  
  - Rounds won  
  - Wonders completed  
  - Wonders destroyed  
  - Total raids launched and defended  
  - Peak nodes controlled in a single round  
- Per-server hall of fame: see 17.4 for leaderboard structure  
- Profile visible via `player show <handle>` and `world hall-of-fame`  
- Cosmetic carry-over: titles per 17.4, further cosmetics in 19.4

**Round archive**

- Full round state stored permanently per server  
- Accessible via `world history <round_id>` after round ends  
- Archive shows: final map state, final resource standings, Wonder HP at win moment, full battle log index, node ownership timeline  
- Storage cost accepted as a product obligation. Mitigation (compression, cold storage) is an infrastructure concern, see 17.5

### Reasoning

- Instant freeze removes ambiguity about what counts as a valid final attack. The defender won the Consecration, period.  
- No cooldown respects team momentum: an engaged office that just finished a round should be able to restart the same day.  
- Manual proposal keeps human intent in the loop and prevents zombie rounds where nobody is actually engaged (matching the Section 13 company-scheduled model).  
- Persistent profiles with no gameplay advantage preserve the full-reset fairness while giving winners something tangible beyond a one-day Slack moment.  
- Hall of fame creates a server-level social artifact that outlasts any single round, feeding the team-building goal across months of play.  
- Permanent archive supports the "office debrief" use case without an arbitrary expiry. Players should be able to revisit the round that ended three months ago.

### Alternatives considered

- **Wind-down period (1h)**: rejected. Clean ending is unambiguous. In-flight attacks failing is part of losing.  
- **Mandatory cooldown**: deferred to post-v1, not rejected on principle.  
- **Auto-rollover**: rejected. Zombie rounds with disengaged players undermine the team-building goal.  
- **No carry-over at all**: rejected. Hall of fame is the social reward for winning. Without it the round ends with no lasting artifact.  
- **30-day archive expiry**: rejected in favor of permanent storage. The debrief value doesn't expire.

### Open follow-ups

- Whether the hall of fame is per-world or per-server aggregate (current per 17.4: per-server aggregate)  
- Whether profile stats include per-round breakdowns or lifetime totals only (current: lifetime totals, per-round history via archive)  
- Cooldown configuration options when revisited post-v1  
- Whether `world history` output is paginated or summarized by default given permanent retention

---

## 16.7 Multi-World Membership

How a single email plays across multiple servers and worlds, and how server admins configure access and concurrency limits.

### Committed

**Identity model**

- Email address is the identity unit  
- A player logs in with an email and gains access to all servers that email is a member of  
- The same email can be a member of multiple servers independently  
- Each server membership is fully scoped: limits, stats, hall-of-fame, and world memberships on Server A have no bearing on Server B  
- A player with two emails (e.g. two employers) has two independent identities, one per email, with no cross-contamination

**Server model**

- A server is owned and configured by a company or any organizing entity  
- Server admins configure two independent limits:  
  - `max_concurrent_worlds`: how many worlds can run simultaneously on this server (0 to unlimited)  
  - `max_worlds_per_account`: how many worlds a single account can be active in at the same time on this server (0 to unlimited)  
- Both limits default to 2, operator-configurable at any time  
- Limit of 0 disables the capability entirely (e.g. `max_concurrent_worlds: 0` freezes world creation)  
- Limit changes apply at join time, not retroactively to existing memberships

**Join access model**

- Server admins configure access via domain whitelist, explicit invite list, or both (union)  
- Domain whitelist: any email matching a configured pattern (e.g. `@acme.com`, `*@*.acme.com`) can join  
- Invite list: explicit email addresses granted access regardless of domain  
- Union model: an email is admitted if it matches either the whitelist or the invite list  
- This covers "open to whole company" and "invite a contractor with a personal email" without separate mechanisms

**Stats and identity scope**

- Player profile, round history, and hall-of-fame are scoped to the server  
- No global profile aggregating across servers  
- A player active on two servers has two independent profiles, each visible only within that server via `player show` and `world hall-of-fame`

**Commands**

- `world list` shows all worlds the player is currently a member of on the active server  
- `world switch <world_id>` sets the active world context for the session  
- `server list` shows all servers the logged-in email has membership on  
- `server switch <server_id>` switches the active server context, then drops to that server's world list  
- REPL login banner shows active server, active world, and flags urgent state in other joined worlds on the same server

### Reasoning

- Email as identity unit matches workplace reality: work email is already the authentication anchor for SSO, Slack, and company tooling  
- Server-scoped identity prevents cross-company stat pollution and keeps hall-of-fame meaningful within the team-building context  
- Same email on multiple servers covers legitimate cases: a consultant working with two clients, a player joining a friend-group server alongside their work server  
- Union access model (domain \+ invite) gives admins full flexibility without two separate configuration surfaces  
- Server-level limits (not global) respect that each company has different engagement expectations and org sizes  
- No retroactive limit enforcement avoids punishing existing players when an admin tightens settings

### Alternatives considered

- **Global account with cross-server profile**: rejected. Pollutes company hall-of-fame with activity from unrelated servers, breaks the coworker-bound team-building goal  
- **One email per server, no multi-server**: rejected. Blocks legitimate multi-server use cases (consultants, friend groups)  
- **Domain whitelist only, no invite list**: rejected. Blocks contractors and guests with non-matching emails  
- **Invite only, no domain whitelist**: rejected. Forces manual invite for every employee, operationally hostile for large companies  
- **Global world limit (not per-server)**: rejected. A limit set for a small team server should not constrain a large company server running the same email

### Open follow-ups

- Whether `server switch` is sticky across sessions or resets to a default server on next login  
- Whether a player can set a default server and default world per server  
- Whether server admins can see which accounts are active across how many worlds (capacity monitoring)  
- Whether the REPL banner shows urgent state from non-active worlds on the current server only, or across all servers (recommend current server only to avoid noise)  
- Whether an account hitting `max_worlds_per_account` gets a clear error with current membership listed, or a generic rejection  
- Authentication mechanism (SSO, API key, password): committed in 17.1

---

## 16.8 Spawn Placement and Balancing

How kingdoms are positioned on the map at T0 and how grace-window joiners are absorbed without breaking fairness or creating timing metagames.

### Committed

**Spawn region constraints**

Every kingdom spawn region must satisfy all of the following (layered on top of 16.5 Poisson-disk rules):

- Minimum 2 hops from any other kingdom (from 16.5)  
- Not adjacent to a Rich node (from 16.5)  
- Not a hub region (degree 5+) (from 16.5)  
- At least 2 adjacent wilderness regions at spawn time  
- Adjacency degree 2-4 (not isolated, not over-connected)

Terrain constraints layer on in 16.10 (Plains or Hills only).

If the generator cannot satisfy all constraints after N attempts (recommend N=50), it relaxes the degree constraint first (allow degree 5), then the wilderness adjacency constraint (allow 1 adjacent wilderness), then logs a warning. The 2-hop, Rich-node, and terrain exclusions are never relaxed.

**Late-joiner slot assignment**

- At world creation, `ceil(players × 1.5)` spawn-eligible regions are reserved  
- T0 players are placed first using max-min Poisson-disk spacing  
- Grace-window joiners (T0 to T0+72h) are assigned randomly among remaining reserved slots at join time  
- No join-time advantage: a player joining at T0+1h gets the same expected slot quality as one joining at T0+71h  
- Reserved slots not consumed by the grace window are released to wilderness at T0+72h

**Late-joiner stockpile adjustment**

- Grace-window joiners receive a one-time stockpile bonus at spawn:  
  - `bonus = floor(hours_since_T0 / 12) × 1000` per resource  
  - Cap: \+4000 per resource (reached at 48h elapsed)  
- Bonus is added on top of the standard 500-per-resource starter stockpile (Section 13\)  
- No building level adjustment, no military adjustment  
- Bonus applies only once at spawn, does not compound

**Example**

| Join time | Hours elapsed | Bonus per resource | Total starter stockpile |
| :---- | :---- | :---- | :---- |
| T0 | 0 | \+0 | 500 |
| T0+12h | 12 | \+1000 | 1500 |
| T0+24h | 24 | \+2000 | 2500 |
| T0+48h | 48 | \+4000 | 4500 |
| T0+72h | 72 | \+4000 (cap) | 4500 |

**Premium spawn slots**

- The generator internally labels regions by spawn desirability during graph construction  
- Labels are used to enforce constraints and seed Poisson-disk placement  
- Labels are never exposed to players, admins, or any command output  
- All valid spawn regions are presented equivalently in the game world

### Reasoning

- Random late-joiner assignment removes any incentive to coordinate join timing in Slack, which would create a metagame outside the intended social layer and disadvantage players in different timezones  
- Stockpile bump closes the economic gap without compounding: a building level advantage persists all round, a resource bonus is spent and gone within the first session  
- Cap at \+4000 (48h equivalent) reflects that a 72h joiner is materially behind in build queue time regardless of resources, and over-compensating with resources does not fix that gap  
- Degree 2-4 constraint prevents dead-end spawns (degree 1-2 leaves no expansion options) and over-connected spawns (degree 5+ makes early defense impossible)  
- Guaranteed 2 adjacent wilderness regions ensures every kingdom has at least 2 viable early expansion targets before hitting a neighbor  
- Internal-only premium labels avoid organizer politics and first-mover metagaming in a workplace context where visible spawn quality would immediately become a Slack argument  
- Relaxation order (degree first, then wilderness adjacency) preserves the economically critical constraints (Rich node exclusion, hop distance) at the cost of the structural ones, which are quality-of-life rather than fairness guarantees

### Alternatives considered

- **First-come best-available**: rewards fast joiners, but in a workplace context fast joining correlates with timezone and notification timing, not engagement  
- **Player-chosen slot**: adds agency but is exploitable (pick adjacent to the weakest neighbor), and creates a lobby-waiting metagame  
- **Building level boost for late joiners**: permanent structural advantage that compounds all round, harder to balance than a one-time stockpile  
- **Time-scaled resource boost with no cap**: a T0+70h joiner could arrive with a massive stockpile, potentially more than a T0 player has accumulated, which inverts the intended disadvantage  
- **No adjustment**: a 72h joiner at L1 production with 500 resources is effectively eliminated before their first build completes, which conflicts with the 72h grace window's intent to absorb missed notifications without permanent exclusion  
- **Exposed premium slots**: immediately becomes an organizer-politics vector in workplace rounds

### Open follow-ups

- Whether the relaxation fallback should fire a visible warning to the world organizer (recommend yes, via round creation log)  
- Whether reserved slots that expire at T0+72h should be seeded with a neutral token garrison or left as plain wilderness  
- Whether the stockpile cap (currently flat \+4000) should scale with round age at the time late joiners arrive, in case a round reaches day 10+ before someone joins (currently outside scope since grace window closes at T0+72h, but worth flagging)  
- Whether the degree constraint should be checked dynamically (after each kingdom is placed) or statically (before T0), in case early-round conquests change region ownership in ways that affect effective expansion options

---

## 16.9 Fog of War

Fog of war is designed in v1 and ships in v1.1. This section locks the v1.1 visibility model, scouting mechanics, and attribution rules so implementation is a build task, not a redesign task. v1 launches with full visibility per Section 8\.

### Committed

**Scope**

- Fog of war is designed in v1, shipped in v1.1  
- v1 launches with full visibility per Section 8  
- This section locks the design so v1.1 implementation is a build task, not a redesign task  
- Section 8 will be updated post-v1 to reference this section when fog ships

**Visibility model (medium fog)**

Always visible to all players:

- Full map structure: region names, adjacency graph, region count  
- Kingdom locations: which region each player spawned in  
- Node locations and quality tier (Rich, Standard, Poor) and resource type  
- Region ownership of regions adjacent to the player's own kingdom or owned regions  
- Your own kingdom state: stockpiles, buildings, armies, queues, incoming attacks with ETA and rough size  
- World announcements: Wonder Foundation, milestones, Consecration, round-end events  
- Hall of fame, player profiles (rounds played, won, lifetime stats per 17.4)

Hidden by default, revealed only by scouting or passive Watchtower:

- Other players' stockpiles and per-resource production rates  
- Other players' building levels and queues  
- Other players' army composition (location, count by unit type)  
- Other players' training queues  
- Node ownership of nodes outside the player's adjacency  
- Incoming attack composition (size shown as bucket, exact unit breakdown hidden until arrival)

**Incoming attack size buckets**

When an attack is detected (via Watchtower ETA visibility), the defender sees a bucket label, not exact numbers:

| Bucket | Total unit count |
| :---- | :---- |
| Small | 1-99 |
| Medium | 100-499 |
| Large | 500-1999 |
| Massive | 2000+ |

Exact composition revealed at battle resolution in the battle log. Scout missions on the attacker's origin region can reveal exact composition before arrival.

**Scouting: hybrid Watchtower \+ Scout unit**

Watchtower passive intel (no Scout dispatch needed):

| Watchtower level | Passive intel on adjacent regions |
| :---- | :---- |
| 1-4 | Region ownership only |
| 5-9 | Ownership \+ visible army presence as bucket (Small/Medium/Large/Massive garrison) |
| 10-14 | Above \+ node ownership in adjacent regions |
| 15-20 | Above \+ stockpile bucket of adjacent kingdoms (Low/Mid/High/Stockpiled) |

Stockpile buckets (per resource, applied independently):

| Bucket | Range |
| :---- | :---- |
| Low | 0 to 10% of cap |
| Mid | 10-50% of cap |
| High | 50-90% of cap |
| Stockpiled | 90%+ of cap |

Scout unit active intel:

- Dispatch Scouts to any region via `scout <region> with <count>`  
- Travel time per 16.3 Scout speed (2.0 regions/hour)  
- On arrival, Scouts attempt to gather intel and return home (round trip)  
- Successful report delivered when Scouts return to home region

**Scout report contents (on success)**

If the target is a kingdom region:

- Stockpile exact values per resource at moment of scout arrival  
- Building list with exact levels  
- Visible army composition with exact unit counts  
- Army movements in and out of region over the last 24 hours (count and direction, attribution per anonymous-detection rules below)

If the target is a wilderness or node region:

- Node ownership and garrison composition (exact)  
- Any armies transiting the region in the last 24 hours

Report does NOT include:

- Build queue or training queue contents  
- Incoming attacks on the scouted player  
- Scout missions the target has dispatched

**Scout interception**

- Target region has a passive scout-detection threshold based on garrison  
- If target garrison contains 10+ Scouts OR 20+ Archers OR any Watchtower level 5+, incoming Scouts are detected  
- Detected Scouts are killed (full loss to attacker)  
- No report is returned  
- Target receives an anonymous notification: "Hostile scouts were intercepted in \[region\]" with no attribution  
- Detection is binary: either all scouts return with a report, or all die  
- Scout missions sent in stacks below 10 units have a 50% chance to slip past detection regardless of garrison (small probes can sometimes succeed)

**Attribution model (anonymous detection)**

- Defender sees scout interception alerts without attacker identity  
- Defender sees attack ETA and size bucket without attacker identity until armies arrive at home region  
- Attribution revealed at battle resolution in the battle log  
- Caravan interceptions remain attributed per Section 12 (interceptor and sender both known)  
- World announcements (Wonder phases, round-end) remain fully attributed per Section 14

**Wonder visibility under fog**

Wonders are fully public regardless of fog state:

- Foundation triggers world announcement naming builder, Wonder choice, and region  
- Construction progress visible to all players via `wonder list` and target inspection  
- Current HP visible to all  
- Milestone payments and Consecration phase trigger world announcements  
- Wonder garrison composition follows standard fog rules (must scout to see exact garrison)

Fog does not protect the builder. Section 14's "office drama" design requires public construction.

**Commands**

- `scout <region> with <count>` dispatches Scouts on a round-trip intel mission  
- `scout reports` lists pending and completed scout reports with timestamps  
- `scout report <id>` displays a specific report  
- `watchtower` displays current passive intel visible to the player based on Watchtower level  
- `intel <player>` displays everything the player currently knows about a target (Watchtower passive \+ cached scout reports), with timestamps

**Intel staleness**

- Scout reports are snapshots, never auto-refresh  
- Reports display the timestamp of the scout arrival, not the time of viewing  
- Watchtower passive intel is always live (current state of adjacent regions)  
- Stale reports are not auto-deleted (let the player decide what's still useful)

### Reasoning

- Medium fog preserves the team-building social layer: every player still knows who their neighbors are and can coordinate in Slack about "the Bob situation" without needing to scout to identify Bob  
- Hiding stockpiles and exact army composition restores the Ogame-style "is he bluffing or loaded" tension that full visibility removes, generating better office conversation than knowing exact numbers  
- Hybrid Watchtower/Scout model gives Watchtower a meaningful job beyond ETA warning, which 16.4 flagged as underweight  
- Scout unit (already costed and statted in 16.3) gets a dedicated purpose, justifying its existence in the unit roster  
- Size buckets on incoming attacks let defenders make decisions (call for help via Slack? recall armies?) without trivializing the attacker's investment in stack composition  
- 24-hour activity window in scout reports lets a coordinated coalition reconstruct enemy movement patterns over several scouts without giving real-time tracking  
- Anonymous detection on scout interception creates paranoia ("someone's watching me") without enabling targeted retaliation, which would suppress scouting and starve the intel economy  
- Binary interception (all-or-nothing) is predictable and explainable, avoids RNG salt  
- 50% slip-through for sub-10 scout stacks gives small probes a real role and prevents Archer-heavy garrisons from being intel-proof  
- Wonder publicity is non-negotiable per Section 14, codified here to prevent future fog expansions from eroding the round finale

### Alternatives considered

- **Light fog**: hides too little, doesn't change the strategic landscape enough to justify the implementation cost  
- **Heavy fog**: breaks the social layer by hiding kingdom locations, making Slack coordination ("attack Bob") require an in-game discovery phase  
- **Asymmetric fog (combat hidden but geography visible)**: coherent alternative, but committed model already achieves this by keeping ownership visible while hiding combat-relevant data  
- **Scout-only intel**: leaves Watchtower as a single-purpose building, wastes design surface  
- **Watchtower-only intel**: removes the Scout unit's reason to exist, kills active intel-gathering as a play pattern  
- **Library-gated intel**: makes intel a passive level-up reward rather than an active decision, weakens the agency around scouting  
- **Full intel reports including build queues**: too powerful, lets coalitions perfectly time strikes against half-built defenses  
- **Probabilistic fuzzy reports**: adds RNG without adding drama, frustrates planning  
- **Stealth roll interception**: RNG-driven failure with no clear feedback, creates "I lost 50 Scouts for nothing" salt  
- **Dedicated anti-scout building**: scope creep, breaks the 12-building cap  
- **Full attribution on detection**: enables defenders to preemptively counter-raid scouters, suppresses intel-gathering  
- **No detection at all**: removes counter-intel as a strategic surface, makes Archer/Scout defense pointless  
- **Wonder hidden under fog**: directly conflicts with Section 14 design intent

### Open follow-ups

- Whether scout reports decay in any way (currently never decay, never auto-delete) or get a "stale" visual marker after 24h  
- Whether Trebuchet/Catapult armies are large enough to leak silhouette info (e.g. "a slow-moving Massive army" implies siege weapons) and whether this should be exposed in size buckets explicitly  
- Whether caravans show up in scout activity reports (recommend yes, since interception is already a known mechanic per Section 12\)  
- Whether passive Watchtower stockpile-bucket intel (level 15+) should also reveal production rate buckets  
- Whether scout reports can be shared between players via an in-game mechanism or only via Slack/screenshots (current implicit answer per Section 12: out-of-game only)  
- Tab-completion behavior for `scout <region>`: should it autocomplete all regions, only regions within Scout march range, or only regions adjacent to player-owned regions  
- Whether a Scout unit count cap exists per mission (currently unbounded)  
- Interaction with Wonder Consecration: should scouting a Wonder-builder's kingdom during Consecration return special intel (garrison composition for the final assault), or follow standard rules (standard rules recommended for simplicity)  
- Section 8 update: when v1.1 ships, the "Full visibility (no fog of war v1)" line in Section 8 needs replacement with a reference to this section

---

## 16.10 Terrain Effects

How regions take on geographic character. Terrain modifies march speed and combat only, never production or visibility, keeping the balancing surface manageable.

### Committed

**Scope**

Terrain affects two mechanics only: march speed and combat. No effect on production, visibility, scouting, or unit availability. Terrain types are fully visible to all players at all times (no fog interaction).

**Terrain types**

Five types, each with fixed march and combat modifiers:

| Terrain | March modifier | Combat effect | Identity |
| :---- | :---- | :---- | :---- |
| Plains | 1.0x | None | Default, neutral ground |
| Forest | 0.8x | \+10% defender Def | Ambush country |
| Hills | 0.9x | \+15% defender Def | Defensible high ground |
| Mountain | 0.6x | \+25% defender Def | Fortress terrain |
| Marsh | 0.5x | \-10% attacker Atk | Punishes attackers, no defender bonus |

**Stacking rules**

- Terrain combat modifier stacks additively on top of the 16.3 home-region defender bonus (+20% home, \+1%/Wall level capped at \+40%)  
- Separate cap: terrain combat modifier cannot exceed \+25% in any single battle  
- Marsh's \-10% attacker Atk applies before RPS multipliers  
- Mountain bonus applies to whoever defends the region, not only the home owner. A captured Mountain node defended by the new owner retains the \+25%.

**Distribution**

Biome clustering during map generation: adjacent regions share terrain with high probability, producing named geographic features (mountain ranges, forest belts, marshlands).

Target shares across the map:

| Terrain | Share |
| :---- | :---- |
| Plains | 40% |
| Forest | 20% |
| Hills | 20% |
| Mountain | 12% |
| Marsh | 8% |

Clustering is enforced by the same seeded procedural generator from 16.5. Seed reproduces identical terrain layout across rematches.

**Spawn constraint**

Kingdom spawn regions must be Plains or Hills only. This layers onto all 16.8 constraints (2-hop spacing, not adjacent to Rich node, not a hub, 2+ adjacent wilderness, degree 2-4). Mountain, Forest, and Marsh are never spawn-eligible.

If the generator cannot satisfy all spawn constraints including terrain after 50 attempts, relaxation order from 16.8 applies first (degree, then wilderness adjacency). Terrain restriction is never relaxed.

**Node-terrain thematic pairing**

Node placement biases toward thematic terrain:

| Resource | Preferred terrain | Bias |
| :---- | :---- | :---- |
| Iron | Mountain, Hills | 70% chance of preferred placement |
| Stone | Mountain, Hills | 70% |
| Wood | Forest | 70% |
| Gold | Plains, Hills | 70% |

The remaining 30% places randomly per 16.5 rules. No yield modification: a Standard Iron node produces \+250/h regardless of terrain. Pairing is geographic flavor, not a balancing mechanic.

**March speed calculation**

march\_time \= base\_distance\_time / ((mod\_origin \+ mod\_destination) / 2\)

Where `base_distance_time` follows 16.3 unit speed rules. Average of origin and destination modifiers, applied to the slowest unit in the army.

Knights and Scouts retain terrain-penalty immunity per 16.3. They march at full base speed regardless of terrain on either end.

**Caravans**

Caravans (per Section 12\) use the same march formula as armies. A caravan moving through Mountain terrain is slow and exposed, making interception more viable. Escort units determine effective speed (slowest unit rule).

**Worked example: Knight raid through Mountain**

Bob marches 100 Knights and 200 Levy from Pale Coast (Plains) to Ironwood (Hills), crossing one Mountain region:

- Pale Coast → Mountain region: avg mod \= (1.0 \+ 0.6) / 2 \= 0.8  
- Mountain → Ironwood: avg mod \= (0.6 \+ 0.9) / 2 \= 0.75  
- Slowest unit: Levy at 0.5 regions/hour base  
- Total march time: \~2h hop 1 \+ \~2h hop 2 \= \~4h  
- Combat at Ironwood: Marta gets \+20% home bonus \+ Wall bonus \+ 15% Hills \= within the \+25% terrain cap

If Bob marched Knights alone (terrain-immune), march time drops to \~2h total. Composition choice matters geographically.

### Reasoning

- Two-effect scope (march \+ combat) keeps balancing surface manageable while making geography decision-relevant  
- Five terrain types give enough variety for memorable map features (Spine of the North, Whispering Wood) without exploding the matrix  
- Biome clustering produces recognizable named features that become Slack-shareable, supporting team-building goals  
- Plains/Hills spawn constraint avoids starter unfairness (a Marsh spawn would face \-10% attacker Atk on every offensive move all round)  
- Separate \+25% terrain cap prevents stacking with Walls into an invulnerable Mountain fortress, preserving the conquest spine  
- Marsh as attacker-punishing rather than defender-buffing creates a tactical anomaly: defenders don't want to fight in marshland either, it just slows everyone and hurts attackers  
- Knight/Scout terrain immunity (already in 16.3) gives cavalry a real strategic identity beyond raw stats: they ignore the geographic friction that pins infantry  
- Thematic node pairing without yield modification gives intuitive geography without adding a balancing dimension  
- Average-of-endpoints march formula is computationally simple, intuitive, and produces sensible cross-terrain numbers  
- Public terrain visibility keeps the social layer informed: alliances form around "Bob is locked in by mountains" geography, not hidden discovery

### Alternatives considered

- **No terrain (status quo)**: rejected. Regions feel interchangeable, geography is just an adjacency graph  
- **Flavor-only labels**: rejected. Naming terrain without mechanical effect wastes the design surface  
- **Rich mechanical (production, visibility, scouting effects)**: rejected. Doubles balancing surface for marginal drama improvement  
- **Per-unit-type asymmetric effects**: rejected. Tempting (cavalry on plains, archers in forest) but multiplies the balance matrix beyond v1 tolerance  
- **3 terrain types**: rejected. Loses Marsh's anti-attacker identity and Hills as a middle-ground defensible terrain  
- **7 terrain types (add Coast, Desert)**: rejected. Coast implies water and naval, Desert has no clear identity distinct from Plains. Scope creep  
- **Uniform random distribution**: rejected. Produces noisy maps without recognizable features  
- **Hand-tuned terrain per seed**: rejected. Defeats procedural generation, adds staffing burden  
- **Mountain march at 0.4x**: rejected. Too punishing, makes Mountain regions effectively impassable for slow units  
- **Marsh as defender-buffing instead of attacker-punishing**: rejected. Makes Marsh kingdoms too defensive, removes the "everyone hates fighting here" identity  
- **Spawn allowed on any terrain with starter compensation**: rejected. Compensation rules for 5 terrain types add complexity, simpler to restrict spawns  
- **Terrain modifies node yield (Iron in Mountain produces \+10%)**: rejected. Adds a balancing dimension and makes thematic placement deterministic rather than flavor  
- **Destination-only or origin-only march modifier**: rejected. Average is more intuitive and avoids edge cases at terrain borders  
- **Worst-of-two march modifier**: rejected. Punitive, makes cross-terrain marches universally slow

### Open follow-ups

- Whether terrain effects should be exposed in `map <region>` output or only in `battle` and `march` previews (recommend exposing in both for clarity)  
- Whether Trebuchet/Catapult armies should have an additional terrain penalty beyond the slowest-unit rule (currently no, but a Trebuchet army through Mountain is already extremely slow given Trebuchet base speed 0.2)  
- Whether biome cluster size should scale with map size (current: fixed clustering aggressiveness regardless of region count)  
- Whether the \+25% terrain cap should be visible in pre-battle combat preview (recommend yes, transparency aids planning)  
- Interaction with Wonder defense: a Wonder in a Hills kingdom gets \+15% defender Def on the home region attack, stackable with Walls. Worth playtest validation for Consecration balance  
- Whether a future Watchtower interaction with Forest (forest reduces incoming-attack ETA visibility) is worth designing now or deferred to 16.9 v1.1  
- Whether captured Mountain nodes are worth a special command flag in `node show` given their inherent defensive value  
- Whether procedural clustering should occasionally produce a "fortress region" (Mountain surrounded by Hills) as an intentional contested feature, similar to how 16.5 flags Rich node placement strategies

---

## 16.11 Special Map Features

Static Ruins and recurring Weather windows. Ruins are one-time caches placed at map generation, Weather temporarily modifies terrain. Neither adds PvE threat or mutates the map mid-round.

### Committed

**Scope**

Two feature types: static one-time **Ruins** placed at map generation, and recurring **Weather windows** that temporarily modify terrain. No PvE threats beyond existing wilderness node garrisons (16.5). No map mutation during a round: adjacency, terrain, and region layout are fixed at T0.

### Ruins

**Ruins placement**

ruin\_count \= max(2, round(players / 4))

| Players | Ruins |
| :---- | :---- |
| 4 | 2 |
| 8 | 2 |
| 12 | 3 |
| 16 | 4 |
| 20 | 5 |
| 24 | 6 |

Constraints (layered on 16.5 and 16.10):

- Placed only in wilderness regions  
- Never in a region that contains a resource node  
- Minimum 2 hops from any T0 kingdom spawn  
- Minimum 2 hops between any two Ruins  
- Never on Mountain or Marsh terrain (keeps clear routes reachable, see 16.10 march modifiers)  
- Same seeded procedural generator as 16.5

**Ruin tiers**

Distribution across all Ruins on the map:

| Tier | Share | Garrison | Cache reward |
| :---- | :---- | :---- | :---- |
| Minor | 50% | 20 Levy, 10 Archer | 4000 G, 4000 W, 2000 S, 4000 I |
| Standard | 35% | 40 Levy, 20 Archer, 10 Pikeman | 10000 G, 10000 W, 6000 S, 10000 I |
| Major | 15% | 60 Levy, 30 Archer, 20 Pikeman, 10 Knight | 25000 G, 25000 W, 15000 S, 25000 I |

Garrisons are static, do not respawn, do not pursue (same rules as wilderness node garrisons).

**Ruin claim mechanics**

- Defeat the garrison via standard combat resolution (16.3)  
- Cache is granted immediately to the army that wins the battle, added to that player's home stockpile (instant transfer, no caravan needed)  
- Cache amounts respect the player's Warehouse cap. Excess is lost  
- Ruin is consumed: region reverts to plain wilderness, no longer appears in `map` output as a Ruin  
- Cleared Ruins do not regenerate during the round

**Ruin visibility**

- Ruin location, tier, garrison composition, and cache contents are fully visible in `map` and `map <region>` output from T0  
- Visibility unaffected by fog of war (16.9): Ruins are treated as fixed map features, not player state  
- Once claimed, the world announcement fires: `>>> [Player] has claimed the [Tier] Ruins in [Region]. <<<`

**Commands**

- `map` lists all Ruins with tier and region  
- `map <region>` shows Ruin tier, garrison, and cache contents if the region holds one  
- `ruins` lists all unclaimed Ruins on the map with tier, region, and approximate march time from home

### Weather windows

Recurring world events that temporarily modify terrain effects (16.10).

**Cadence**

- First weather window fires at T0 \+ 96h (day 4\)  
- Subsequent windows fire every 72-96h thereafter (randomized within range, seeded)  
- Round ends without further windows during the Consecration phase: once any Wonder enters Consecration, no new weather windows spawn  
- Active windows in progress when Consecration begins continue to their scheduled end

**Window structure**

Each window targets one terrain type and applies one modifier for 24 hours.

| Modifier | Effect | Telegraph |
| :---- | :---- | :---- |
| **Storms** | Target terrain march \-20%, defender Def \-10% on that terrain | "Storms gather over the \[terrain\]" |
| **Fair Weather** | Target terrain march \+20% | "Clear skies favor travel through the \[terrain\]" |
| **Fog** | Target terrain march \-10%, attacker Atk \-10% in battles on that terrain | "Fog rolls across the \[terrain\]" |

One modifier and one terrain per window. Terrain selection is weighted by map share (Plains and Forest hit more often than Marsh), modifier selection is uniform.

**Telegraph and timing**

- World announcement fires 12h before the window opens: `>>> Storms will reach the Mountain regions in 12 hours. Effects last 24 hours. <<<`  
- Second announcement at window open: `>>> Storms have broken over the Mountain regions. <<<`  
- Final announcement at window close: `>>> The storms have passed. <<<`  
- Announcements visible to all players regardless of fog

**Stacking with 16.10**

- Weather modifiers stack additively with terrain base modifiers  
- Combat modifiers respect the \+25% terrain combat cap from 16.10 (weather counts toward the cap)  
- March modifiers have no cap, but the slowest-unit rule still applies  
- Knight/Scout terrain immunity (16.3) is preserved: they ignore both base terrain and weather march penalties  
- Weather never inverts a terrain effect (e.g. Fair Weather on Mountain does not produce a march bonus above 1.0x, it caps at 1.0x)

**Worked example: Storms on Mountain**

Bob's 50 Knights \+ 100 Pikemen march through a Mountain region during a Mountain Storms window:

- Base Mountain march modifier: 0.6x  
- Storm modifier: \-20% additional  
- Effective Mountain modifier: 0.4x  
- Slowest unit: Pikeman at 0.4 regions/hour base  
- Cross-Mountain segment: \~3x slower than fair-weather Plains

If a battle resolves in Mountain during the same window:

- Defender Def: \+20% home \+ Wall bonus \+ 25% Mountain \- 10% Storm \= capped at \+25% terrain total  
- Storm Mountain still hits the cap, but combined with home and Walls the defender remains formidable  
- Storm's value here is the march penalty, not the combat hit

**Commands**

- `weather` shows current and announced upcoming windows with timestamps  
- Login banner includes weather alerts in the urgent state summary

### Reasoning

- **Ruins as one-time caches** give early-to-mid round objectives that any player can chase without depending on neighbor aggression. A 72h grace-window joiner can target a Minor Ruin as a viable catch-up play, complementing the 16.8 stockpile bump.  
- **Cache amounts** scale to mid-round economy: a Major Ruin (25k mixed) is roughly 2-3 days of L5 production. Meaningful but not round-deciding.  
- **Tier garrisons** scale to require committed forces, not first-day rushes. A Major Ruin's 10 Knights and 20 Pikemen demand at least Stables and Barracks investment, gating early grabs.  
- **No respawn** keeps the map state monotonically simplifying as the round progresses, matching the no-mutation commitment.  
- **Mountain/Marsh exclusion** prevents Ruins from being economically unreachable due to terrain penalties, especially relevant for slow Catapult-led claims.  
- **Weather as terrain-only modifier** stays inside the 16.10 design surface. No new mechanics to balance, just temporary deltas on existing ones.  
- **24h windows** match the async absence tolerance: a player offline for one workday will miss at most one window, and the 12h telegraph means they can pre-plan before logging off.  
- **72-96h cadence** produces roughly 5-9 weather events per 21-28 day round, enough to matter without becoming noise.  
- **No weather during Consecration** prevents the round finale from being decided by an RNG terrain modifier on the defender's home region. Consecration drama stays in player hands.  
- **Fair Weather cap at 1.0x** prevents weather from making bad terrain temporarily better than Plains, preserving Plains identity as the neutral default.

### Alternatives considered

- **Ruins respawn after 48h**: rejected. Creates an ongoing PvE chore loop, violates the "fear coworkers not bandits" principle from 16.5  
- **Ruin cache scales with claim time** (earlier \= bigger): rejected. Punishes grace-window joiners and rewards no-life T0 rushes  
- **Cache delivered via caravan from Ruin to home**: rejected. Interceptable caravans are interesting for trade, but a claim reward intercepted before delivery would feel terrible. Instant grant is cleaner  
- **Ruins placed on any terrain including Mountain**: rejected. A Mountain Major Ruin behind Storm windows would be functionally unclaimable  
- **Weather effects on production**: rejected. Production is deliberately predictable per 16.4, weather should not interfere  
- **Weather effects on visibility/scouting**: rejected. Conflicts with 16.9 fog rules, adds a new dimension to scout interception RNG  
- **Random weather, no telegraph**: rejected. Async absence tolerance demands warning windows for any event affecting movement decisions  
- **Weather windows that affect single regions, not whole terrain types**: rejected. Region-scoped weather is harder to communicate and feels arbitrary. Terrain-scoped weather produces recognizable patterns ("Storm season hits the mountains again")  
- **Weather as a permanent slow drift** (terrain modifiers shift over the round): rejected. Adds creeping complexity, conflicts with predictable strategy planning  
- **Continuous weather** (always one terrain affected): rejected. Removes the "telegraphed event" character that creates Slack moments

### Open follow-ups

- Whether Ruin tier distribution should weight toward Major in larger player counts (current: fixed 50/35/15 regardless of size)  
- Whether the cache reward should be partially refunded (e.g. 50%) if the claimant's Warehouse cap is exceeded, or fully lost (current: fully lost, encourages Warehouse investment)  
- Whether Ruin garrisons should include Trebuchets or Catapults at Major tier (current: no, keeps siege weapons as player-exclusive)  
- Whether weather modifiers should be exposed in pre-battle combat preview (recommend yes, transparency aids planning)  
- Whether weather windows interact with caravan interception odds (current: only via standard march speed, no special interception bonus)  
- Whether a "weather log" view of past windows is worth a command, or covered by `world history` in round archive  
- Whether weather distributions should favor terrain types that are more strategically relevant for the current round state (e.g. more Mountain weather if Stone is heavily contested), probably no, adds adaptive complexity  
- Whether Ruins that remain unclaimed at round end appear in the archive (recommend yes, for "the Major Ruin nobody touched" debrief moments)  
- Interaction with 16.9 fog when v1.1 ships: Ruin visibility should remain public regardless of fog state, codified here

---

# Part III — Platform

Section 17 covers everything outside the game loop itself: how players authenticate, how abuse is contained, what does and does not integrate with external tools, what carries across rounds, and how the system is built and run.

These decisions are not gameplay, but they shape the conditions under which gameplay can happen. Get them wrong and the gameplay never reaches the player.

---

# 17\. Account, Identity, and Infrastructure

## 17.1 Authentication and Identity

How players log in, what an identity is, how handles and real names work, and how server admins are constituted.

### Committed

**Authentication method**

- Hybrid magic link \+ long-lived API key  
- First login: user enters email in browser or CLI, receives magic link via email, clicks to authenticate  
- Magic link validates email ownership and issues a long-lived API key to the CLI  
- API key stored in `~/.config/dun/credentials` (chmod 600 by default)  
- Subsequent CLI sessions authenticate via the stored API key with no further interaction  
- Magic links expire after 15 minutes, single-use only

**Identity unit**

- Email address is the identity unit (per 16.7)  
- Email ownership verified at first login via magic link  
- Multiple emails per human supported, each is an independent identity with no cross-account merge  
- No password ever stored or transmitted

**Server association**

- Per 16.7: server admins configure access via domain whitelist \+ explicit invite list (union)  
- No auto-creation of servers from email domain  
- Users must be admitted to a server before joining any worlds on it  
- Server membership granted at first successful login matching the access rules

**Display name model**

- Each player has a handle and a real name  
- Handle is shown in battle reports, world announcements, scout reports, hall of fame, leaderboards, and all gameplay contexts  
- Real name is visible only in `player show <handle>` profile output  
- Both fields are per-server scoped per 16.7 (same email can have different handles on different servers)

**Handle constraints**

- 3-24 characters  
- Allowed: letters (a-z, A-Z), digits (0-9), underscore, hyphen — regex `^[A-Za-z0-9_-]{3,24}$`  
- No spaces  
- Case-preserved on display, case-insensitive for uniqueness checks ("IronFist" and "ironfist" collide)  
- Reserved handles blocked: `admin`, `system`, `dun`, `world`, `neutral`, `wilderness`, `server`, `anonymous`, `none`, `null`  
- Unique per server (not globally)

**Real name**

- Required at first login on each server  
- Free-form text, 1-60 characters, unicode permitted  
- Editable anytime via `player set-name "..."`  
- Visible only to server members via `player show`

**Handle change policy**

- Handle is set at first login on a server  
- Editable between rounds via `player set-handle <new>`  
- Locked during any active round the player is a member of  
- Handle changes do not affect persistent profile stats (keyed to email, not handle)  
- Past battle reports and archives retain the handle as it was at the time of the event

**Token lifetime**

- API key valid for 90 days from last use  
- Each successful authenticated request refreshes the expiry  
- Inactive keys expire and require a fresh magic link login  
- Users can list active keys via `dun auth list` and revoke via `dun auth revoke <key_id>`  
- Server admins can revoke any member's keys via the admin console  
- Key rotation is automatic on suspicious activity (trigger rules in 17.2)

**Admin model**

- Server creator is the initial admin  
- Admins can grant or revoke admin status to other server members  
- Server must always have at least one admin (cannot revoke the last admin)  
- Admin actions: configure access rules (domain whitelist, invite list), configure world limits (`max_concurrent_worlds`, `max_worlds_per_account`), revoke member keys, remove members, transfer ownership  
- Admin status is per-server only, no cross-server admin

**Commands**

- `dun login` initiates magic link flow, prompts for email  
- `dun logout` clears local credentials  
- `dun auth list` shows active API keys for the current account with last-used timestamps  
- `dun auth revoke <key_id>` revokes a specific key  
- `dun server list` shows all servers the email has access to  
- `dun server switch <server_id>` sets the active server  
- `player set-handle <new>` sets handle (between rounds only)  
- `player set-name <real_name>` sets or updates real name  
- `player show <handle>` shows handle, real name, and persistent profile stats

### Reasoning

- Magic link \+ API key matches developer CLI conventions (`gh`, `fly`, `stripe`), zero password management burden, and gives the terminal-native session model the design demands  
- 15-minute magic link expiry balances usability against email-account-compromise risk  
- 90-day rolling API key matches industry norms for CLI tools and respects the dev workflow (re-auth quarterly is tolerable, weekly is hostile)  
- Handle \+ real name split preserves classic MMO drama ("IronFist has destroyed the Sky Tower") while keeping the coworker accountability that makes team-building work  
- Per-server handle uniqueness (not global) matches the 16.7 server-scoped identity model and avoids contention across unrelated companies  
- Between-rounds-only handle changes prevent mid-round confusion in battle reports and Slack threads, while still allowing players to refresh their identity each round  
- Reserved handle list prevents impersonation and command parser ambiguity ("attack admin" must not be a valid target)  
- Real name as required-but-private gives admins and coworkers a recovery path when handles don't ring a bell, without exposing real identity in every gameplay surface  
- Single-admin-minimum prevents orphaned servers  
- Per-server admin scope respects the 16.7 principle that servers are sovereign

### Alternatives considered

- **Email \+ password**: rejected, hostile to developer workflow and adds password-reset infrastructure burden  
- **GitHub OAuth as primary**: rejected, GitHub email often does not match work email, breaks the domain whitelist model in 16.7  
- **SSO (Google Workspace, Okta) as primary**: deferred to v1.1 enhancement on top of magic link, excludes companies without SSO if made primary  
- **API key only with no web flow**: rejected, bootstrapping problem (how does the user get the first key)  
- **Real name as primary display**: rejected, removes the drama and fun of handles, makes battle reports read like HR memos  
- **Handle-only with no real name**: rejected, coworker recognition is the team-building anchor  
- **Globally unique handles**: rejected, conflicts with 16.7 server-scoped identity and creates artificial contention  
- **Mid-round handle changes**: rejected, breaks battle report continuity and out-of-game Slack coordination  
- **Short-lived tokens (1h with refresh)**: rejected, dev-hostile, refresh-token complexity not worth the marginal security gain for a workplace game  
- **Permanent tokens**: rejected, no revocation path on compromise without a separate mechanism  
- **Auto-create server from email domain**: rejected, invites squatting and abandoned servers, conflicts with 16.7 admin-driven server creation  
- **Flat admin model (all members admin)**: rejected, no workplace tool works this way, invites accidental destructive actions

### Open follow-ups

- Magic link email delivery provider (SES, Postmark, Resend, in-house) deferred to 17.5 tech architecture  
- SSO support as v1.1 enhancement, scope and provider list TBD  
- Whether API keys can be named/tagged by the user for multi-machine workflows (e.g. "laptop", "work-desktop")  
- Whether the admin console is a CLI surface (`dun admin ...`) or a web dashboard, deferred to 17.5  
- Profanity filter on handles and real names: in-scope for v1 or deferred to moderation (17.2)  
- Whether a player who leaves a server (or is removed) retains their handle reservation for future rejoins (recommend yes, tied to email)  
- Whether handle uniqueness collisions on join attempt prompt the user to pick another, or auto-suffix (recommend prompt)  
- Whether real name is shown to server admins separately (e.g. admin member list) regardless of `player show` visibility (recommend yes)  
- Whether magic link emails are rate-limited per address to prevent abuse (recommend yes, 5 per hour, deferred specifics to 17.2)  
- Email change flow: not supported in v1, deferred. A new email \= new identity, no migration path  
- Account deletion / GDPR right-to-erasure flow: committed in 17.4

---

## 17.2 Anti-Cheating and Griefing

Protections against behaviors that erode trust: multi-account abuse, resource collusion, harassment, scripted automation, power dynamics, and repeat-target griefing. The design leans on the workplace social layer (visibility, accountability, admin discretion) rather than mechanical enforcement, matching the project's team-building intent.

### Committed

**Multi-account detection (alts)**

Prevents a single human from running multiple kingdoms in the same world to gain unfair economic or military advantage.

- Primary defense: domain whitelist \+ invite list enforcement per 16.7 access model  
- Admin audit view (`dun admin audit`) surfaces clusters of accounts sharing IP or device fingerprint  
- Clusters are flags for admin review, not auto-bans  
- No automated action on multi-account suspicion  
- Personal-email contractors and consultants admitted via explicit invite list remain a legitimate use case, not flagged by default

**Anti-collusion: public trade ledger**

Prevents two or more players from secretly funneling resources to a single Wonder builder or dominant kingdom. The mechanism is visibility, not prohibition.

- All caravan transfers (sender, receiver, amount, resource, timestamp, origin region, destination region, status) recorded to a world-scoped public ledger  
- World-scoped visibility: all members of a world can see all trades within that world  
- No caps on send volume, no taxes, no friction  
- Interception events recorded with attacker identity per Section 12  
- Marketplace swaps recorded separately (no second party)  
- Ledger is permanent for the round, archived with the world per 16.6

**Commands**

CLI surface for querying the trade ledger.

- `trade ledger` shows recent trades, paginated, newest first  
- `trade ledger <player>` filters to caravans sent or received by that player  
- `trade ledger --since <duration>` filters by recency (e.g. `--since 24h`)  
- Output columns: timestamp, sender, receiver, resource, amount, status (in-transit, delivered, intercepted)

**Reporting and moderation**

In-game mechanism for players to escalate concerns (harassment, suspected cheating, rule violations) to server admins. Non-anonymous to fit workplace accountability norms.

- `report <player> <reason>` files a non-anonymous report to the server admin queue  
- Reason is free-form text, 1-500 characters  
- Reporter identity visible to admins  
- Reports scoped to the server, not the world (admin context matches 16.7 server-scoped admin)  
- `dun admin reports` lists open reports with reporter, target, reason, timestamp  
- `dun admin reports <id>` shows full report detail  
- Admin actions on a report: dismiss, warn (sends a message to target), suspend account (server-scoped), remove from server  
- All admin actions on reports are logged and visible to the reporter and target

**Rate limits**

Protects the game economy and API from scripted abuse, compromised credentials, and runaway clients. Tuned to be invisible during normal play.

- Per-account write-command limits: 60 writes/minute, 1000 writes/hour  
- Write commands: `attack`, `march`, `recall`, `build`, `train`, `send`, `scout`, `wonder start`, `wonder repair`, `wonder cancel`, all admin actions  
- Read commands unlimited (`status`, `map`, `world`, `player show`, `trade ledger`, `scout reports`, `intel`, `weather`, `wonder` inspection)  
- Exceeded limits return a structured error with retry-after seconds  
- Limits are per-account, applied across all sessions and devices for the same identity  
- Server admins can override limits per server (raise or lower), defaults shipped as above

**Workplace dynamics (manager/report)**

Addresses the specific risk that organizational power dynamics (a manager farming their direct reports, a senior IC bullying juniors) could turn the game into a hostile workplace incident. v1 defers mechanical intervention in favor of organizational and admin discretion.

- No in-game mechanism in v1  
- Workplace power dynamics handled out of game by HR, management, and admin discretion  
- Server admins responsible for advising on world composition (recommend managers and direct reports do not join the same world)  
- Deferred to post-v1 if playtest reveals real harm patterns

**Griefing: repeat-attack cap**

Prevents a single attacker from repeatedly farming the same target into ragequit. Configurable per server because tolerance for aggression varies by team culture.

- Configurable per server: `max_raids_per_target_per_24h` (default 3\)  
- Applied per attacker-target player pair, sliding 24-hour window  
- Counts successful attack arrivals, not dispatches (cancelled or recalled marches do not count)  
- Counts attacks on home region and on player-owned non-home regions  
- Wonder assaults (any attack on a Wonder-owning region during Foundation, Construction, or Consecration phases) are exempt from the cap  
- Caravan interceptions are exempt (target is a caravan, not a player home)  
- Exceeded cap returns error at dispatch: `Your banner is known too well in [region]. You cannot raid [player] again for [N] hours.`  
- Server admins can set the cap to 0 (no limit) or any positive integer

### Reasoning

Justifies the design choices against the project's core principles: team-building, lean design, workplace fit, and the conquest spine.

- **Domain enforcement plus audit view** leverages the 16.7 access model as the primary alt defense without inventing new infrastructure. Workplace context makes mass-alt creation socially expensive even when technically possible.  
- **Public trade ledger** turns collusion into a visible social act. Two coworkers funneling resources to one Wonder builder shows up in the ledger immediately, and the office handles it (Slack, side conversation, retaliation). Caps would be arbitrary and gameable via chains, the social layer is more effective.  
- **World-scoped ledger visibility** matches the social layer the trade is part of. Cross-world visibility would expose unrelated games and serves no purpose.  
- **Non-anonymous reports** fit workplace norms. False reports have social cost. Anonymity would feel out of place when everyone is in the same Slack.  
- **Server-scoped reports** match the 16.7 admin scope. World-scoped admins do not exist.  
- **Rate limits as guardrails, not balance levers**: 60/min is generous for human play and tight enough to block scripted abuse. 1000/hr prevents sustained automation. Admin override path exists for power users or unusual server profiles.  
- **Workplace dynamics deferred** acknowledges that the right intervention is organizational, not mechanical. A "do not target" tool risks legitimizing the dynamic it tries to mitigate. If playtest reveals harm patterns, revisit with real data.  
- **Per-server raid cap configuration** respects that different server cultures tolerate different aggression intensities. A competitive engineering org might set 10, a casual marketing team might set 1\. Default 3 balances coverage against late-round conquest momentum.  
- **Wonder assault exemption** preserves Section 14 intent: the round finale must be unfettered. Capping Wonder strikes would create a defender exploit where the builder waits out the cap.  
- **Successful-arrival counting** prevents griefers from spamming dispatch-and-recall to lock out legitimate attackers. The cost (march time, exposure) is paid only on real attacks.

### Alternatives considered

- **Auto-ban on multi-account detection**: rejected. False positives on shared office IPs and VPNs would generate more grief than the alts cause  
- **Trade volume caps**: rejected. Arbitrary numbers, gameable via chains, undermines the legitimate-alliance trade pattern  
- **Anonymous reporting**: rejected. Workplace context makes anonymity feel paranoid and removes accountability for false reports  
- **No rate limits**: rejected. API key compromise or scripted automation could trivially overwhelm the economy  
- **Per-IP rate limits**: rejected. Shared office IPs and VPNs would punish legitimate concurrent players  
- **Admin-configurable no-target pairs**: rejected for v1. Reintroduces a non-aggression mechanism that conflicts with Section 12's trade-only diplomacy stance. Revisit if playtest reveals harm  
- **Mutual opt-in non-aggression**: rejected. Player-driven pacts conflict directly with Section 12  
- **Hard global raid cap (not configurable)**: rejected. Server cultures vary, configurability matches the 16.7 operator-configurable pattern  
- **Cap on dispatched marches instead of arrivals**: rejected. Enables grief-by-dispatch (lock the cap with cancelled marches)  
- **Wonder assaults subject to cap**: rejected. Breaks Section 14 round finale

### Open follow-ups

- Whether the trade ledger should expose marketplace swap volume per player (currently logged separately, not in the main ledger view)  
- Whether reports filed during a round are visible to the target immediately or only after admin action (recommend after action, to allow admin context)  
- Whether server admins can see rate-limit hit logs (recommend yes, as a signal of potential abuse or genuine power-user need)  
- Whether the raid cap should track attempts on nodes owned by the target separately from home region attacks (current: counted together, may need split)  
- Whether the cap should reset on round end or roll over (current: round-scoped, resets at round start)  
- Whether an admin can grant a one-time cap waiver for a specific attacker-target pair (e.g. "the final assault is sanctioned")  
- Profanity filter on report reason text and on handles, deferred from 17.1  
- Whether `trade ledger` should support `--format json` for tooling integration (likely yes once 16.1 format strings ship)  
- Audit log retention policy for admin actions and reports, deferred to 17.5 persistence model

---

## 17.3 Out-of-Game Integrations

What does and does not connect to external tools (Slack, email, calendar, webhooks). v1 ships none of these. The architecture is built so they can be added later without rework.

### Committed

**Scope**

No out-of-game integrations in v1. The game ships as a self-contained CLI experience with in-session notifications only (per 16.1 login banner and urgent state summary).

**Explicitly out of scope for v1**

- Slack bot and Slack notifications of any kind  
- Email notifications beyond magic link authentication (per 17.1)  
- Daily or round-end email digests  
- Mobile push notifications  
- Calendar integration (.ics exports, OAuth sync)  
- Outbound webhooks  
- Discord, Teams, or any other chat platform integration  
- SMS, RSS, or any other delivery channel

**In-session notification surface (already committed elsewhere)**

- REPL login banner with urgent state summary (16.1)  
- World announcements visible on next CLI session (Wonder phases, round-end, weather windows, Ruin claims)  
- Battle reports, scout reports, caravan status accessible via commands  
- No async push of any kind

**API design constraint**

The internal event model must be designed such that out-of-game integrations can be added post-v1 without backend rework. Specifically, gameplay events (battle resolved, Wonder phase change, attack detected, etc.) should be emitted to an internal event bus that any future integration layer can subscribe to. This is a non-functional requirement on the v1 architecture, not a v1 feature.

### Reasoning

**Why nothing**

- The core value proposition is the workday-fit, console-native loop. Adding integrations before validating that loop dilutes focus and conflates "is the game fun" with "are notifications well-tuned."  
- Slack and email integrations each carry real operational cost: bot hosting, OAuth flows, per-workspace install, deliverability concerns, abuse monitoring. None of these add gameplay.  
- The team-building goal is served by coworkers talking in their existing channels about what happened in the game, not by the game posting into those channels. v1 should test whether the in-game state alone generates that conversation.  
- Mobile push and calendar sync drag the game outside the workday, conflicting with the core positioning.  
- Webhooks, while lean to build, expose internal event schemas before they stabilize. Locking in a public contract pre-v1 is premature.

**Why the API constraint**

- Post-v1 integrations are the most likely first expansion. Designing the event bus from day one avoids a painful retrofit.  
- Costs nothing extra during v1 implementation since the game already needs an internal event flow for the REPL banner and world announcements.

### Alternatives considered

- **Slack notifications only (one-way bot)**: rejected. Even minimal Slack integration requires workspace install flow, channel configuration UX, deliverability monitoring, and abuse defenses. The team-building hypothesis can be tested without it.  
- **Daily email digest only**: rejected. Spam complaints, deliverability infrastructure, unsubscribe handling, and per-player digest configuration are all real burdens for a feature that mostly duplicates the next CLI login.  
- **Calendar .ics export**: rejected despite being trivial to build. Pulls in calendar UX expectations (timezone handling, event updates when rounds extend) that compound. Round-end ETAs are already in `wonder` output.  
- **Outbound webhooks only**: rejected. Lean to build, but locks in a public event schema before playtest reveals which events matter.  
- **Slack \+ email \+ webhooks combo**: rejected as scope creep.

### Open follow-ups

- Internal event bus design (event types, payload schemas, retention) deferred to 17.5 tech architecture  
- Slack bot as the most likely post-v1 integration, full design deferred to a future subsection  
- Email digest as second-most-likely post-v1 integration  
- Whether the eventual integration layer is per-server (admins install a Slack workspace) or per-player (each player connects their own Slack)  
- Whether webhook support arrives before or after Slack, depending on which serves the most users  
- Calendar .ics export as a low-cost post-v1 add if rounds become a workplace ritual worth scheduling around  
- Whether the v1 CLI should support a `--json` event tail mode (e.g. `dun events --follow`) that power users could pipe to their own scripts, providing webhook-like functionality without us building webhooks. Likely yes since 16.1 already commits `--json` output and a follow mode is a small extension

---

## 17.4 Persistence Model

What carries across rounds within a server: profile stats, leaderboards, winner titles, scoping rules, and the retention/deletion policy. Builds on 16.6 (round-over commits), 16.7 (server-scoped identity), and 17.1 (email as identity, handle as display name).

### Committed

**Stat surface**

Each player has a per-server profile tracking lifetime totals across all rounds played on that server. Per-round breakdowns are not stored on the profile, they live in the round archive (16.6).

Tracked stats:

| Stat | Definition |
| :---- | :---- |
| `rounds_played` | Rounds the player was a member of at any point |
| `rounds_won` | Rounds the player won by Consecration survival |
| `wonders_completed` | Wonders the player successfully built (= rounds\_won) |
| `wonders_destroyed` | Wonders the player destroyed as attacker (killing blow attribution) |
| `peak_nodes` | Highest node count controlled in a single round, across all rounds |
| `raids_launched` | Total offensive attacks dispatched that reached resolution |
| `raids_defended` | Total incoming attacks resolved at the player's regions |
| `raids_won_offense` | Subset of raids\_launched resulting in attacker victory |
| `raids_won_defense` | Subset of raids\_defended resulting in defender victory |
| `resources_looted` | Lifetime total resources stolen via successful raids, summed across all four types |

Stats update at the moment of resolution: battle end for raid counters, round end for peak\_nodes and round counters, Wonder destruction event for wonders\_destroyed.

**Leaderboards**

Four independent per-server leaderboards, each top 10 by default with `world hall-of-fame --all` exposing the full list.

| Leaderboard | Primary sort | Secondary sort |
| :---- | :---- | :---- |
| Champions | rounds\_won desc | wonders\_destroyed desc |
| Wreckers | wonders\_destroyed desc | rounds\_won desc |
| Warlords | peak\_nodes desc | rounds\_won desc |
| Veterans | rounds\_played desc | rounds\_won desc |

Leaderboards are snapshotted at round end and recomputed only when a round closes. Reads return the cached snapshot, immutable until the next round-end event. Empty leaderboards (server with zero completed rounds) render an empty list with a "no completed rounds yet" message.

**Wrecker attribution**

A Wonder destruction credits `wonders_destroyed` to exactly one player: the player whose Trebuchets dealt the killing blow (the attack that brought Wonder HP to 0). Coalition contributions are recorded in the round archive as assists but do not increment the leaderboard stat. Ties at the killing-blow battle (multiple players' Trebuchets in the same resolution) are broken by largest Trebuchet contribution in that battle, then by earliest dispatch timestamp.

**Titles**

Round winners receive a permanent title tied to the world they won.

Format: `[Champion of <World Name>]`

Display rules:

- Most recent title shown inline next to the handle in battle reports, world announcements, scout reports, hall of fame, and `player list` output  
- Repeat wins on the same world collapse into a count suffix: `[Champion of Aldermarch ×3]`  
- Repeat wins on different worlds: only the most recent appears inline, full title list visible in `player show`  
- `player show <handle>` displays the full title history with world names and round end dates, plus all profile stats

**Display surfaces**

| Surface | Content |
| :---- | :---- |
| Inline handle (battle reports, world announcements, scout reports, hall of fame, `player list`) | Handle \+ most recent title |
| `player show <handle>` | Handle, real name (per 17.1), full title list with world and round end timestamp, all 10 stats, server join date |
| `world hall-of-fame` | Top 10 of each of the four leaderboards |
| `world hall-of-fame --all` | Full ranked list for each leaderboard |
| `world hall-of-fame <leaderboard>` | Single leaderboard, top 10 |
| `world hall-of-fame <leaderboard> --all` | Single leaderboard, full list |

**Scoping**

- All stats, titles, and leaderboards are per-server (matches 16.7 identity model)  
- Same email on two servers maintains two independent profiles with no cross-server aggregation  
- No global view across servers

**Retention and deletion**

- Profiles and round archives persist permanently by default  
- Account deletion (initiated by the player via `dun auth delete-account` or by a server admin) zeroes the player's profile stats, removes their titles from leaderboards, and replaces their handle with `[deleted player]` in all historical battle reports, scout reports, world announcements, and round archives  
- Deleted accounts free their handle for reuse on the server after 30 days  
- Round archive structural integrity is preserved: battle outcomes, Wonder events, and timeline data remain intact, only the player attribution is anonymized  
- Real name is purged immediately on deletion  
- Deletion is irreversible

**Commands**

- `player show <handle>` displays full profile with title history and all stats  
- `world hall-of-fame` displays all four leaderboards, top 10 each  
- `world hall-of-fame <name>` displays a single leaderboard  
- `world hall-of-fame --all` or `world hall-of-fame <name> --all` displays full ranked lists  
- `dun auth delete-account` initiates the account deletion flow (requires confirmation)

### Reasoning

- **Stat surface** matches 16.6 commits exactly and gives enough material for office bragging without inflating the schema. Lifetime totals are cheap to update incrementally, per-round detail already lives in the archive (16.6), so duplicating it on the profile would be wasteful.  
- **Four leaderboards** create multiple recognition paths so the player who never wins but destroys every Wonder still has a public trophy. Single ranked lists collapse the social texture the game depends on.  
- **Snapshot-at-round-end leaderboards** make reads cheap, results immutable mid-round, and align with the 16.6 round-archive model. Live recomputation invites caching layers and edge cases when stats change mid-round.  
- **Killing-blow attribution** for Wreckers gives a clean answer to "who destroyed the Wonder" that battle reports already surface. Coalition-shared credit would dilute the leaderboard and create attribution arguments. Assists are logged for office context without leaderboard impact.  
- **Titles inline next to handle** are the lightweight cosmetic with maximum drama value: every battle report that mentions Marta now reads "Marta `[Champion of Aldermarch]`", which is exactly the kind of artifact that travels into Slack threads.  
- **Multi-win count suffix** keeps inline rendering compact while preserving the "she won three times" recognition. The full title history lives in `player show` for anyone who wants the breakdown.  
- **Per-server scoping** confirms 16.7 and avoids the cross-employer leak risk of any global view. A consultant on two servers maintains two separate identities, as intended.  
- **Anonymized deletion** balances GDPR right-to-erasure against archive integrity. Battle reports stay coherent ("`[deleted player]` raided Pale Coast on day 8") without preserving identifying data. Immediate real name purge meets the strongest interpretation of erasure requests.  
- **30-day handle quarantine** after deletion prevents immediate impersonation while still letting the handle re-enter the pool.

### Alternatives considered

- **Single ranked leaderboard**: rejected. One axis flattens the recognition surface and undervalues defensive/destructive play  
- **Elo-style rating**: rejected. Fragile with small per-server player pools, punishes late joiners, hostile to the casual workplace tone  
- **Seasonal leaderboards**: rejected. Adds a "season" concept not present elsewhere in the design and would require a separate cadence commitment  
- **Exhaustive stat tracking**: rejected. Stat bloat with most fields never read, expands the schema surface for marginal trophy-case gain  
- **Per-round breakdowns on profile**: rejected. Round archive already provides this, duplicating onto the profile is redundant  
- **No cosmetic carry-over**: rejected. Winners need a visible artifact beyond a stat increment, and titles are the cheapest way to provide it  
- **Wonder-name preservation in profile**: rejected as a separate field. The archive already records the Wonder name, `player show` can render it from there without a duplicate field  
- **Custom banners or colors**: rejected. Pulls 19.4 cosmetic scope into v1, adds a configuration surface that doesn't yet exist  
- **Cross-server global view**: rejected. Conflicts with 16.7 identity scoping and creates a leak vector across unrelated employers  
- **Auto-expire inactive profiles**: rejected. Arbitrary cutoff, loses returning-player history, no clear benefit over permanent retention  
- **Admin-configurable retention per server**: rejected. Defers a hard decision and creates inconsistent player experience across servers  
- **Hard deletion with archive integrity loss**: rejected. Breaks the round archive coherence promise from 16.6  
- **Coalition-shared Wrecker credit**: rejected. Splits the stat across attackers and creates attribution arguments. Killing-blow is unambiguous

### Open follow-ups

- Whether `peak_nodes` should be capped at the round's total node count (currently raw value, but a round with 8 nodes can never produce peak\_nodes \> 8, so the cap is implicit)  
- Whether resources\_looted should be split by resource type for the profile view (currently summed, may want per-resource breakdown in `player show`)  
- Whether titles should have rarity indicators (e.g. `[Champion of Aldermarch ×5]` shown in a distinct color in CLI output) deferred to display polish  
- Whether `world hall-of-fame` accepts a `--round <id>` flag to view a historical snapshot from a specific round-end (recommend yes, ties into 16.6 archive)  
- Whether killing-blow attribution should expose a tiebreaker log in the battle report when ties occur (recommend yes, transparency aids the "who actually killed it" Slack debate)  
- Whether the 30-day handle quarantine after deletion is configurable per server (recommend no, keep as a uniform anti-impersonation default)  
- Whether profile stats should be exportable (e.g. `player show <handle> --json`) once 16.1 JSON format ships (likely yes, no work required beyond standard JSON support)  
- Interaction with 17.2 moderation: whether an admin-initiated removal (vs voluntary deletion) results in the same anonymization or different treatment (recommend same treatment, simpler model)  
- Whether deleted accounts free their `wonders_destroyed` credit from historical Wreckers leaderboards immediately or only after the 30-day quarantine (recommend immediately, matches the stat zeroing)

---

## 17.5 Tech Architecture

How the system is built and operated: topology, application stack, the tick model, client distribution, hosting, and observability. Self-hosted per server.

### Committed

**Server topology**

Dedicated instance per server (single-tenant). Each company runs its own self-hosted stack. No shared database, no shared compute across servers. A server's worlds, players, archives, and observability are fully isolated.

- One database per server, one application stack per server  
- Failure blast radius is one company maximum  
- Schema migrations roll per server, no fleet-wide coordination required  
- Server upgrade is admin-triggered: `dun upgrade` pulls new image, runs migrations behind health checks, restarts services  
- Automatic pre-migration backup before any schema change

**Application stack**

Ruby on Rails 8 application, monolithic by design.

| Layer | Choice |
| :---- | :---- |
| Web framework | Rails 8 |
| Background jobs | Active Job with Solid Queue (Rails 8 native) |
| Recurring jobs | Solid Queue recurring tasks |
| Cache backend | Solid Cache (Rails 8 native) |
| Pub/sub backend | Solid Cable (Rails 8 native, available if needed post-v1) |
| Database | PostgreSQL 16+ |
| Test framework | Minitest |
| Mocking | Mocha |
| Fixtures and factories | factory\_bot |

Rationale for Rails 8 specifically: the Solid Queue / Solid Cache / Solid Cable trio removes Redis as a hard dependency for v1, simplifying the docker-compose surface and reducing the number of services a self-hosting admin operates. Redis can be reintroduced as a v1.1 cache or queue backend if Solid Queue throughput becomes a bottleneck.

**Tick model**

Fixed-interval ticks running on Solid Queue recurring jobs.

| Cadence | Job class | Responsibilities |
| :---- | :---- | :---- |
| 5 seconds | `DiscreteEventTick` | Build completions, training completions, march arrivals, battle resolutions, Wonder phase transitions, caravan arrivals and interception checks, weather window edges |
| 1 minute | `ProductionCheckpoint` | Snapshot stockpiles to DB, enforce Warehouse caps, record production accruals since last checkpoint |
| 5 minutes | `StatsRefresh` | Aggregate player stats, leaderboard recompute eligibility, audit-view cluster recomputation (17.2) |
| 1 hour | `WorldHousekeeping` | Grace-window expiry (16.8), weather window scheduling lookahead (16.11), rate-limit window rollover housekeeping, scout report cleanup eligibility |

Production and stockpile values are computed lazily on read (from last-checkpoint timestamp plus elapsed seconds at current production rate), with a guaranteed write at least once per minute. Discrete events are scheduled at exact timestamps and executed by the 5-second tick or earlier if the worker is idle.

Tick jitter tolerance is ±5 seconds for discrete events. ETAs displayed to players are rounded to the minute to hide jitter.

**Client-server protocol**

HTTP/JSON request-response. No push, no streaming, no long polling.

- CLI commands map to HTTP endpoints, authenticated via API key (17.1)  
- REPL banner refreshes on each command execution  
- All structured output available as JSON via `--json` (per 16.1)  
- Versioned API path (`/v1/...`) to enable future protocol evolution without breaking older clients

**Client distribution**

RubyGem published to rubygems.org.

- Install: `gem install dun`  
- Minimum Ruby version: 3.3+  
- Admin subcommands merged into the main `dun` binary (no separate `dun-server`). Admin commands gated by server role checks  
- Recommend `--user-install` to avoid global gem pollution  
- Self-update via `dun update` (checks rubygems.org for newer versions)  
- Tebako or Ruby Packer for single-binary distribution evaluated as a v1.1 enhancement if installation friction emerges in playtest

**Hosting model**

Self-hosted by each server admin. v1 launch targets Hetzner as the reference deployment, but any Docker host works.

- Reference deployment: single Hetzner CX22 or CPX21 (€5-10/month) running docker-compose  
- `docker-compose.yml` ships in the repository with services: `dun-web`, `dun-worker` (Solid Queue), `postgres`, `caddy` (TLS termination, automatic Let's Encrypt)  
- Bootstrap: `dun init-server` generates secrets, runs initial migrations, brings up the stack  
- Backup: nightly `pg_dump` to a configurable destination (S3-compatible, local volume, Hetzner Storage Box)  
- Updates: `dun upgrade` pulls new image, runs migrations with pre-backup, restarts services with health checks

Managed hosting is out of scope for v1. Revisit alongside 18.1 monetization.

**Observability**

OpenTelemetry instrumentation throughout the application. Self-hosted observability stack bundled as an opt-in docker-compose profile.

| Concern | Tool |
| :---- | :---- |
| Tracing | OpenTelemetry SDK \+ auto-instrumentation gems → Tempo |
| Logs | Structured JSON logs (lograge or ougai) → Loki |
| Metrics | OpenTelemetry metrics → VictoriaMetrics |
| Dashboards | Grafana |

- Bundled stack opt-in via `docker-compose --profile observability up`  
- Admins running large servers can ship telemetry to their own external backend by overriding OTel exporter endpoints  
- Suggested external alternatives documented in the runbook (Honeycomb, Grafana Cloud, Datadog, self-hosted SigNoz, etc.)  
- Bundled stack resource overhead noted in docs: adds roughly 1-2GB RAM to the host, recommend a CPX31 or larger if running both dun and observability on one VM

### Reasoning

- **Single-tenant** isolates failure domains and matches the 17.1 server-as-sovereign model. Self-hosted distribution makes per-server stacks essentially free operationally because the admin owns the infrastructure cost. Iteration is faster in v1 because schema and feature changes can roll to a single test server without coordinating a fleet.  
- **Rails 8** maximizes iteration speed during a design-heavy v1 where mechanics will change weekly. ActiveRecord handles the relational game state (players, regions, armies, battles, nodes, caravans) without ORM ceremony. The mature ecosystem means most operational problems have a known answer.  
- **Solid Queue / Cache** removes Redis as a hard dependency. One less service for self-hosting admins to operate, one less backup target, one less failure mode. Throughput is sufficient for v1 player scale (dozens of worlds per server, thousands of jobs per minute peak).  
- **5-second discrete tick** is precise enough for battle resolution and Wonder phase timing while halving the load of a 1-second tick. ETA display is minute-rounded, so 5s jitter is invisible to players. Revisit only if playtest reveals timing complaints.  
- **Lazy production with 1-minute checkpoint** keeps reads cheap (compute from timestamp delta) while guaranteeing durable state at minute granularity. Warehouse cap enforcement runs at checkpoint to prevent overflows from ever being observable.  
- **HTTP/JSON only** matches the 16.1 stateful REPL model and 17.3 no-push commitment. Corporate network friction is zero since outbound HTTPS is universally available. SSE or WebSocket reintroduced only if mid-session staleness becomes a real complaint.  
- **RubyGem distribution** matches the language choice and developer audience. Ruby is widely installed on dev machines, and `gem install` is a familiar workflow. Single-binary packaging via Tebako stays in the v1.1 queue as a friction-reduction lever rather than a v1 requirement.  
- **Merged admin CLI** keeps the surface coherent. One binary, one credential store, one help system. Admin subcommands gated by server role checks (17.1 admin model). A separate `dun-server` binary would duplicate auth, config loading, and help infrastructure for no clear benefit at v1 scale.  
- **Hetzner reference deployment** sets a concrete cost-and-capability target without locking admins to a provider. docker-compose is portable to any Docker host (Fly.io, DigitalOcean, AWS Lightsail, on-prem). The reference numbers help admins right-size their first deployment.  
- **OpenTelemetry as the instrumentation standard** future-proofs the telemetry layer. Admins who already operate observability infrastructure can point exporters at their existing backends. Admins who don't get a working stack out of the box.  
- **Bundled Grafana stack opt-in** acknowledges that self-hosting admins won't configure observability if it's a 12-step exercise. One docker-compose profile flag turns it on. VictoriaMetrics over Prometheus for lower resource footprint at small scale, Tempo and Loki for the Grafana-native experience.

### Alternatives considered

- **Multi-tenant with dedicated upgrade tier**: rejected for v1. Multi-tenant invites cross-server bug exposure during a phase when bugs are the norm. Revisit when v1 is stable  
- **Shared compute, isolated DB**: rejected. Marginal infrastructure savings, real operational complexity (connection pool sprawl, schema migration coordination per DB)  
- **Go**: rejected in favor of Rails. Performance advantage is real but doesn't matter for dun's I/O-bound workload. Rails iteration speed wins during the design-heavy v1 phase  
- **Rust**: rejected. Iteration speed too slow for v1, type safety advantages don't pay off until mechanics stabilize  
- **Elixir**: rejected. Niche hiring, smaller ecosystem, ops less familiar to most admins who will self-host  
- **Rails with Sidekiq instead of Solid Queue**: rejected for v1 to avoid Redis dependency. Sidekiq is more battle-tested at high scale, but Solid Queue handles v1 scale comfortably and removes a service. Revisit if Solid Queue shows throughput problems  
- **Rails with PostgreSQL but Sequel ORM**: rejected. Sequel is excellent but ActiveRecord's integration with the rest of Rails (validations, callbacks, fixtures, generators) outweighs Sequel's query elegance for this codebase  
- **RSpec instead of Minitest**: rejected per user preference. Minitest is faster and ships with Rails, RSpec adds a dependency and a DSL learning surface for new contributors  
- **Fixed 1-second tick**: rejected. Doubles tick load with no visible player benefit given minute-rounded ETAs  
- **Fully lazy materialization**: rejected. Breaks weather windows, Consecration countdowns, round-end announcements, and other events that must fire even when no player is observing  
- **SSE for live banner**: rejected for v1. REPL banner staleness within a session is acceptable since each command refreshes it. Reintroduce in v1.1 if playtest reveals demand  
- **WebSocket**: rejected. Corporate firewall friction, infrastructure complexity, no clear v1 user benefit  
- **Single binary via GitHub Releases**: rejected for v1 given Ruby choice. Tebako packaging adds build complexity. RubyGem is the natural channel for a Ruby tool. Single binary stays in the v1.1 queue  
- **Multi-channel distribution (gem \+ Homebrew \+ apt)**: rejected for v1. Maintenance overhead for marginal reach gain when target audience is developers comfortable with `gem install`  
- **Single managed cloud region**: rejected for v1. Self-hosted is cheaper, simpler, and avoids the operational responsibility of running a SaaS during a phase when the product is still being designed  
- **Multi-region active-active**: rejected. Massive complexity for an async game with hour-scale actions where 200ms latency is invisible  
- **Bundled observability without VictoriaMetrics (Prometheus instead)**: rejected. Prometheus resource footprint is higher at small scale. VictoriaMetrics is wire-compatible and lighter  
- **Datadog or Honeycomb as the default backend**: rejected. Forces admins into a vendor relationship for v1 testing. OTel exporter swap remains available for admins who prefer SaaS observability

### Open follow-ups

- Whether Solid Queue throughput holds at v1.1 scale (multiple companies with 50+ players each, 10+ concurrent worlds per server). Revisit Sidekiq if benchmarks show queue lag during peak tick load  
- Exact 5-second discrete tick budget: max number of events resolvable per tick before the worker falls behind. Needs a load test before v1 launch  
- Whether the bundled observability profile should include alerting (Grafana Alerting or similar) or stay pure-visualization for v1  
- Tebako single-binary packaging as a v1.1 deliverable, conditional on installation friction reports from v1 admins  
- Backup destination defaults: should `dun init-server` prompt for an S3 endpoint, default to local volume, or stay unconfigured until admin sets it. Recommend local volume default with a warning banner until configured  
- Schema migration strategy for downgrade: v1 supports forward migrations only via `dun upgrade`. Rollback strategy (restore from pre-migration backup) documented but not automated. Automated rollback deferred to v1.2  
- Whether server-to-server federation (cross-server trade, cross-server tournaments) is permanently out of scope or a v2 consideration. Current design assumes hard isolation per server  
- Multi-arch Docker images (amd64 \+ arm64) for Hetzner ARM instances and Apple Silicon admin development environments. Recommend yes for v1, GitHub Actions can build both  
- TLS certificate management when the server runs behind a corporate proxy or on a domain without public DNS. Caddy \+ Let's Encrypt requires public reachability. Document manual cert mode for air-gapped or internal-only deployments  
- Whether `dun upgrade` should support pinning to a specific version or always pull `latest`. Recommend version pinning with `dun upgrade --to v1.2.3` and `latest` as default  
- Observability data retention defaults: 7 days for logs, 30 days for metrics, 7 days for traces. Configurable per admin. Document in runbook  
- Whether the admin console is purely CLI (`dun admin ...`) or also exposes a minimal web dashboard for non-CLI-comfortable admins. Recommend CLI-only for v1, web dashboard as a v1.1 enhancement targeting non-developer admins (HR, ops)  
- Interaction with 17.4 account deletion: ensure pre-deletion DB backup runs and is retained per backup policy, so GDPR erasure does not silently destroy archive recoverability

---

# Part IV — Business and Future

Section 18 holds the still-pending business model decisions. Section 19 catalogs deferred features that have been intentionally pushed past v1. Section 20 is a quick-reference summary of everything committed.

---

# 18\. Business Model and Rollout

This section is pending. The core gameplay design is sufficiently complete to begin implementation, but the business model and launch strategy require further discussion. Both subsections below list the open questions and constraints established so far.

## 18.1 Monetization (Pending)

### Open questions

- Free for individual self-hosted servers, or charge per server, per seat, or per world  
- Hosted offering pricing model if managed hosting is introduced post-v1 (per 17.5)  
- Whether premium cosmetics (19.4) are an in-game purchase, a server-level admin purchase, or a subscription perk  
- Whether monetization is the same across the open-source self-host and a future hosted offering, or differentiated  
- License model: source-available, open-source under permissive license, or proprietary  
- Whether companies pay for admin tools (audit views, advanced moderation, SSO support per 17.1) as an enterprise tier  
- Treatment of personal-use (non-company) servers  
- Whether the bundled observability stack is free for all or gated

### Constraints established

- Premium spawn slots are an internal generator concept only and are never exposed to players (16.8). Monetization cannot expose them  
- The team-building positioning rules out any pay-to-win mechanic: monetization must not affect resources, units, buildings, combat outcomes, or any gameplay state  
- Cosmetics (19.4) are the most likely monetizable surface  
- Anything sold to a player individually must not produce in-game advantage over their coworkers

## 18.2 Launch and Growth Strategy (Pending)

### Open questions

- Whether to launch open beta to a small group of companies or via public availability  
- How to seed the first wave of servers (founder networks, hand-picked engineering orgs, broad open call)  
- Whether a hosted demo server exists for evaluation before self-hosting  
- Documentation and onboarding scope for the admin install path  
- Whether the first launch milestone is "first round completed" or "first round with N concurrent players"  
- Channels for distribution and discovery (Hacker News, developer Slack communities, conference demos, blog posts)  
- Whether marketing leads with the team-building angle (HR-friendly) or the developer-fun angle (engineer-friendly), or both  
- Telemetry and feedback loop during launch (per 17.5 observability) for early issue detection  
- Whether a public roadmap of v1.1 / v1.2 features is published at launch

### Constraints established

- Self-hosted distribution is the v1 default (17.5), constraining the launch surface to admins willing to operate Docker  
- Workplace-context positioning rules out marketing channels or messaging that conflict with HR norms  
- The team-building hypothesis is unverified at launch and is the first claim to test

---

# 19\. Future Enhancements

Mechanics intentionally deferred from v1. These are recorded so the v1 design surface does not silently absorb them, and so post-v1 planning has a starting catalog.

Each subsection below describes the deferred feature, the reasoning for deferring it, and any v1 design hooks that make a future addition cheaper.

## 19.1 Specialized Unit Types

Additional unit archetypes beyond the 8 committed in 16.3.

### Candidates

- **Wonder Garrison**: specialized defender unit referenced in Section 14, defensive bonus inside the Wonder kingdom during Construction and Consecration phases  
- **Spy / Saboteur**: covert unit that bypasses standard scout interception (16.9), potentially disrupts production or reveals queue contents  
- **Diplomat**: facilitates a formal pact mechanic if 12.diplomacy is revisited  
- **Naval units**: if coastal regions and water terrain are introduced post-v1  
- **Mercenaries**: hireable units paid in Gold with a time-limited contract, potentially balancing late-round catch-up

### Rationale for deferral

The 8-unit roster covers the conquest spine, RPS layer, siege, scouting, and Wonder attack/defense. Each candidate above either duplicates an existing role (Saboteur vs Scout), depends on a deferred mechanic (Diplomat vs revised 12, naval vs water terrain), or risks balance debt before the v1 unit interactions are validated by playtest. Revisit individually once v1 combat stabilizes.

### v1 hooks

- 16.3 stats schema accommodates new unit rows without restructuring  
- 16.1 command grammar already parses arbitrary unit names in `train` and army composition

## 19.2 Heroes and Champions

Persistent named units that level up, gain abilities, and follow a player across or within rounds.

### Scope sketch

- One Hero per player, named at first round on a server  
- Hero accompanies a designated army and provides modifiers (combat bonus, march speed, scout effectiveness)  
- Hero gains experience from battles, levels up within a round, resets at round end (or persists per profile)  
- Optional cosmetic carry-over: Hero name and title carry across rounds per 17.4 model

### Rationale for deferral

Heroes add a parallel progression axis on top of buildings, units, and nodes. v1 already balances three progression layers and the Wonder timeline. Adding Heroes would require rebalancing battle resolution (16.3), reconsidering the unit roster (16.3), and possibly redesigning carry-over (17.4). The conquest spine works without Heroes, and the team-building goal does not depend on them.

### v1 hooks

- 17.4 persistent profile schema supports adding a hero\_name field without migration risk  
- 16.3 combat resolution accepts unit-specific multipliers, which could extend to Hero modifiers

## 19.3 Quest and Event System

Server-driven or world-driven objectives that grant rewards on completion.

### Scope sketch

- Round-scoped events triggered by world milestones (e.g. "first Wonder Foundation triggers a 24-hour Rich-node race")  
- Player-personal objectives (e.g. "defeat a Knight army" with a small resource cache reward)  
- Admin-authored seasonal events for special rounds (themed cadence, custom rewards)  
- Integration with the existing Weather window cadence (16.11) as an event surface

### Rationale for deferral

Ruins (16.11) already provide a static event-like surface, and Weather windows already provide a recurring event-like surface. A full quest system would require authoring tools, reward balancing, and a content pipeline that v1 does not need. Defer until v1 reveals whether players want more structured objectives beyond raid / build / Wonder.

### v1 hooks

- 16.11 weather window cadence and announcement infrastructure is reusable for any event scheduler  
- 17.3 internal event bus design includes generic event types that a quest layer can subscribe to without backend rework

## 19.4 Cosmetics and Personalization

Visual or textual customization beyond the title system in 17.4.

### Candidates

- **Custom kingdom banners**: ASCII art or colored sigil rendered in `status` and battle reports  
- **Custom Wonder descriptions**: builder-authored text shown in the Wonder world announcement  
- **Color themes for CLI output**: per-player color palette for self vs others vs neutral, beyond the 16.1 ANSI 16 defaults  
- **Title customization**: post-victory choice from a menu of suffixes ("The Bold", "the Iron", "of the Mountain")  
- **Battle report flair**: short builder-authored taglines appended to Wonder events  
- **Custom handle prefixes**: server-admin-grantable cosmetic prefixes (e.g. `[Founder]`, `[Alumnus]`)

### Rationale for deferral

Cosmetics are the most likely monetizable surface (18.1) and the most likely vector for player expression beyond combat outcomes. v1 ships with the title system from 17.4, which covers the highest-value cosmetic (round-winner recognition). Additional cosmetics depend on monetization decisions and are best designed once the model in 18.1 is committed.

### v1 hooks

- 17.4 title system already renders inline beside handles, providing the integration surface for additional cosmetic suffixes  
- 16.1 ANSI 16 palette and `\color` toggle commit a CLI rendering layer that custom themes can extend

---

# 20\. Quick-Reference Summary

A condensed reference covering all committed decisions in Parts I through III. Use as a refresher or design check, the full reasoning lives in the respective sections.

## Identity and positioning

- **Game name**: `dun` (from Gaelic for an ancient or medieval fort)  
- **Genre**: async multiplayer turn-based strategy with kingdom-building, conquest spine, light diplomacy  
- **Theme**: medieval fantasy kingdoms  
- **Platform**: console / CLI, native to developer terminals  
- **Target user**: software developers with frequent micro-idle moments at work  
- **Positioning**: passive team-building tool for coworkers in the same server (typically a company)  
- **Session shape**: 30 seconds to 5 minutes, 1-10 sessions per day, moderate absence tolerance  
- **Round length**: 2-4 weeks typical, ends when a Wonder survives Consecration

## Win condition

- **Wonder victory only**: build a Wonder, survive its 24-hour Consecration phase  
- No score fallback, no time cap, no alternate paths  
- Wonder name is player-chosen from a menu, mechanically identical options  
- Wonder built in the player's home kingdom

## Economy

- **Four resources**: Gold (currency), Wood (renewable), Stone (defensive/endgame), Iron (military)  
- **Production model**: four resource buildings \+ capturable map nodes  
- **Stockpile cap**: Warehouse, quadratic scaling, \~1M per resource at L20  
- **Loot**: up to 25% of defender stockpile per raid, capped by carrying capacity

## Map

- **Procedural per round**: seeded planar graph, 16-64 regions scaling with player count (2.5 regions per player \+ 6\)  
- **Five terrain types**: Plains, Forest, Hills, Mountain, Marsh, with march and combat modifiers (16.10)  
- **Resource nodes**: 1.2 per player, three tiers (Rich \+500/h, Standard \+250/h, Poor \+120/h), Stone overweighted to 35% of nodes  
- **Static garrisons** on unclaimed nodes, no respawn, no PvE roaming  
- **Static Ruins**: 2-6 per map, three tiers with one-time resource caches (16.11)  
- **Weather windows**: recurring 24-hour terrain modifiers, telegraphed 12h ahead (16.11)  
- **Spawn placement**: Poisson-disk, 2-hop minimum spacing, Plains or Hills only, 2+ adjacent wilderness regions (16.5, 16.8, 16.10)

## Military

- **Eight units across three tiers**: Levy, Archer, Pikeman, Knight, Catapult, Royal Guard, Scout, Trebuchet  
- **Rock-paper-scissors layer**: Knights \> Archers, Pikemen \> Knights, Archers \> Pikemen (1.4-1.6x multipliers)  
- **Specialist roles**: Catapults capture nodes and damage walls, Trebuchets damage Wonders (50 HP per surviving unit), Royal Guard as no-RPS anchor  
- **Combat resolution**: 6-round simulation, ±8% variance, defender bonus \+20% home, terrain caps at \+25%  
- **Carrying capacity**: per-unit stat, gates loot and caravan escort sizing  
- **March speed**: terrain-modified, slowest-unit rule, Knights and Scouts terrain-immune

## Buildings

- **12 buildings**: Town Hall, Gold Mint, Lumber Camp, Quarry, Iron Mine, Warehouse, Barracks, Stable, Siege Workshop, Walls, Watchtower, Stone Mason  
- **Linear levels 1-20** with exponential cost (1.75x per level) and exponential time (1.55x with 24h cap)  
- **Single upgrade slot**, Town Hall unlocks \+1 at L10 and L20  
- **Tier gates**: Stable requires Barracks 3, Siege Workshop requires Barracks 5 and Iron Mine 5, Wonder requires multiple prerequisites  
- **75% cancel refund** on resources, no time refund  
- **Stone Mason**: \-2% build time per level, capped at \-30% at L15

## Wonder

- **Total cost**: 800k Gold, 600k Wood, 2.4M Stone, 800k Iron (Stone-heavy)  
- **Three phases**: Foundation (instant, 25% payment), Construction (90h at 100 HP/h to 10,000 HP with milestone payments at 25/50/75%), Consecration (24h at maximum vulnerability)  
- **Defense**: home kingdom garrison, Walls, Watchtower  
- **Repair**: 1 HP per 8 Stone, capped 2,000 HP per phase  
- **Damage vectors**: Trebuchets (HP), Catapults (Walls), raids (garrison and Wonder-construction stockpile)  
- **Locked queue** during Construction (no other building upgrades), unit training continues  
- **Restart on destruction**: builder loses all paid resources, must start over

## Diplomacy and social layer

- **Trade only**: resources sent via caravan, intercepted en route, public ledger  
- **No formal pacts, alliances, or intel sharing in-game**  
- All coordination handled out of game (Slack, voice, in person)  
- **Marketplace deferred** to post-v1, direct caravan trade is the only v1 mechanic

## Onboarding

- **Company-scheduled rounds** with on-demand organizer trigger and minimum player count  
- **72-hour grace window** after T0 for joining, then closed  
- **Late-joiner stockpile bonus**: \+1000 per resource per 12h elapsed, capped at \+4000 (16.8)  
- **Pre-built starter kingdom**: each resource building at L1, Barracks at L1, Walls L1, Watchtower L1, 500 of each resource, 20 Levy  
- **Contextual hints**, opt-in `tutorial` command, no forced tutorial

## Console interaction

- **Stateful REPL** by default (`dun` drops to `dun>` prompt), one-shot mode for scripting  
- **Mixed grammar**: top-level verbs for frequent actions, noun subcommands for inspection, backslash meta commands  
- **Colored output**: ANSI 16, auto-detect TTY, `NO_COLOR` respected  
- **JSON output** via `--json` or `\json on`, structured for tooling  
- **Tab completion**, contextual hints (fade after first uses), did-you-mean error suggestions  
- **In-session notifications only**: login banner, urgent state summary, no push to external channels in v1

## Round lifecycle

- **Instant freeze** on Wonder Consecration survival  
- **No mandatory cooldown** between rounds  
- **Manual proposal** by an organizer for the next round  
- **No gameplay carry-over**: full reset of resources, buildings, armies, nodes  
- **Persistent profile carry-over**: lifetime stats, titles, hall of fame, all per-server scoped  
- **Permanent round archive**: full state and battle log retained per server

## Identity and access

- **Email as identity unit**, ownership verified via magic link  
- **Long-lived API key** stored in `~/.config/dun/credentials`, 90-day rolling expiry  
- **Server-scoped identity**: same email on multiple servers maintains independent profiles  
- **Server access**: domain whitelist \+ explicit invite list, union model, operator-configurable  
- **World caps**: `max_concurrent_worlds` and `max_worlds_per_account`, both default 2, configurable per server  
- **Display name model**: handle (public, drives all gameplay surfaces) \+ real name (visible only in `player show`)  
- **Per-server admins**, server creator is initial admin, minimum one admin always

## Anti-abuse

- **Multi-account defense**: domain enforcement \+ admin audit view, no auto-action  
- **Trade ledger**: world-scoped, public, permanent for the round  
- **Reporting**: `report <player> <reason>`, non-anonymous, scoped to server admin queue  
- **Rate limits**: 60 writes/minute, 1000 writes/hour per account, reads unlimited  
- **Repeat-raid cap**: configurable per server, default 3 raids per attacker-target pair per 24h, Wonder assaults exempt

## Out-of-game integrations

- **None in v1**: no Slack, no email digests, no calendar, no webhooks, no push  
- **Magic link email** is the sole outbound message in v1 (per 17.1)  
- **Internal event bus** designed so post-v1 integrations can be added without backend rework

## Persistence and stats

- **Per-server lifetime profile**: rounds played and won, Wonders completed and destroyed, peak nodes, raid counts (offense and defense), resources looted  
- **Four leaderboards**: Champions, Wreckers, Warlords, Veterans, snapshotted at round end  
- **Titles**: `[Champion of <World Name>]` permanent on round win, inline beside handle, repeat-win count suffix  
- **Account deletion**: anonymizes handle to `[deleted player]` in all archives, purges real name immediately, 30-day handle quarantine

## Tech architecture

- **Per-server self-hosted** stack, single-tenant, full isolation  
- **Rails 8 monolith**, Solid Queue / Solid Cache / Solid Cable (no Redis dependency in v1)  
- **PostgreSQL 16+** as the only database  
- **5-second discrete event tick**, 1-minute production checkpoints, 5-minute stats refresh, 1-hour housekeeping  
- **HTTP/JSON protocol**, no push or streaming in v1  
- **Distribution**: RubyGem, single `dun` binary, Ruby 3.3+  
- **Reference deployment**: Hetzner CX22/CPX21, docker-compose with Caddy for TLS, nightly `pg_dump` backup  
- **Observability**: OpenTelemetry instrumentation, opt-in bundled Grafana stack (Tempo, Loki, VictoriaMetrics)

## Pending and deferred

- **Section 18 (Business model and rollout)**: monetization model, launch strategy, license, hosted offering, all open  
- **Section 19 (Future enhancements)**: specialized units, heroes, quests and events, additional cosmetics, all deferred  
- **v1.1 in-scope**: fog of war (16.9 already designed), SSO support beyond magic link, possibly Slack integration as first external surface

---

