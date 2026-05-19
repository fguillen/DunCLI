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
	"github.com/fguillen/dun-cli/internal/config"
)

// withTestHomeAndBackend stubs HOME, writes a config.toml pointing at
// the given backend URL, and returns the resolved HOME for assertions.
// Every login/logout/keys/account test must call this before any of the
// helpers in client.go run, since they all read from ~/.dun/.
func withTestHomeAndBackend(t *testing.T, backendURL string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".dun")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	body := `base_url = "` + backendURL + `"` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644))
	return home
}

// writeAcceptedNoBody is the magic-link request handler — 202 with no body.
func writeAcceptedNoBody(_ *testing.T, w http.ResponseWriter, requestID string) {
	w.Header().Set("X-Request-Id", requestID)
	w.WriteHeader(http.StatusAccepted)
}

// writeExchangeOK emits the 201 ExchangeResponse the backend would.
func writeExchangeOK(t *testing.T, w http.ResponseWriter, email, apiKey string) {
	t.Helper()
	w.Header().Set("X-Request-Id", "req-exchange")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	body := map[string]any{
		"api_key":    apiKey,
		"expires_at": "2026-08-17T00:00:00Z",
		"owner": map[string]any{
			"id":    "owner-1",
			"email": email,
			"name":  strings.Split(email, "@")[0],
			"type":  "player",
		},
	}
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

// writeErrorEnvelope is the mini-envelope helper for cmd-level tests.
// It duplicates internal/api/client_test.go's writeEnvelope to avoid an
// import cycle between cmd and api test code.
func writeErrorEnvelope(t *testing.T, w http.ResponseWriter, status int, code, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", "req-test")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}))
}

func TestRunLogin_happyPath(t *testing.T) {
	var (
		gotEmail string
		gotToken string
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/magic_link", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotEmail = body.Email
		writeAcceptedNoBody(t, w, "req-magic")
	})
	mux.HandleFunc("/v1/auth/exchange", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotToken = body.Token
		writeExchangeOK(t, w, "alice@example.com", "raw-key-abc")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := withTestHomeAndBackend(t, srv.URL+"/v1")

	in := bytes.NewBufferString("alice@example.com\ntok-from-email\n")
	out := &bytes.Buffer{}

	require.NoError(t, runLogin(context.Background(), in, out))
	require.Equal(t, "alice@example.com", gotEmail)
	require.Equal(t, "tok-from-email", gotToken)

	// Credentials file is created with mode 0600.
	credPath := filepath.Join(home, ".dun", auth.FileName)
	info, err := os.Stat(credPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Store contains the new credential and Current points at it.
	store, err := auth.LoadStore()
	require.NoError(t, err)
	require.Len(t, store.Credentials, 1)
	require.Equal(t, "alice@example.com", store.Credentials[0].Email)
	require.Equal(t, "raw-key-abc", store.Credentials[0].APIKey)
	cur, ok := store.CurrentCredential()
	require.True(t, ok)
	require.Equal(t, "alice@example.com", cur.Email)

	require.Contains(t, out.String(), "Logged in as alice@example.com")
}

func TestRunLogin_expiredMagicLinkToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/magic_link", func(w http.ResponseWriter, _ *http.Request) {
		writeAcceptedNoBody(t, w, "req-magic")
	})
	mux.HandleFunc("/v1/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		writeErrorEnvelope(t, w, http.StatusUnauthorized, "expired", "Magic link has expired")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	home := withTestHomeAndBackend(t, srv.URL+"/v1")

	in := bytes.NewBufferString("alice@example.com\nstale-token\n")
	out := &bytes.Buffer{}

	err := runLogin(context.Background(), in, out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")

	// No credentials file should have been written.
	_, statErr := os.Stat(filepath.Join(home, ".dun", auth.FileName))
	require.True(t, os.IsNotExist(statErr), "no credentials file on failed login")
}

func TestRunLogin_wrongScope401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/magic_link", func(w http.ResponseWriter, _ *http.Request) {
		writeAcceptedNoBody(t, w, "req-magic")
	})
	mux.HandleFunc("/v1/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		writeErrorEnvelope(t, w, http.StatusUnauthorized, "wrong_scope", "Magic link is for admin scope")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	withTestHomeAndBackend(t, srv.URL+"/v1")

	in := bytes.NewBufferString("admin@example.com\nadmin-tok\n")
	out := &bytes.Buffer{}

	err := runLogin(context.Background(), in, out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong_scope")
}

func TestRunLogin_emptyEmail(t *testing.T) {
	withTestHomeAndBackend(t, "http://unused.test/v1")

	in := bytes.NewBufferString("\n")
	out := &bytes.Buffer{}

	err := runLogin(context.Background(), in, out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "email is required")
}
