package shell

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// StateFileName is the basename under ~/.dun/ that holds the
// session-context snapshot. JSON, mode 0644.
const StateFileName = "state.json"

// stateFile is the on-disk shape. We tie the snapshot to the active
// (BaseURL, Email) so a `dun account switch` (or a manual
// credentials edit) doesn't re-apply someone else's scope.
type stateFile struct {
	BaseURL       string `json:"base_url,omitempty"`
	Email         string `json:"email,omitempty"`
	ServerSlug    string `json:"server_slug,omitempty"`
	WorldSlug     string `json:"world_slug,omitempty"`
	KingdomHandle string `json:"kingdom_handle,omitempty"`
}

// FileStore persists ContextSnapshot to ~/.dun/state.json. It is
// keyed by (BaseURL, Email) so context belonging to a different
// credential is ignored on load — a cheap form of multi-account
// safety.
type FileStore struct {
	BaseURL string
	Email   string
}

// NewFileStore returns a FileStore bound to the given credential.
func NewFileStore(baseURL, email string) *FileStore {
	return &FileStore{BaseURL: baseURL, Email: email}
}

// Load reads ~/.dun/state.json and returns the persisted context.
// Returns a zero snapshot with no error if the file does not exist
// or if the persisted (BaseURL, Email) does not match this store —
// both are "no relevant context yet" states.
func (s *FileStore) Load() (ContextSnapshot, error) {
	path, err := s.path()
	if err != nil {
		return ContextSnapshot{}, err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ContextSnapshot{}, nil
		}
		return ContextSnapshot{}, fmt.Errorf("shell: read state: %w", err)
	}
	var f stateFile
	if err := json.Unmarshal(buf, &f); err != nil {
		return ContextSnapshot{}, fmt.Errorf("shell: parse state: %w", err)
	}
	if f.BaseURL != s.BaseURL || f.Email != s.Email {
		// Stale or foreign — ignore quietly.
		return ContextSnapshot{}, nil
	}
	return ContextSnapshot{
		ServerSlug:    f.ServerSlug,
		WorldSlug:     f.WorldSlug,
		KingdomHandle: f.KingdomHandle,
	}, nil
}

// Save serializes the snapshot atomically (write+rename). Directory
// is created at 0700, file at 0644.
func (s *FileStore) Save(snap ContextSnapshot) error {
	path, err := s.path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("shell: mkdir state dir: %w", err)
	}
	body, err := json.MarshalIndent(stateFile{
		BaseURL:       s.BaseURL,
		Email:         s.Email,
		ServerSlug:    snap.ServerSlug,
		WorldSlug:     snap.WorldSlug,
		KingdomHandle: snap.KingdomHandle,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("shell: marshal state: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state.*.tmp")
	if err != nil {
		return fmt.Errorf("shell: create temp state: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("shell: chmod temp state: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("shell: write temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("shell: close temp state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("shell: rename temp state: %w", err)
	}
	return nil
}

func (s *FileStore) path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("shell: resolve home: %w", err)
	}
	return filepath.Join(home, ".dun", StateFileName), nil
}
