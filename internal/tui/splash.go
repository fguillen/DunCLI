package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// splashModel is the initial screen shown when `dun tui` launches. It exists
// to prove the build chain end-to-end; later phases replace it with the auth /
// server-list flow.
type splashModel struct {
	width  int
	height int
}

func newSplash() splashModel { return splashModel{} }

func (splashModel) Init() tea.Cmd { return nil }

func (m splashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	hintStyle = lipgloss.NewStyle().
			Faint(true)
)

func (m splashModel) View() string {
	body := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Welcome to dun"),
		"",
		hintStyle.Render("press q to quit"),
	)

	// Until we get a WindowSizeMsg, render flush-left so headless renders
	// (snapshot tests) still produce deterministic output.
	if m.width == 0 || m.height == 0 {
		return body
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}
