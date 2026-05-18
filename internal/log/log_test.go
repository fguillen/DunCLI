package log

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"nonsense", slog.LevelInfo},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			require.Equal(t, c.want, parseLevel(c.in))
		})
	}
}

// TestInit_WritesJSONUnderXDGStateHome verifies that Init creates the
// dun-cli subdirectory under $XDG_STATE_HOME and that emitted records are
// valid JSON.
func TestInit_WritesJSONUnderXDGStateHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	logger, closer, err := Init("debug")
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })

	logger.Info("hello", "k", "v")

	path := filepath.Join(dir, "dun-cli", LogFileName)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data, "log file should not be empty after a write")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(data, &rec), "log line should be valid JSON")
	require.Equal(t, "hello", rec["msg"])
	require.Equal(t, "v", rec["k"])
}
