package auth

import "context"

// FileProvider is the api.TokenProvider implementation backed by the
// credentials Store. It holds a pointer to a Store plus the scope it
// serves, and reads the matching current-credential pointer on every
// Token() call so a login/logout that mutates the same Store is picked
// up without rebuilding the api.Client.
//
// Returning ("", nil) is valid per api.TokenProvider's contract — it
// signals "no credentials available" and lets the backend respond 401,
// which surfaces as a typed api.Error for the caller to handle.
type FileProvider struct {
	store *Store
	scope string
}

// NewFileProvider wraps the given Store for the player surface. A nil
// Store is allowed and results in Token() always returning ("", nil) —
// handy for tests that want an "unauthenticated" client without
// constructing an empty Store.
func NewFileProvider(s *Store) *FileProvider {
	return &FileProvider{store: s, scope: ScopePlayer}
}

// NewAdminFileProvider wraps the given Store for the admin surface.
// Token() yields the credential pointed at by [current_admin].
func NewAdminFileProvider(s *Store) *FileProvider {
	return &FileProvider{store: s, scope: ScopeAdmin}
}

// Token implements api.TokenProvider.
func (p *FileProvider) Token(_ context.Context) (string, error) {
	if p == nil || p.store == nil {
		return "", nil
	}
	var (
		c  Credential
		ok bool
	)
	if p.scope == ScopeAdmin {
		c, ok = p.store.CurrentAdminCredential()
	} else {
		c, ok = p.store.CurrentCredential()
	}
	if !ok {
		return "", nil
	}
	return c.APIKey, nil
}
