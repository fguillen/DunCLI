package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

// TestSplash_QuitsAndRendersWelcome drives the splash model headlessly via
// teatest and asserts the rendered output contains the welcome line and that
// pressing `q` causes a clean quit.
func TestSplash_QuitsAndRendersWelcome(t *testing.T) {
	tm := teatest.NewTestModel(t, newSplash(), teatest.WithInitialTermSize(80, 24))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	out := readAll(t, tm)
	require.Contains(t, out, "Welcome to dun")
	require.Contains(t, out, "press q to quit")
}

func readAll(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	b, err := readAllBytes(tm)
	require.NoError(t, err)
	return string(b)
}

func readAllBytes(tm *teatest.TestModel) ([]byte, error) {
	var sb strings.Builder
	r := tm.FinalOutput(nil)
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return []byte(sb.String()), nil
}
