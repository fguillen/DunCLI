# 06 — TUI Primitives

Phase 3 of [TODO.md](../../TODO.md), the parts of the TUI shared by
every verb that needs richer input than a single typed line.

The primitives live under
[internal/tui/selector/](../../internal/tui/selector/) (transient
Bubble Tea programs) and
[internal/tui/theme/](../../internal/tui/theme/) (the Lipgloss
palette). Verbs reach for these helpers; verbs never import
bubbletea, lipgloss, or huh directly.

---

## The two rules that shape this package

1. **Scrollback is sacred.** The REPL prompt does not run in
   alt-screen — see [CLAUDE.md](../../CLAUDE.md) "Interaction model".
   Transient programs (selectors, forms, confirmations) **may** open
   alt-screen, but they must exit cleanly and leave the final result
   to be printed back into scrollback as plain text by the calling
   verb.
2. **One package, one job each.** `selector.Pick` is for picking from
   a list. `selector.Form` is for multi-field input.
   `selector.Confirm` is for y/N. Verbs that need something else go
   build it here, not in their own file.

If a verb needs a primitive that doesn't exist yet, add it to
`internal/tui/selector` — keep the surface narrow and the API
function-shaped (`Pick(ctx, title, items) (Item, error)`), not
component-shaped.

---

## `selector.Pick` — list picker

[selector/list.go](../../internal/tui/selector/list.go) wraps
`charmbracelet/bubbles/list` in a single `Pick` function.

```go
type Item struct {
    Title       string
    Description string
    Value       string  // opaque payload (slug, ULID, …)
}

func Pick(ctx context.Context, title string, items []Item) (Item, error)
```

Bindings:

| Key | Action |
|---|---|
| `↑` / `↓` (and Bubbles defaults) | Move selection |
| `Enter` | Choose the highlighted item |
| `Esc` / `Ctrl-C` / `q` | Return `ErrCancelled` |

Cancellation is a **soft signal** — callers should treat
`ErrCancelled` as "user backed out" and return `nil` so the dispatch
loop just goes back to the prompt without a red error line.

Verbs use `Pick` when there's a finite list to choose from and an
explicit name is missing:

- `postLoginServerPicker` in
  [servers.go](../../internal/tui/verbs/servers.go) opens it when the
  player has ≥2 server memberships.
- `pickUpgradableBuilding` in
  [kingdom.go](../../internal/tui/verbs/kingdom.go) opens it when
  `build` is called with no `<kind>` argument.

Implementation detail: `Pick` opens with `tea.WithAltScreen()` because
the list redraws on every keystroke, and a redraw in scrollback would
spam history. The alt-screen is local to this transient program —
when it exits, the terminal returns to scrollback with the prompt
intact.

`Pick` also runs with `tea.WithContext(ctx)` so a parent context
cancellation tears down the program.

---

## `selector.Confirm` — y/N prompt

