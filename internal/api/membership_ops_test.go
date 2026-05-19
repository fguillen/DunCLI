package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient_JoinServer_happyPath(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/servers/srv-1/join"))
		w.Header().Set("X-Request-Id", "req-join")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"membership_id": "mem-1",
			"server": {"id": "srv-1", "slug": "acme", "name": "Acme", "member": true}
		}`))
		require.NoError(t, err)
	})

	// Seed the resolver cache so we can verify JoinServer invalidates it.
	c.cache.servers["stale"] = "stale-id"

	got, err := c.JoinServer(context.Background(), "srv-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "mem-1", got.MembershipID)
	require.Equal(t, "acme", got.Server.Slug)
	require.True(t, got.Server.Member)

	// Cache must have been invalidated by the successful join.
	require.Empty(t, c.cache.servers, "JoinServer must invalidate the server cache on success")
}

func TestClient_JoinServer_forbidden(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-forb")
		writeEnvelope(t, w, http.StatusForbidden, "forbidden",
			"server is invite-only", 0)
	})

	_, err := c.JoinServer(context.Background(), "srv-x")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "forbidden", apiErr.Code)
}

func TestClient_UpdateOwnProfile_handleAndRealName(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/servers/srv-1/me"))
		body := readJSONBody(t, r)
		require.Equal(t, "IronFist", body["handle"])
		require.Equal(t, "Fernando G.", body["real_name"])
		w.Header().Set("X-Request-Id", "req-prof")
		writeJSON(t, w, `{
			"handle": "IronFist",
			"real_name": "Fernando G.",
			"title": null,
			"stats": {
				"rounds_played": 0, "rounds_won": 0,
				"wonders_completed": 0, "wonders_destroyed": 0,
				"peak_nodes": 0,
				"raids_launched": 0, "raids_defended": 0,
				"raids_won_offense": 0, "raids_won_defense": 0,
				"resources_looted": 0
			}
		}`)
	})

	handle := "IronFist"
	realName := "Fernando G."
	got, err := c.UpdateOwnProfile(context.Background(), "srv-1", ProfileUpdate{
		Handle:   &handle,
		RealName: &realName,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	h, ok := got.Handle.Get()
	require.True(t, ok)
	require.Equal(t, "IronFist", h)
}

func TestClient_UpdateOwnProfile_omitsUnsetFields(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		body := readJSONBody(t, r)
		// Only handle should be present in the wire body; real_name was nil.
		require.Contains(t, body, "handle")
		_, hasReal := body["real_name"]
		require.False(t, hasReal, "real_name must be omitted when ProfileUpdate.RealName is nil")
		w.Header().Set("X-Request-Id", "req-prof2")
		writeJSON(t, w, `{
			"handle": "h",
			"real_name": null,
			"title": null,
			"stats": {
				"rounds_played": 0, "rounds_won": 0,
				"wonders_completed": 0, "wonders_destroyed": 0,
				"peak_nodes": 0,
				"raids_launched": 0, "raids_defended": 0,
				"raids_won_offense": 0, "raids_won_defense": 0,
				"resources_looted": 0
			}
		}`)
	})

	h := "h"
	_, err := c.UpdateOwnProfile(context.Background(), "srv-1", ProfileUpdate{Handle: &h})
	require.NoError(t, err)
}

func TestClient_UpdateOwnProfile_handleLocked422(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-locked")
		writeEnvelope(t, w, http.StatusUnprocessableEntity, "handle_locked",
			"handle cannot be changed", 0)
	})

	h := "newHandle"
	_, err := c.UpdateOwnProfile(context.Background(), "srv-1", ProfileUpdate{Handle: &h})
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "handle_locked", apiErr.Code)
}
