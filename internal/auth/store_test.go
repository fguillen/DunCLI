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

	s.Delete(c.BaseURL, c.Email)
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

	s.Delete(other.BaseURL, other.Email)

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
	s2.Delete("http://localhost:3000/v1", "alice@example.com")
	require.NoError(t, s2.Save())

	s3, err := LoadStore()
	require.NoError(t, err)
	require.Empty(t, s3.Credentials)
}
