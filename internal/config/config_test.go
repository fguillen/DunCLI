package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeConfig is a small helper that writes the given TOML to
// $HOME/.dun/config.toml. Tests set HOME via t.Setenv.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	home := os.Getenv("HOME")
	require.NotEmpty(t, home, "test must set HOME before calling writeConfig")
	dir := filepath.Join(home, ".dun")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644))
}

func TestLoad_missingFile_returnsDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL, cfg.BaseURL)
}

func TestLoad_overridesBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeConfig(t, `base_url = "https://api.example.test/v1"`+"\n")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "https://api.example.test/v1", cfg.BaseURL)
}

func TestLoad_emptyBaseURL_fallsBackToDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeConfig(t, `base_url = ""`+"\n")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL, cfg.BaseURL)
}

func TestLoad_malformedFile_returnsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeConfig(t, "this is not = valid = toml\n[unclosed\n")

	_, err := Load()
	require.Error(t, err)
}
