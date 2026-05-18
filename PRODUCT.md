# PRODUCT.md — dun-cli

`dun-cli` is the terminal client for [dun](https://github.com/fguillen/dun),
an async multiplayer medieval-fantasy strategy game built for developer
micro-idle moments. The game itself is described in
[docs/backend/product.md](docs/backend/product.md); the rules and
mechanics are in [docs/backend/game-design.md](docs/backend/game-design.md).
This document is about the *client* — what it is, who it's for, and the
principles that constrain its UX.

## What it is

A keyboard-driven Bubble Tea TUI that lives in the same terminal the
target user already has open. Single binary, no install ritual, no
configuration required after `dun login`. It speaks the player surface
of the `dun` JSON API; it does not run the game.

## Who it's for

Software developers and adjacent technical roles (SRE, data engineers,
DevOps) who:

- Spend most of their workday in a terminal or IDE.
- Hit multiple short idle windows per day (builds, CI, deploys, LLM
  responses, package installs).
- Prefer keyboard, text, and muscle memory over mouse-driven UIs.
- Want to participate in a coworker-bound persistent world without
  context-switching to a browser tab.

The CLI is the *only* way they will interact with `dun`. There is no web
client. Every constraint below derives from that.

## Session shape

| Aspect | Value |
| :---- | :---- |
| Cold-launch budget | < 200 ms to splash, < 500 ms to first useful view |
| Session length | 30 s – 5 min (sweet spot 1–3 min) |
| Sessions per day | 1 – 10 |
| Round length | 2 – 4 weeks |

A session is "30 seconds while my Docker build runs." Anything that
costs the user a second of staring at a loading spinner is a design
defect.

## UX principles

1. **Launch fast, always.** Cold start is on the critical path. Defer
   network calls until the user picks a screen that needs them; never
   block the splash on an API call.
2. **Keyboard-first, mouse-irrelevant.** Every action has a keybinding.
   Mouse support is opt-in and never required.
3. **Low cognitive load.** Show the next 1–3 actions, not all 12. Status
   bar tells you where you are and what `?` opens.
4. **Optimistic where safe, blocking where consequential.** Refresh a
   list view in the background; confirm before launching an army.
5. **Errors are first-class.** Surface the backend's error envelope
   `code` and `X-Request-Id` in a toast the user can copy. Don't swallow.
6. **Persistent world ≠ persistent UI.** The TUI is stateless across
   sessions; the truth lives in the backend. We can be killed and
   restarted at any moment without losing anything.

## Anti-goals

- No browser. No GUI. No real-time multiplayer interactivity.
- No background daemon. The CLI runs only while the user is in it.
- No admin surface. Server creation, world configuration, and player
  management are out of scope; admins use the API directly (or a
  separate tool, eventually).

## Where this fits

The CLI is consumer #1 of the `dun` HTTP API. Future consumers may
appear (a web client, a Slack/Discord bot, a mobile app), but the API
contract is owned by the backend repo and mirrored into
[docs/backend/](docs/backend/) here. This client adapts to the API; the
API does not adapt to this client.
