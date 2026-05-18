// Package tui hosts the Bubble Tea program and its screens. For this scaffold
// the program only renders a splash screen; later phases stack additional
// screens on top (see TODO.md, Phase 3: TUI shell).
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI and blocks until the user quits or ctx is cancelled.
func Run(ctx context.Context) error {
	p := tea.NewProgram(
		newSplash(),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	_, err := p.Run()
	return err
}
