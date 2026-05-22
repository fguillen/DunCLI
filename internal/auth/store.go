// Package auth persists credentials at ~/.dun/credentials (TOML, mode
// 0600) and provides the file-backed api.TokenProvider that reads from
// it. See [PRODUCT.md] for the storage rationale; key points:
//
//   - One file holds every (base_url, email, scope) credential the
//     user has logged in with — supports multiple base URLs (dev,
//     prod), multiple emails per base URL, and both a player and an
//     admin key for the same email (the scope discriminator).
//   - Two pointer tables select the active credential per scope:
//     [current] for the player surface, [current_admin] for the admin
//     surface. They are independent so the `dun>` and `dun-admin>`
//     shells never clobber each other. The TokenProvider reads its
//     scope's pointer on every Token() call so a re-login is picked up
//     without rebuilding the api.Client.
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

// Credential scopes. A credential with no scope on disk is a player
// credential — empty normalizes to ScopePlayer so files written before
// the admin track load unchanged.
const (
	ScopePlayer = "player"
	ScopeAdmin  = "admin"
)

// normalizeScope maps an on-disk scope string to its canonical form.
// The zero value ("") is a player credential.
func normalizeScope(s string) string {
	if s == ScopeAdmin {
		return ScopeAdmin
	}
	return ScopePlayer
}

// Credential is one persisted login. Scope discriminates the player
// surface from the admin surface so both can coexist for the same
// (base_url, email); an empty Scope is treated as ScopePlayer.
type Credential struct {
	BaseURL   string    `toml:"base_url"`
	Email     string    `toml:"email"`
	APIKey    string    `toml:"api_key"`
	ExpiresAt time.Time `toml:"expires_at"`
	Scope     string    `toml:"scope,omitempty"`
}

// currentRef is an on-disk pointer table ([current] / [current_admin]).
// We persist only the key fields so renames/edits don't need to keep
// two copies of the api_key in sync.
type currentRef struct {
	BaseURL string `toml:"base_url,omitempty"`
	Email   string `toml:"email,omitempty"`
}

// Store is the in-memory mirror of ~/.dun/credentials. Mutations operate
// on the slice; Save() is what writes back to disk.
type Store struct {
	Current      currentRef   `toml:"current,omitempty"`
	CurrentAdmin currentRef   `toml:"current_admin,omitempty"`
	Credentials  []Credential `toml:"credentials,omitempty"`
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

// Upsert replaces (or inserts) a credential keyed by
// (BaseURL, Email, Scope). A player and an admin credential for the
// same (BaseURL, Email) are distinct entries and never clobber.
func (s *Store) Upsert(c Credential) {
	scope := normalizeScope(c.Scope)
	for i := range s.Credentials {
		e := s.Credentials[i]
		if e.BaseURL == c.BaseURL && e.Email == c.Email && normalizeScope(e.Scope) == scope {
			s.Credentials[i] = c
			return
		}
	}
	s.Credentials = append(s.Credentials, c)
}

// Get returns the credential for the given (baseURL, email, scope) key,
// if any. An empty scope matches player credentials.
func (s *Store) Get(baseURL, email, scope string) (Credential, bool) {
	want := normalizeScope(scope)
	for _, c := range s.Credentials {
		if c.BaseURL == baseURL && c.Email == email && normalizeScope(c.Scope) == want {
			return c, true
		}
	}
	return Credential{}, false
}

// CurrentCredential returns the player credential pointed at by
// [current], if any. Returns (_, false) when there is no current
// pointer or when it references a credential that is no longer present.
func (s *Store) CurrentCredential() (Credential, bool) {
	if s.Current.BaseURL == "" || s.Current.Email == "" {
		return Credential{}, false
	}
	return s.Get(s.Current.BaseURL, s.Current.Email, ScopePlayer)
}

// CurrentAdminCredential returns the admin credential pointed at by
// [current_admin], if any. The admin pointer is independent of the
// player [current] pointer.
func (s *Store) CurrentAdminCredential() (Credential, bool) {
	if s.CurrentAdmin.BaseURL == "" || s.CurrentAdmin.Email == "" {
		return Credential{}, false
	}
	return s.Get(s.CurrentAdmin.BaseURL, s.CurrentAdmin.Email, ScopeAdmin)
}

// SetCurrent points [current] at the (baseURL, email) player
// credential. The pair must already be present in Credentials.
func (s *Store) SetCurrent(baseURL, email string) error {
	if _, ok := s.Get(baseURL, email, ScopePlayer); !ok {
		return fmt.Errorf("auth: SetCurrent: no player credential for %s %s", baseURL, email)
	}
	s.Current = currentRef{BaseURL: baseURL, Email: email}
	return nil
}

// SetCurrentAdmin points [current_admin] at the (baseURL, email) admin
// credential. The pair must already be present in Credentials.
func (s *Store) SetCurrentAdmin(baseURL, email string) error {
	if _, ok := s.Get(baseURL, email, ScopeAdmin); !ok {
		return fmt.Errorf("auth: SetCurrentAdmin: no admin credential for %s %s", baseURL, email)
	}
	s.CurrentAdmin = currentRef{BaseURL: baseURL, Email: email}
	return nil
}

// Delete removes the credential matching (baseURL, email, scope), if
// present. If the deleted entry was the current one for its scope, the
// matching pointer ([current] or [current_admin]) is cleared.
func (s *Store) Delete(baseURL, email, scope string) {
	want := normalizeScope(scope)
	out := s.Credentials[:0]
	for _, c := range s.Credentials {
		if c.BaseURL == baseURL && c.Email == email && normalizeScope(c.Scope) == want {
			continue
		}
		out = append(out, c)
	}
	s.Credentials = out
	if want == ScopePlayer && s.Current.BaseURL == baseURL && s.Current.Email == email {
		s.Current = currentRef{}
	}
	if want == ScopeAdmin && s.CurrentAdmin.BaseURL == baseURL && s.CurrentAdmin.Email == email {
		s.CurrentAdmin = currentRef{}
	}
}

// Clear wipes every credential and both pointer tables. Used by
// `dun account delete` (the server has revoked everything, so the
// whole file is moot — admin entries included).
func (s *Store) Clear() {
	s.Credentials = nil
	s.Current = currentRef{}
	s.CurrentAdmin = currentRef{}
}

// credentialsPath returns ~/.dun/credentials.
func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dun", FileName), nil
}
