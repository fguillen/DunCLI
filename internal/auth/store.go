// Package auth persists player credentials at ~/.dun/credentials (TOML,
// mode 0600) and provides the file-backed api.TokenProvider that reads
// from it. See [PRODUCT.md] for the storage rationale; key points:
//
//   - One file holds every (base_url, email) credential the user has
//     logged in with — supports multiple base URLs (dev, prod) and
//     multiple emails per base URL.
//   - A separate [current] table points at the active credential; the
//     TokenProvider reads that on every Token() call so a re-login is
//     picked up without rebuilding the api.Client.
//   - No OS keychain; no XDG; no $DUN_HOME override.
package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// FileName is the credentials file basename, under ~/.dun/.
const FileName = "credentials"

// Credential is one persisted login.
type Credential struct {
	BaseURL   string    `toml:"base_url"`
	Email     string    `toml:"email"`
	APIKey    string    `toml:"api_key"`
	ExpiresAt time.Time `toml:"expires_at"`
}

// currentRef is the on-disk [current] pointer. We persist only the key
// fields so renames/edits don't need to keep two copies of the api_key in
// sync.
type currentRef struct {
	BaseURL string `toml:"base_url,omitempty"`
	Email   string `toml:"email,omitempty"`
}

// Store is the in-memory mirror of ~/.dun/credentials. Mutations operate
// on the slice; Save() is what writes back to disk.
type Store struct {
	Current     currentRef   `toml:"current,omitempty"`
	Credentials []Credential `toml:"credentials,omitempty"`
}

// LoadStore reads ~/.dun/credentials. Missing file returns an empty
// Store with no error so first-run callers can Upsert + Save without
// pre-checking. Present-but-unparseable returns a typed error.
func LoadStore() (*Store, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, fmt.Errorf("resolve credentials path: %w", err)
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Store{}, nil
		}
		return nil, fmt.Errorf("read credentials %q: %w", path, err)
	}

	var s Store
	if err := toml.Unmarshal(buf, &s); err != nil {
		return nil, fmt.Errorf("parse credentials %q: %w", path, err)
	}
	return &s, nil
}

// Save serializes the Store and writes ~/.dun/credentials atomically
// (write to a sibling temp file then rename). Directory is created at
// 0700, file at 0600.
func (s *Store) Save() error {
	path, err := credentialsPath()
	if err != nil {
		return fmt.Errorf("resolve credentials path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(path), err)
	}

	body, err := toml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".credentials.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp credentials: %w", err)
	}
	tmpPath := tmp.Name()
	// On any error after temp creation, clean up.
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp credentials: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp credentials: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp credentials: %w", err)
	}
	return nil
}

// Upsert replaces (or inserts) a credential keyed by (BaseURL, Email).
func (s *Store) Upsert(c Credential) {
	for i := range s.Credentials {
		if s.Credentials[i].BaseURL == c.BaseURL && s.Credentials[i].Email == c.Email {
			s.Credentials[i] = c
			return
		}
	}
	s.Credentials = append(s.Credentials, c)
}

// Get returns the credential for the given key, if any.
func (s *Store) Get(baseURL, email string) (Credential, bool) {
	for _, c := range s.Credentials {
		if c.BaseURL == baseURL && c.Email == email {
			return c, true
		}
	}
	return Credential{}, false
}

// CurrentCredential returns the credential pointed at by [current], if
// any. Returns (_, false) when there is no current pointer or when the
// pointer references a credential that is no longer present.
func (s *Store) CurrentCredential() (Credential, bool) {
	if s.Current.BaseURL == "" || s.Current.Email == "" {
		return Credential{}, false
	}
	return s.Get(s.Current.BaseURL, s.Current.Email)
}

// SetCurrent points [current] at the (baseURL, email) pair. The pair
// must already be present in Credentials.
func (s *Store) SetCurrent(baseURL, email string) error {
	if _, ok := s.Get(baseURL, email); !ok {
		return fmt.Errorf("auth: SetCurrent: no credential for %s %s", baseURL, email)
	}
	s.Current = currentRef{BaseURL: baseURL, Email: email}
	return nil
}

// Delete removes the credential matching (baseURL, email), if present.
// If the deleted entry was the current one, [current] is cleared.
func (s *Store) Delete(baseURL, email string) {
	out := s.Credentials[:0]
	for _, c := range s.Credentials {
		if c.BaseURL == baseURL && c.Email == email {
			continue
		}
		out = append(out, c)
	}
	s.Credentials = out
	if s.Current.BaseURL == baseURL && s.Current.Email == email {
		s.Current = currentRef{}
	}
}

// Clear wipes every credential and the [current] pointer. Used by
// `dun account delete` (server has revoked everything, so the file is
// moot).
func (s *Store) Clear() {
	s.Credentials = nil
	s.Current = currentRef{}
}

// credentialsPath returns ~/.dun/credentials.
func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dun", FileName), nil
}