[selector/form.go:30](../../internal/tui/selector/form.go#L30) wraps
a single `huh.NewConfirm` in a function:

```go
func Confirm(ctx context.Context, title, description string) (bool, error)
```

Verbs use this in front of irreversible mutations:

- Before `queueBuildOrder`: "Queue upgrade: town_hall → L2?  Resources
  will be deducted immediately. You can cancel later for a 75%
  refund."
- Before `cancelBuildOrder`: "Cancel this build order?  This refunds
  75% of resources spent. Elapsed time is lost."

`description` is optional. ErrCancelled is returned on Esc / Ctrl-C
(user aborted before choosing).

The `huh` form uses `ThemeBase16()` — the closest huh theme to the
shared palette. The Lipgloss `theme.Active()` selection styling
applies to the rest of the CLI; huh runs its own theme inside its
program because retrofitting huh to use external Lipgloss styles
would be a substantial fork.

---

## `selector.Form` — multi-field input

[selector/form.go:53](../../internal/tui/selector/form.go#L53) is a
huh-backed multi-input prompt:

```go
type Field struct {
    Key      string                    // map key in the returned Result
    Label    string                    // shown above the input
    Initial  string                    // seed value
    Validate func(string) error        // optional client-side validation
}

type Result map[string]string

func Form(ctx context.Context, title string, fields []Field) (Result, error)
```

The canonical caller is `runProfileSet` in
[profile.go](../../internal/tui/verbs/profile.go) — when both flags
are omitted, the verb opens a Form with the handle (seeded from
`Context.KingdomHandle()`) and real-name fields. The handle field
wires the `validateHandle` regex into `Validate`, so an
obviously-bad handle is rejected inline before the form returns.

Implementation:

- One `huh.NewInput()` per `Field`, mounted in a single
  `huh.NewGroup`.
- Values are written into `map[string]*string` so the closure can
  read them out after `RunWithContext` returns.
- ErrUserAborted from huh maps to `selector.ErrCancelled`.

The CLAUDE.md "Interaction model" guidance is firm here: forms are
for multi-field input where flags would be awkward. When the verb's
inputs are all positional or flag-shaped, prefer the dispatcher's
flag parsing — fewer surprises, no terminal modes to enter.

---

## `theme` — the Lipgloss palette

[theme/theme.go](../../internal/tui/theme/theme.go) exposes a single
`Styles` struct resolved once at process start via
`termenv.HasDarkBackground()`:

```go
type Styles struct {
    Prompt   lipgloss.Style
    Subtle   lipgloss.Style
    Strong   lipgloss.Style
    Error    lipgloss.Style
    Warn     lipgloss.Style
    Success  lipgloss.Style
    Hint     lipgloss.Style
    Selected lipgloss.Style

    TableHeader lipgloss.Style
    TableRow    lipgloss.Style
}

func Active() Styles
```

`Active()` is the only entry point. The first call resolves dark vs
light and caches the resulting `Styles`; subsequent calls return the
same cached value via `sync.Once`.

### Why two palettes?

The REPL preserves scrollback. That means the user's terminal
background bleeds through every styled line. A palette tuned for
"vivid against black" looks washed out on a white terminal, and
vice-versa. Two palettes is the minimum that stays legible on either.

Color hexes are tuned for contrast, not for vivid accents:

| Token | Dark | Light |
|---|---|---|
| Strong fg | `#E0E0E0` | `#1A1A1A` |
| Subtle fg | `#9A9A9A` | `#555555` |
| Accent | `#7AA2F7` | `#3B5BDB` |
| Error | `#F7768E` | `#C92A2A` |
| Success | `#9ECE6A` | `#2F9E44` |

### Why a flat struct, not nested groups?

The rule from the source comment is: **"if it is not in this struct,
it does not belong in the CLI's output."** A flat struct makes it
trivial to grep call sites; a nested struct invites ad-hoc styles
that drift over time. The `TableHeader` / `TableRow` pair is the only
"group" today because the table-rendering shape was load-bearing —
add more groups only when a third member would join them.

### Who uses what

| Style | Used by |
|---|---|
| `Prompt` | `dun> ` rendered by `shell.Run` |
| `Strong` | Section titles via `shell.Strong`, `shell.Section` |
| `Subtle` | `shell.Info`, hints, `(none)` placeholder |
| `Error` | `shell.Err` |
| `Success` | `shell.Success` |
| `Hint` | The "Tab completes verbs, slugs, and flags" line in `help` |
| `Warn` | reserved — no callers in Phases 0-7 |
| `Selected` | reserved — selector primitives use their own bubbles defaults |
| `TableHeader` / `TableRow` | reserved — no table verbs shipped yet |

The reserved styles are kept in the struct so a future caller doesn't
need to invent a one-off; they're not load-bearing today.

---

## Calling pattern from a verb

The standard shape, copied from `runBuildDefault`:

```go
ok, err := selector.Confirm(ctx,
    fmt.Sprintf("Queue upgrade: %s → L%d?", kind, target),
    "Resources will be deducted immediately. You can cancel later for a 75% refund.")
if err != nil {
    if errors.Is(err, selector.ErrCancelled) {
        return nil          // soft cancel — back to the prompt
    }
    return err              // any other error — let the dispatcher render
}
if !ok {
    shell.Info(sess.Out, "aborted")
    return nil
}
// proceed with the mutation
```

Three rules to keep:

1. **Treat `ErrCancelled` as a soft signal.** Returning `nil`
   gracefully puts the user back at the prompt without an error line.
2. **`!ok` is the explicit No.** The user actively chose No (vs.
   backing out via Esc) — print a one-line `shell.Info("aborted")`
   and return.
3. **Other errors propagate.** A bubbletea program failure, a context
   cancellation that isn't a user action — let the dispatcher format
   it.

---

## When NOT to reach for a primitive

If a verb's interaction can be expressed as positional args and
`--flag`s, **don't open a transient program**:

- `profile set --handle ironfist` is better than always opening a
  Form, because the form makes scripting impossible and burns a
  full-screen redraw on a one-field edit.
- The Form opens **only** when the user types `profile set` with no
  flags — a clear "I want to be guided" signal.

The "default to a flag, open a form when none" pattern is the
load-bearing UX choice here. Don't invert it.

---

## Tests

These primitives are best tested via the verb tests that use them.
The selector package itself has no unit tests today: testing a Bubble
Tea program in isolation requires `teatest` and the test would
mostly be asserting that bubbles works (which it does). The verb
tests stub out `*api.Client` and exercise the
"Confirm-then-API-call" / "Pick-then-route" flows via the dispatcher.

If you add a new primitive, write its verb-side test first — that's
the layer that actually changes behavior.
