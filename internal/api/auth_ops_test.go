package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// readJSONBody is a small helper that decodes a request body for the
// auth-flow tests, which need to assert the email / token round-tripped.
func readJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	buf, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	defer r.Body.Close()
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf, &got))
	return got
}

func TestClient_RequestPlayerMagicLink_happyPath(t *testing.T) {
	var gotEmail string
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/auth/magic_link"))
		body := readJSONBody(t, r)
		gotEmail, _ = body["email"].(string)
		w.Header().Set("X-Request-Id", "req-magic")
		w.WriteHeader(http.StatusAccepted)
	})

	require.NoError(t, c.RequestPlayerMagicLink(context.Background(), "alice@example.com"))
	require.Equal(t, "alice@example.com", gotEmail)
}

func TestClient_RequestPlayerMagicLink_422(t *testing.T) {
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-422")
		writeEnvelope(t, w, http.StatusUnprocessableEntity, "param_missing", "missing email", 0)
	})

	err := c.RequestPlayerMagicLink(context.Background(), "")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "param_missing", apiErr.Code)
}

func TestClient_ExchangePlayerMagicLink_happyPath(t *testing.T) {
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/auth/exchange"))
		body := readJSONBody(t, r)
		require.Equal(t, "tok-123", body["token"])
		w.Header().Set("X-Request-Id", "req-exchange")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"api_key": "raw-key-abc",
			"expires_at": "2026-08-17T00:00:00Z",
			"owner": {
				"id": "01H...",
				"email": "alice@example.com",
				"name": "alice",
				"type": "player"
			}
		}`))
		require.NoError(t, err)
	})

	got, err := c.ExchangePlayerMagicLink(context.Background(), "tok-123")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "raw-key-abc", got.APIKey)
	require.Equal(t, "alice@example.com", got.Owner.Email)
	require.Equal(t, "player", string(got.Owner.Type))
}

func TestClient_ExchangePlayerMagicLink_expiredToken(t *testing.T) {
	// Per TODO.md Phase 2 test list: expired magic-link token surfaces
	// as the typed envelope with code=expired.
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-expired")
		writeEnvelope(t, w, http.StatusUnauthorized, "expired", "Magic link has expired", 0)
	})

	_, err := c.ExchangePlayerMagicLink(context.Background(), "stale-token")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "expired", apiErr.Code)
	require.Equal(t, "req-expired", apiErr.RequestID)
}

func TestClient_ExchangePlayerMagicLink_invalidToken(t *testing.T) {
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-invalid")
		writeEnvelope(t, w, http.StatusUnauthorized, "invalid_token", "Magic link is invalid", 0)
	})

	_, err := c.ExchangePlayerMagicLink(context.Background(), "nope")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "invalid_token", apiErr.Code)
}

func TestClient_ListPlayerApiKeys_happyPath(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/auth/keys"))
		require.Equal(t, "Bearer t", r.Header.Get("Authorization"))
		w.Header().Set("X-Request-Id", "req-keys")
		writeJSON(t, w, `{
			"keys": [
				{
					"id": "key-1",
					"name": "laptop",
					"last_used_at": "2026-05-19T10:00:00Z",
					"expires_at": "2026-08-17T00:00:00Z",
					"revoked_at": null,
					"current": true
				},
				{
					"id": "key-2",
					"name": null,
					"last_used_at": null,
					"expires_at": "2026-08-01T00:00:00Z",
					"revoked_at": "2026-05-01T00:00:00Z",
					"current": false
				}
			]
		}`)
	})

	keys, err := c.ListPlayerAPIKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Equal(t, "key-1", keys[0].ID)
	require.True(t, keys[0].Current)
	require.False(t, keys[1].Current)
}

func TestClient_ListPlayerApiKeys_unauthorized(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("expired"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-401")
		writeEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "Bearer token invalid or revoked", 0)
	})

	_, err := c.ListPlayerAPIKeys(context.Background())
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "unauthorized", apiErr.Code)
}

func TestClient_RevokePlayerApiKey_happyPath(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/auth/keys/key-xyz"))
		w.Header().Set("X-Request-Id", "req-revoke")
		w.WriteHeader(http.StatusNoContent)
	})

	require.NoError(t, c.RevokePlayerAPIKey(context.Background(), "key-xyz"))
}

func TestClient_RevokePlayerApiKey_notFound(t *testing.T) {
	// revokePlayerApiKey is the one op with NAMED error subtypes; this
	// pins down that the decode switch handles them.
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-rev-404")
		writeEnvelope(t, w, http.StatusNotFound, "not_found", "no such key", 0)
	})

	err := c.RevokePlayerAPIKey(context.Background(), "key-missing")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
}

func TestClient_RevokePlayerApiKey_unauthorized(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-rev-401")
		writeEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "wrong scope", 0)
	})

	err := c.RevokePlayerAPIKey(context.Background(), "key-id")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "unauthorized", apiErr.Code)
}

func TestClient_DeleteAccount_happyPath(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/account"))
		w.Header().Set("X-Request-Id", "req-del")
		w.WriteHeader(http.StatusNoContent)
	})

	require.NoError(t, c.DeleteAccount(context.Background()))
}

func TestClient_DeleteAccount_unauthorized(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-del-401")
		writeEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "bad token", 0)
	})

	err := c.DeleteAccount(context.Background())
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "unauthorized", apiErr.Code)
}
