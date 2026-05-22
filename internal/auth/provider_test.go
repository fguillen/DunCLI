package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileProvider_nilStore_returnsEmpty(t *testing.T) {
	p := NewFileProvider(nil)
	tok, err := p.Token(context.Background())
	require.NoError(t, err)
	require.Empty(t, tok)
}

func TestFileProvider_emptyStore_returnsEmpty(t *testing.T) {
	p := NewFileProvider(&Store{})
	tok, err := p.Token(context.Background())
	require.NoError(t, err)
	require.Empty(t, tok)
}

func TestFileProvider_returnsCurrentAPIKey(t *testing.T) {
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))

	p := NewFileProvider(s)
	tok, err := p.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, c.APIKey, tok)
}

func TestFileProvider_reflectsLiveStoreMutation(t *testing.T) {
	// Phase 1 security adapter calls Token() per request; the provider
	// must reflect store mutations without being rebuilt.
	s := &Store{}
	p := NewFileProvider(s)

	tok, err := p.Token(context.Background())
	require.NoError(t, err)
	require.Empty(t, tok)

	c := sampleCredential(t)
	s.Upsert(c)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))

	tok, err = p.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, c.APIKey, tok)

	// After Delete, Token returns empty again.
	s.Delete(c.BaseURL, c.Email, ScopePlayer)
	tok, err = p.Token(context.Background())
	require.NoError(t, err)
	require.Empty(t, tok)
}

func TestAdminFileProvider_selectsAdminScope(t *testing.T) {
	// A store holding both a player and an admin credential for the same
	// email: each provider must yield its own scope's key.
	s := &Store{}
	player := sampleCredential(t)
	admin := sampleAdminCredential(t)
	s.Upsert(player)
	s.Upsert(admin)
	require.NoError(t, s.SetCurrent(player.BaseURL, player.Email))
	require.NoError(t, s.SetCurrentAdmin(admin.BaseURL, admin.Email))

	playerTok, err := NewFileProvider(s).Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, player.APIKey, playerTok)

	adminTok, err := NewAdminFileProvider(s).Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, admin.APIKey, adminTok)
}

func TestAdminFileProvider_noAdminCredential_returnsEmpty(t *testing.T) {
	// A store with only a player credential: the admin provider has no
	// [current_admin] pointer to follow and returns ("", nil).
	s := &Store{}
	c := sampleCredential(t)
	s.Upsert(c)
	require.NoError(t, s.SetCurrent(c.BaseURL, c.Email))

	tok, err := NewAdminFileProvider(s).Token(context.Background())
	require.NoError(t, err)
	require.Empty(t, tok)
}
