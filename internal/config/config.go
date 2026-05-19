// Package config loads the dun-cli TOML config file at ~/.dun/config.toml.
// A missing file is not an error — defaults apply. Malformed TOML is a
// typed error so the caller can distinguish "user has no config yet" from
// "user's config is broken." Phase 2 wires only BaseURL; later phases may
// add fields as the CLI surface grows.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// DefaultBaseURL is the API root including the version prefix. Matches the
// Rails backend's local dev port.
const DefaultBaseURL = "http://localhost:3000/v1"

// FileName is the config file's basename, under ~/.dun/.
const FileName = "config.toml"

// Config is the loaded user configuration.
type Config struct {
	BaseURL string
}

// Load reads ~/.dun/config.toml. Missing file returns defaults with no
// error. A present-but-unparseable file returns a wrapped error so the
// caller can show a clear message instead of falling back silently.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	v.SetDefault("base_url", DefaultBaseURL)

	if err := v.ReadInConfig(); err != nil {
		// fs.ErrNotExist covers both the wrapped os.PathError and viper's
		// own ConfigFileNotFoundError (it implements fs.ErrNotExist via
		// errors.Is on recent versions; for older versions we also
		// explicitly check the typed error below).
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist) {
			return &Config{BaseURL: DefaultBaseURL}, nil
		}
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := &Config{
		BaseURL: v.GetString("base_url"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return cfg, nil
}

// configPath returns ~/.dun/config.toml. Resolved via os.UserHomeDir per
// CLAUDE.md storage rules (no XDG, no $DUN_HOME override).
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dun", FileName), nil
}

// Stat returns os.Stat for the config file, exposed for tests that want
// to assert presence/absence without re-deriving the path.
func Stat() (fs.FileInfo, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}
