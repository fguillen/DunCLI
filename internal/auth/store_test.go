package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sampleCredential(t *testing.T) Credential {
	t.Helper()
	exp, err := time.Parse(time.RFC3339, "2026-08-17T12:00:00Z")
	require.NoError(t, err)
	return Credential{
		BaseURL:   "http://localhost:3000/v1",
		Email:     "alice@example.com",
		APIKey:    "raw-key-abc",
		ExpiresAt: exp,
	}
}

func TestLoadStore_missingFile_returnsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s, err := LoadStore()
	require.NoError(t, err)
	require.Empty(t, s.Credentials)
	_, ok := s.CurrentCredential()
	require.False(t, ok)
}

func TestStore_SaveAndLoad_roundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))
	require.NoError(t, s.Save())

	// File mode 0600.
	path := filepath.Join(home, ".dun", FileName)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "credentials file must be mode 0600")

	// Parent dir mode 0700.
	dirInfo, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "~/.dun must be mode 0700")

	// Round-trip.
	loaded, err := LoadStore()
	require.NoError(t, err)
	require.Len(t, loaded.Credentials, 1)
	require.Equal(t, c.BaseURL, loaded.Credentials[0].BaseURL)
	require.Equal(t, c.Email, loaded.Credentials[0].Email)
	require.Equal(t, c.APIKey, loaded.Credentials[0].APIKey)
	require.True(t, c.ExpiresAt.Equal(loaded.Credentials[0].ExpiresAt))

	got, ok := loaded.CurrentCredential()
	require.True(t, ok)
	require.Equal(t, c.Email, got.Email)
}

func TestStore_Upsert_replacesByKey(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)

	c2 := c
	c2.APIKey = "new-key"
	s.Upsert(c2)

	require.Len(t, s.Credentials, 1)
	require.Equal(t, "new-key", s.Credentials[0].APIKey)
}

func TestStore_Upsert_appendsDifferentEmail(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)

	c2 := c
	c2.Email = "bob@example.com"
	c2.APIKey = "bob-key"
	s.Upsert(c2)

	require.Len(t, s.Credentials, 2)
}

func TestStore_SetCurrent_rejectsUnknown(t *testing.T) {
	s := &Store{}
	err := s.SetCurrent("http://nope", "nobody@example.com")
	require.Error(t, err)
}

func TestStore_Delete_clearsCurrentIfMatches(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))

	s.Delete(c.BaseURL, c.Email, ScopePlayer)
	require.Empty(t, s.Credentials)
	_, ok := s.CurrentCredential()
	require.False(t, ok)
}

func TestStore_Delete_keepsCurrentWhenDifferent(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)
	other := c
	other.Email = "other@example.com"
	s.Upsert(other)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))

	s.Delete(other.BaseURL, other.Email, ScopePlayer)

	require.Len(t, s.Credentials, 1)
	got, ok := s.CurrentCredential()
	require.True(t, ok)
	require.Equal(t, c.Email, got.Email)
}

func TestStore_Clear_wipesEverything(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))

	s.Clear()
	require.Empty(t, s.Credentials)
	_, ok := s.CurrentCredential()
	require.False(t, ok)
}

// sampleAdminCredential is the admin-scope sibling of sampleCredential
// — same (base_url, email), distinct scope.
func sampleAdminCredential(t *testing.T) Credential {
	t.Helper()
	c := sampleCredential(t)
	c.APIKey = "raw-key-admin"
	c.Scope = ScopeAdmin
	return c
}

func TestStore_playerAndAdminCredentialsCoexist(t *testing.T) {
	s := &Store{}
	player := sampleCredential(t)
	admin := sampleAdminCredential(t)
	s.Upsert(player)
	s.Upsert(admin)

	// Same (base_url, email) but distinct scopes — two entries, no clobber.
	require.Len(t, s.Credentials, 2)

	gotP, ok := s.Get(player.BaseURL, player.Email, ScopePlayer)
	require.True(t, ok)
	require.Equal(t, "raw-key-abc", gotP.APIKey)

	gotA, ok := s.Get(admin.BaseURL, admin.Email, ScopeAdmin)
	require.True(t, ok)
	require.Equal(t, "raw-key-admin", gotA.APIKey)
}

