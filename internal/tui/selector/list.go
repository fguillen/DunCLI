// Package selector hosts the transient Bubble Tea programs that the
// REPL launches when a verb needs richer input than a single line —
// list pickers (Pick) and multi-field forms (Form, in form.go).
//
// Both primitives run in alt-screen, return control to the prompt
// when done, and leave the final selection for the calling verb to
// print into scrollback as plain text. Verbs never import bubbletea
// directly; they reach for these helpers instead.
package selector

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Item is one row in a Pick() call. Title is the primary label;
// Description is shown in a subtle style underneath; Value is the
// opaque payload the caller cares about (e.g. a slug or ULID).
type Item struct {
	Title       string
	Description string
	Value       string
}

// ErrCancelled is returned when the user dismisses the picker with
// Esc / Ctrl-C / q. Callers should treat it as a soft "user backed
// out" signal, not an error to log.
var ErrCancelled = errors.New("selector: cancelled by user")

// pickItem is the bubbles/list adapter wrapping our public Item.
type pickItem struct{ Item }

func (i pickItem) Title() string       { return i.Item.Title }
func (i pickItem) Description() string { return i.Item.Description }
func (i pickItem) FilterValue() string { return i.Item.Title }

// pickModel is the transient tea.Model for one Pick() invocation.
type pickModel struct {
	list     list.Model
	chosen   *Item
	quitting bool
}

func (m pickModel) Init() tea.Cmd { return nil }

func (m pickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Leave a couple of lines for the title + status; the rest is
		// list real estate.
		m.list.SetSize(msg.Width, msg.Height-2)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(pickItem); ok {
				m.chosen = &it.Item
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickModel) View() string {
	if m.quitting {
		return ""
	}
	return m.list.View()
}

// Pick opens an interactive list picker titled `title` over `items`
// and returns the user's choice. On Esc/Ctrl-C/q it returns
// ErrCancelled. ctx is honored: when it is cancelled the picker
// exits and the error is propagated. The caller is responsible for
// printing any post-selection summary back to scrollback.
func Pick(ctx context.Context, title string, items []Item) (Item, error) {
	if len(items) == 0 {
		return Item{}, errors.New("selector: no items to pick from")
	}
	li := make([]list.Item, 0, len(items))
	for _, it := range items {
		li = append(li, pickItem{Item: it})
	}
	d := list.NewDefaultDelegate()
	m := pickModel{list: list.New(li, d, 0, 0)}
	m.list.Title = title
	m.list.Styles.Title = lipgloss.NewStyle().Bold(true).Padding(0, 1)

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	final, err := p.Run()
	if err != nil {
		return Item{}, fmt.Errorf("selector: %w", err)
	}
	res, ok := final.(pickModel)
	if !ok {
		return Item{}, fmt.Errorf("selector: unexpected model type %T", final)
	}
	if res.chosen == nil {
		return Item{}, ErrCancelled
	}
	return *res.chosen, nil
}
