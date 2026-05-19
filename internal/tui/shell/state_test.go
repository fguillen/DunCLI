package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileStore_roundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	s := NewFileStore("http://localhost:3000/v1", "alice@example.com")
	snap := ContextSnapshot{
		ServerSlug:    "acme",
		WorldSlug:     "spring-2026",
		KingdomHandle: "IronFist",
	}
	require.NoError(t, s.Save(snap))

	got, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, snap, got)

	// File mode 0644 on the saved state.
	info, err := os.Stat(filepath.Join(dir, ".dun", StateFileName))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestFileStore_missingFile_returnsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := NewFileStore("u", "e")
	got, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, ContextSnapshot{}, got)
}

func TestFileStore_foreignCredential_isIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Save under one credential.
	a := NewFileStore("u1", "alice@example.com")
	require.NoError(t, a.Save(ContextSnapshot{ServerSlug: "acme"}))

	// Load under a different credential — must come back empty.
	b := NewFileStore("u1", "bob@example.com")
	got, err := b.Load()
	require.NoError(t, err)
	require.Equal(t, ContextSnapshot{}, got)
}

func TestContext_SetServerClearsDependents(t *testing.T) {
	c := NewContext()
	c.SetServer("acme")
	c.SetWorld("spring-2026")
	c.SetKingdomHandle("IronFist")

	c.SetServer("beta")
	require.Equal(t, "beta", c.ServerSlug())
	require.Empty(t, c.WorldSlug())
	require.Empty(t, c.KingdomHandle())
}
