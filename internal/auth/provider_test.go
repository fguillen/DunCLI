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
	s.Delete(c.BaseURL, c.Email)
	tok, err = p.Token(context.Background())
	require.NoError(t, err)
	require.Empty(t, tok)
}