func TestStore_emptyScopeNormalizesToPlayer(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t) // Scope left empty
	s.Upsert(c)

	got, ok := s.Get(c.BaseURL, c.Email, ScopePlayer)
	require.True(t, ok, "an empty scope must be reachable as a player credential")
	require.Equal(t, c.APIKey, got.APIKey)

	_, ok = s.Get(c.BaseURL, c.Email, ScopeAdmin)
	require.False(t, ok, "an empty scope must not match the admin scope")
}

func TestStore_currentAdminPointer_independentOfPlayer(t *testing.T) {
	s := &Store{}
	s.Upsert(sampleCredential(t))
	s.Upsert(sampleAdminCredential(t))
	require.NoError(t, s.SetCurrent("http://localhost:3000/v1", "alice@example.com"))
	require.NoError(t, s.SetCurrentAdmin("http://localhost:3000/v1", "alice@example.com"))

	p, ok := s.CurrentCredential()
	require.True(t, ok)
	require.Equal(t, "raw-key-abc", p.APIKey)

	a, ok := s.CurrentAdminCredential()
	require.True(t, ok)
	require.Equal(t, "raw-key-admin", a.APIKey)
}

func TestStore_SetCurrentAdmin_rejectsPlayerOnlyEmail(t *testing.T) {
	s := &Store{}
	s.Upsert(sampleCredential(t)) // player only
	err := s.SetCurrentAdmin("http://localhost:3000/v1", "alice@example.com")
	require.Error(t, err, "SetCurrentAdmin must require an admin-scope credential")
}

func TestStore_Delete_scopedToOneSurface(t *testing.T) {
	s := &Store{}
	s.Upsert(sampleCredential(t))
	s.Upsert(sampleAdminCredential(t))
	require.NoError(t, s.SetCurrent("http://localhost:3000/v1", "alice@example.com"))
	require.NoError(t, s.SetCurrentAdmin("http://localhost:3000/v1", "alice@example.com"))

	// Deleting the admin entry leaves the player entry + pointer intact.
	s.Delete("http://localhost:3000/v1", "alice@example.com", ScopeAdmin)
	require.Len(t, s.Credentials, 1)
	_, ok := s.CurrentCredential()
	require.True(t, ok, "player [current] must survive an admin delete")
	_, ok = s.CurrentAdminCredential()
	require.False(t, ok, "admin [current_admin] must be cleared by the admin delete")
}

func TestStore_scope_roundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	s := &Store{}
	s.Upsert(sampleCredential(t))
	s.Upsert(sampleAdminCredential(t))
	require.NoError(t, s.SetCurrentAdmin("http://localhost:3000/v1", "alice@example.com"))
	require.NoError(t, s.Save())

	loaded, err := LoadStore()
	require.NoError(t, err)
	require.Len(t, loaded.Credentials, 2)

	got, ok := loaded.CurrentAdminCredential()
	require.True(t, ok)
	require.Equal(t, ScopeAdmin, got.Scope)
	require.Equal(t, "raw-key-admin", got.APIKey)
}

func TestLoadStore_malformedFile_returnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".dun")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte("[unclosed"), 0o600))

	_, err := LoadStore()
	require.Error(t, err)
}

func TestStore_Save_overwritesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	s1 := &Store{}
	s1.Upsert(sampleCredential(t))
	require.NoError(t, s1.SetCurrent("http://localhost:3000/v1", "alice@example.com"))
	require.NoError(t, s1.Save())

	s2, err := LoadStore()
	require.NoError(t, err)
	s2.Delete("http://localhost:3000/v1", "alice@example.com", ScopePlayer)
	require.NoError(t, s2.Save())

	s3, err := LoadStore()
	require.NoError(t, err)
	require.Empty(t, s3.Credentials)
}
