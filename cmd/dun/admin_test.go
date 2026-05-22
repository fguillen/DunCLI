package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/auth"
)

// writeAdminExchangeOK emits the 201 ExchangeResponse the backend would
// for an admin magic-link exchange — owner type "admin".
func writeAdminExchangeOK(t *testing.T, w http.ResponseWriter, email, apiKey, ownerType string) {
	t.Helper()
	w.Header().Set("X-Request-Id", "req-admin-exchange")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	body := map[string]any{
		"api_key":    apiKey,
		"expires_at": "2026-08-17T00:00:00Z",
		"owner": map[string]any{
			"id":    "owner-admin-1",
			"email": email,
			"name":  strings.Split(email, "@")[0],
			"type":  ownerType,
		},
	}
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func TestRunAdminLogin_happyPath(t *testing.T) {
	var (
		gotEmail string
		gotToken string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/admin/auth/magic_link", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotEmail = body.Email
		writeAcceptedNoBody(t, w, "req-admin-magic")
	})
	mux.HandleFunc("/v1/admin/auth/exchange", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotToken = body.Token
		writeAdminExchangeOK(t, w, "boss@example.com", "raw-admin-key", "admin")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := withTestHomeAndBackend(t, srv.URL+"/v1")

	in := bytes.NewBufferString("boss@example.com\nadmin-tok\n")
	out := &bytes.Buffer{}

	require.NoError(t, runAdminLogin(context.Background(), in, out))
	require.Equal(t, "boss@example.com", gotEmail)
	require.Equal(t, "admin-tok", gotToken)

	// Credential persisted under the admin scope; [current_admin] set.
	store, err := auth.LoadStore()
	require.NoError(t, err)
	require.Len(t, store.Credentials, 1)
	require.Equal(t, auth.ScopeAdmin, store.Credentials[0].Scope)

	cur, ok := store.CurrentAdminCredential()
	require.True(t, ok)
	require.Equal(t, "boss@example.com", cur.Email)
	require.Equal(t, "raw-admin-key", cur.APIKey)

	// The player [current] pointer is untouched.
	_, ok = store.CurrentCredential()
	require.False(t, ok)

	require.Contains(t, out.String(), "Logged in as admin boss@example.com")

	// Credentials file mode 0600.
	info, err := os.Stat(filepath.Join(home, ".dun", auth.FileName))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRunAdminLogin_wrongScope401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/admin/auth/magic_link", func(w http.ResponseWriter, _ *http.Request) {
		writeAcceptedNoBody(t, w, "req-admin-magic")
	})
	mux.HandleFunc("/v1/admin/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		writeErrorEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "wrong scope")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := withTestHomeAndBackend(t, srv.URL+"/v1")

	in := bytes.NewBufferString("boss@example.com\nstale-token\n")
	out := &bytes.Buffer{}

	err := runAdminLogin(context.Background(), in, out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")

	_, statErr := os.Stat(filepath.Join(home, ".dun", auth.FileName))
	require.True(t, os.IsNotExist(statErr), "no credentials file on failed admin login")
}

func TestRunAdminLogin_rejectsPlayerOwnerType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/admin/auth/magic_link", func(w http.ResponseWriter, _ *http.Request) {
		writeAcceptedNoBody(t, w, "req-admin-magic")
	})
	mux.HandleFunc("/v1/admin/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		// Backend issued a player-typed key — runAdminLogin must refuse it.
		writeAdminExchangeOK(t, w, "boss@example.com", "raw-player-key", "player")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	withTestHomeAndBackend(t, srv.URL+"/v1")

	in := bytes.NewBufferString("boss@example.com\nadmin-tok\n")
	out := &bytes.Buffer{}

	err := runAdminLogin(context.Background(), in, out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected an admin credential")
}

// TestAdminAndPlayerCredentialsCoexist proves a player and an admin
// login for the same email land as two distinct entries with their own
// current pointers.
func TestAdminAndPlayerCredentialsCoexist(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/magic_link", func(w http.ResponseWriter, _ *http.Request) {
		writeAcceptedNoBody(t, w, "req-magic")
	})
	mux.HandleFunc("/v1/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		writeExchangeOK(t, w, "dual@example.com", "raw-player-key")
	})
	mux.HandleFunc("/v1/admin/auth/magic_link", func(w http.ResponseWriter, _ *http.Request) {
		writeAcceptedNoBody(t, w, "req-admin-magic")
	})
	mux.HandleFunc("/v1/admin/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		writeAdminExchangeOK(t, w, "dual@example.com", "raw-admin-key", "admin")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	withTestHomeAndBackend(t, srv.URL+"/v1")

	require.NoError(t, runLogin(context.Background(),
		bytes.NewBufferString("dual@example.com\nptok\n"), &bytes.Buffer{}))
	require.NoError(t, runAdminLogin(context.Background(),
		bytes.NewBufferString("dual@example.com\natok\n"), &bytes.Buffer{}))

	store, err := auth.LoadStore()
	require.NoError(t, err)
	require.Len(t, store.Credentials, 2, "player and admin keys must be separate entries")

	player, ok := store.CurrentCredential()
	require.True(t, ok)
	require.Equal(t, "raw-player-key", player.APIKey)

	admin, ok := store.CurrentAdminCredential()
	require.True(t, ok)
	require.Equal(t, "raw-admin-key", admin.APIKey)
}
