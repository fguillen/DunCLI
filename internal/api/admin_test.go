package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// adminExchangeBody is a well-formed ExchangeResponse JSON body with an
// admin-typed owner.
const adminExchangeBody = `{
  "api_key": "admin-key-xyz",
  "expires_at": "2026-08-17T12:00:00Z",
  "owner": {"id": "adm-1", "email": "boss@example.com", "name": "Boss", "type": "admin"}
}`

func TestClient_adminMagicLink_roundTrip(t *testing.T) {
	var (
		sawEmail string
		sawToken string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/admin/auth/magic_link", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		sawEmail = body.Email
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /v1/admin/auth/exchange", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		sawToken = body.Token
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-adm-x")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(adminExchangeBody))
	})

	c, _ := newTestClient(t, StaticToken(""), mux.ServeHTTP)

	require.NoError(t, c.RequestAdminMagicLink(context.Background(), "boss@example.com"))
	require.Equal(t, "boss@example.com", sawEmail)

	resp, err := c.ExchangeAdminMagicLink(context.Background(), "tok-123")
	require.NoError(t, err)
	require.Equal(t, "tok-123", sawToken)
	require.Equal(t, "admin-key-xyz", resp.APIKey)
	require.Equal(t, "boss@example.com", resp.Owner.Email)
	require.Equal(t, "admin", string(resp.Owner.Type))
}

func TestClient_exchangeAdminMagicLink_wrongScope401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/admin/auth/exchange", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-adm-401")
		writeEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "wrong scope", 0)
	})
	c, _ := newTestClient(t, StaticToken(""), mux.ServeHTTP)

	_, err := c.ExchangeAdminMagicLink(context.Background(), "bad-token")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "unauthorized", apiErr.Code)
	require.Equal(t, "req-adm-401", apiErr.RequestID)
}

func TestClient_adminKeys_listAndRevoke(t *testing.T) {
	var (
		sawAuth   string
		revokedID string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/admin/auth/keys", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		writeJSON(t, w, `{"keys": [
		  {"id": "kadm-1", "name": "laptop", "last_used_at": null,
		   "expires_at": "2026-08-17T12:00:00Z", "revoked_at": null, "current": true}
		]}`)
	})
	mux.HandleFunc("DELETE /v1/admin/auth/keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		revokedID = r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})

	c, _ := newTestClient(t, StaticToken("admin-key-xyz"), mux.ServeHTTP)

	keys, err := c.ListAdminAPIKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "kadm-1", keys[0].ID)
	require.True(t, keys[0].Current)
	require.Equal(t, "Bearer admin-key-xyz", sawAuth, "admin keys call must carry the admin bearer token")

	require.NoError(t, c.RevokeAdminAPIKey(context.Background(), "kadm-1"))
	require.Equal(t, "kadm-1", revokedID)
}

func TestClient_revokeAdminApiKey_notFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /v1/admin/auth/keys/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-adm-404")
		writeEnvelope(t, w, http.StatusNotFound, "not_found", "no such key", 0)
	})
	c, _ := newTestClient(t, StaticToken("admin-key-xyz"), mux.ServeHTTP)

	err := c.RevokeAdminAPIKey(context.Background(), "kadm-missing")
	require.Error(t, err)
	require.Equal(t, "not_found", AsError(err).Code)
}

func TestClient_resolveAdminServer_cachesAfterFirstList(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/admin/servers", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writeJSON(t, w, `{"servers": [
		  {"id": "srv-1", "slug": "acme", "name": "Acme",
		   "max_concurrent_worlds": 2, "max_worlds_per_account": 1, "owner_admin_id": "adm-1"}
		]}`)
	})
	c, _ := newTestClient(t, StaticToken("admin-key-xyz"), mux.ServeHTTP)

	id, err := c.ResolveAdminServer(context.Background(), "acme")
	require.NoError(t, err)
	require.Equal(t, "srv-1", id)

	// Second lookup is served from the cache — no second HTTP call.
	id, err = c.ResolveAdminServer(context.Background(), "acme")
	require.NoError(t, err)
	require.Equal(t, "srv-1", id)
	require.Equal(t, int32(1), calls.Load(), "ResolveAdminServer must memoize the list call")

	// Unknown slug surfaces as a typed not_found. A miss always
	// re-lists (mirrors the player ResolveServer), so this is call 2.
	_, err = c.ResolveAdminServer(context.Background(), "ghost")
	require.Error(t, err)
	require.Equal(t, "not_found", AsError(err).Code)
	require.Equal(t, int32(2), calls.Load())

	// InvalidateAdminServers forces a refetch — call 3.
	c.InvalidateAdminServers()
	_, err = c.ResolveAdminServer(context.Background(), "acme")
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load(), "InvalidateAdminServers must drop the cache")
}

func TestClient_resolveAdminWorld_perServerCache(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/admin/servers/{serverId}/worlds", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "srv-1", r.PathValue("serverId"))
		writeJSON(t, w, `{"worlds": [
		  {"id": "wld-1", "server_id": "srv-1", "name": "Spring 2026", "slug": "spring-2026",
		   "seed": "abc", "status": "active", "min_players": 8, "auto_cancel_after_hours": 72,
		   "t0_at": "2026-05-01T00:00:00Z"}
		]}`)
	})
	c, _ := newTestClient(t, StaticToken("admin-key-xyz"), mux.ServeHTTP)

	id, err := c.ResolveAdminWorld(context.Background(), "srv-1", "spring-2026")
	require.NoError(t, err)
	require.Equal(t, "wld-1", id)

	_, err = c.ResolveAdminWorld(context.Background(), "srv-1", "spring-2026")
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load(), "ResolveAdminWorld must memoize per server")

	c.InvalidateAdminWorlds("srv-1")
	_, err = c.ResolveAdminWorld(context.Background(), "srv-1", "spring-2026")
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load(), "InvalidateAdminWorlds must drop the cache")
}
