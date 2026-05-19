// Package log wires the application's slog logger. Logs are written as JSON
// to ~/.dun/dun-cli.log so interactive output (REPL prompt, transient
// selectors) is never polluted by log lines.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LogFileName is the file written under the state dir.
const LogFileName = "dun-cli.log"

// Init opens (or creates) the log file at ~/.dun/dun-cli.log and returns a
// JSON slog.Logger plus the underlying file so callers can Close() it on
// shutdown.
//
// level accepts: "debug", "info", "warn", "error" (case-insensitive). An empty
// or unrecognized value defaults to "info".
func Init(level string) (*slog.Logger, io.Closer, error) {
	dir, err := dunDir()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve dun dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create dun dir %q: %w", dir, err)
	}

	path := filepath.Join(dir, LogFileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler), f, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// dunDir returns ~/.dun — the single canonical location for all persistent
// client state (config, credentials, logs, history, last-used context). No
// XDG env vars are consulted; the dotfolder convention matches ~/.aws/ and
// ~/.ssh/ and avoids scattering files across XDG paths.
func dunDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dun"), nil
}
