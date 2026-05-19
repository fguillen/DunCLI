// Package theme exposes the Lipgloss style palette shared by the
// REPL shell, selectors, forms, and Phase 4+ verb output. Two
// variants (light + dark) are detected once at process start via
// termenv.HasDarkBackground(); from then on the same Styles value is
// returned by every call to Active().
package theme

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Styles is the flat palette every UI surface reaches for. Fields are
// chosen so a verb handler never has to invent a style ad-hoc — if it
// is not in this struct, it does not belong in the CLI's output.
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

var (
	once   sync.Once
	cached Styles
)

// Active returns the resolved Styles for the current terminal. It is
// safe to call from any goroutine; the resolution runs exactly once.
func Active() Styles {
	once.Do(func() { cached = build(termenv.HasDarkBackground()) })
	return cached
}

func build(dark bool) Styles {
	// Two palettes, dark-on-default and light-on-default. The colors
	// are picked to remain legible without relying on alt-screen — the
	// REPL preserves scrollback, so contrast against the user's
	// terminal background matters more than vivid accents.
	var (
		fgStrong, fgSubtle, fgHint     lipgloss.Color
		accent, errFg, warnFg, okFg    lipgloss.Color
		selBg, selFg                   lipgloss.Color
		tableHeaderFg, tableHeaderLine lipgloss.Color
	)
	if dark {
		fgStrong = lipgloss.Color("#E0E0E0")
		fgSubtle = lipgloss.Color("#9A9A9A")
		fgHint = lipgloss.Color("#6E6E6E")
		accent = lipgloss.Color("#7AA2F7")
		errFg = lipgloss.Color("#F7768E")
		warnFg = lipgloss.Color("#E0AF68")
		okFg = lipgloss.Color("#9ECE6A")
		selBg = lipgloss.Color("#2A2F3A")
		selFg = lipgloss.Color("#FFFFFF")
		tableHeaderFg = lipgloss.Color("#C0CAF5")
		tableHeaderLine = lipgloss.Color("#3B4261")
	} else {
		fgStrong = lipgloss.Color("#1A1A1A")
		fgSubtle = lipgloss.Color("#555555")
		fgHint = lipgloss.Color("#8A8A8A")
		accent = lipgloss.Color("#3B5BDB")
		errFg = lipgloss.Color("#C92A2A")
		warnFg = lipgloss.Color("#A26900")
		okFg = lipgloss.Color("#2F9E44")
		selBg = lipgloss.Color("#E7EAF6")
		selFg = lipgloss.Color("#1A1A1A")
		tableHeaderFg = lipgloss.Color("#1A1A1A")
		tableHeaderLine = lipgloss.Color("#C0C4CE")
	}

	return Styles{
		Prompt:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		Subtle:      lipgloss.NewStyle().Foreground(fgSubtle),
		Strong:      lipgloss.NewStyle().Foreground(fgStrong).Bold(true),
		Error:       lipgloss.NewStyle().Foreground(errFg).Bold(true),
		Warn:        lipgloss.NewStyle().Foreground(warnFg),
		Success:     lipgloss.NewStyle().Foreground(okFg),
		Hint:        lipgloss.NewStyle().Foreground(fgHint).Italic(true),
		Selected:    lipgloss.NewStyle().Foreground(selFg).Background(selBg).Bold(true),
		TableHeader: lipgloss.NewStyle().Foreground(tableHeaderFg).Bold(true).BorderBottom(true).BorderForeground(tableHeaderLine).BorderStyle(lipgloss.NormalBorder()),
		TableRow:    lipgloss.NewStyle().Foreground(fgStrong),
	}
}
