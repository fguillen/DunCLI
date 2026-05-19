package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/auth"
)

// seedCurrentCredential writes a single credential to ~/.dun/credentials
// and marks it as current. Used by logout / keys-revoke / account-delete
// tests to skip the login flow.
func seedCurrentCredential(t *testing.T, baseURL, email, apiKey string) {
	t.Helper()
	exp, err := time.Parse(time.RFC3339, "2026-08-17T00:00:00Z")
	require.NoError(t, err)
	store := &auth.Store{}
	store.Upsert(auth.Credential{
		BaseURL:   baseURL,
		Email:     email,
		APIKey:    apiKey,
		ExpiresAt: exp,
	})
	require.NoError(t, store.SetCurrent(baseURL, email))
	require.NoError(t, store.Save())
}

func TestRunLogout_revokesAndClearsLocalEntry(t *testing.T) {
	var revokedID string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/keys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-list")
		_, err := w.Write([]byte(`{
			"keys": [
				{
					"id": "key-current",
					"name": "laptop",
					"last_used_at": "2026-05-19T10:00:00Z",
					"expires_at": "2026-08-17T00:00:00Z",
					"revoked_at": null,
					"current": true
				}
			]
		}`))
		require.NoError(t, err)
	})
	mux.HandleFunc("/v1/auth/keys/key-current", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		revokedID = "key-current"
		w.Header().Set("X-Request-Id", "req-revoke")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := withTestHomeAndBackend(t, srv.URL+"/v1")
	seedCurrentCredential(t, srv.URL+"/v1", "alice@example.com", "raw-key")

	out := &bytes.Buffer{}
	require.NoError(t, runLogout(context.Background(), out))
	require.Equal(t, "key-current", revokedID)
	require.Contains(t, out.String(), "Logged out alice@example.com")

	// The credentials file should now exist but contain no entries.
	credPath := filepath.Join(home, ".dun", auth.FileName)
	_, statErr := os.Stat(credPath)
	require.NoError(t, statErr)

	store, err := auth.LoadStore()
	require.NoError(t, err)
	require.Empty(t, store.Credentials)
	_, ok := store.CurrentCredential()
	require.False(t, ok)
}

func TestRunLogout_noSession(t *testing.T) {
	withTestHomeAndBackend(t, "http://unused.test/v1")
	out := &bytes.Buffer{}
	err := runLogout(context.Background(), out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no active session")
}

func TestRunLogout_revokeFails_keepsLocalEntry(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{
				"keys": [
					{"id": "key-cur", "name": null, "last_used_at": null, "expires_at": "2026-08-17T00:00:00Z", "revoked_at": null, "current": true}
				]
			}`))
			require.NoError(t, err)
			return
		}
	})
	mux.HandleFunc("/v1/auth/keys/key-cur", func(w http.ResponseWriter, _ *http.Request) {
		writeErrorEnvelope(t, w, http.StatusInternalServerError, "server_error", "kaboom")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	withTestHomeAndBackend(t, srv.URL+"/v1")
	seedCurrentCredential(t, srv.URL+"/v1", "bob@example.com", "raw-key")

	out := &bytes.Buffer{}
	err := runLogout(context.Background(), out)
	require.Error(t, err)

	// Local credential must NOT be deleted when revoke fails.
	store, err := auth.LoadStore()
	require.NoError(t, err)
	require.Len(t, store.Credentials, 1)
	cur, ok := store.CurrentCredential()
	require.True(t, ok)
	require.Equal(t, "bob@example.com", cur.Email)
}
