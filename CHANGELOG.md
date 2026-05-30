# Changelog

All notable changes to `dun-cli` are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.1] - 2026-05-30

First tagged release. Covers the full v1 player surface (Phases 0–13) and
the start of the admin track (Phase 14).

### Added

#### Player surface

- **REPL shell** (Phases 3–4): interactive `dun>` prompt with tab completion,
  line history (`~/.dun/history`), and scrollback-preserving (non-alt-screen)
  rendering. Server-membership and profile verbs.
- **Auth** (Phase 2): magic-link login, file-based credentials store
  (`~/.dun/credentials`, mode 0600), and account commands (`login`, `logout`).
- **API client** (Phase 1): hand-rolled wrapper around the ogen-generated
  client with auth, retries, typed error-envelope decoding, and `X-Request-Id`
  threading. Full request/response logging at debug level.
- **Worlds, map & kingdom** (Phases 5–7): world join, `map`/`regions`,
  kingdom dashboard. `world use` & `server use` scope verbs. Region owners on
  the map and a kingdoms roster. Home region on the kingdom dashboard; real
  owner shown on home-hoard nodes.
- **Map redesign**: multi-line region blocks showing your-army presence, real
  march ETAs, and visible armies.
- **Training, armies & marches** (Phase 8): unit training with a trainable-unit
  catalog preview; active march target & ETA shown on armies.
- **Combat & battle reports** (Phase 9).
- **Nodes & ruins** (Phase 10): capture wizards (node attack merged into
  capture for the backend `Nodes::Capture` flow).
- **Caravans & trade ledger** (Phase 11).
- **Wonders** (Phase 12).
- **Archive & hall of fame** (Phase 13).
- **Events**: `events` root verb for the kingdom event timeline.
- **Version display**: CLI version shown in the shell welcome line, the
  `version` verb, and the `dun version` subcommand.
- **Loop command**: repeat the last command on an interval.

#### Admin surface

- **Admin auth & shell** (Phase 14): `dun admin` enters a dedicated
  `dun-admin>` REPL with its own credential scope and `admin-state.json`
  session context.

#### Foundations

- Cobra root with `version` and `tui` subcommands; Bubble Tea splash screen
  (Phase 0).
- `~/.dun/` storage layout (no XDG, no OS keychain); slog JSON logging to
  `~/.dun/dun-cli.log`.
- `profile show` reads the player's own profile from the backend.

### Fixed

- Handle null participant `kingdom_id` for wilderness battles.
- Handle null defender `kingdom_id` for wilderness battles.
- Flag unclaimed home-hoard regions in the map as `(wild*)`.
- Tighten profile handle rule to 3–24 characters.

[0.5.1]: https://github.com/fguillen/dun-cli/releases/tag/v0.5.1
