package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient_ShowWorldArchive_returnsArchive(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/archive"),
			"unexpected path %s", r.URL.Path)
		w.Header().Set("X-Request-Id", "req-archive")
		writeJSON(t, w, `{
			"world_id": "wld-1",
			"winner_kingdom_id": "kgd-7",
			"wonder_name": "sky_tower",
			"ended_at": "2026-05-21T12:00:00Z",
			"frozen_state": {
				"ended_at": "2026-05-21T12:00:00Z",
				"regions": [
					{"id": "reg-1", "name": "Greyhollow", "terrain": "forest",
					 "position": {"x": 0.1, "y": 0.2}, "is_hub": false,
					 "node_ids": ["nd-1"]}
				],
				"kingdoms": [
					{"id": "kgd-7", "handle": "IronFist", "home_region_id": "reg-1",
					 "final_stockpiles": {"gold": 0, "wood": 0, "stone": 0, "iron": 0,
					                      "checkpoint_at": "2026-05-21T12:00:00Z"},
					 "building_levels": {"town_hall": 5},
					 "peak_nodes": 8, "final_node_count": 6,
					 "joined_at": "2026-05-01T00:00:00Z"}
				],
				"wonder": {
					"kingdom_id": "kgd-7", "name": "sky_tower",
					"status": "completed", "hp": 10000, "target_hp": 10000
				},
				"battles_count": 42,
				"caravans_count": 17,
				"nodes_count": 9
			}
		}`)
	})

	got, err := c.ShowWorldArchive(context.Background(), "wld-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "wld-1", got.WorldID)
	v, ok := got.WinnerKingdomID.Get()
	require.True(t, ok)
	require.Equal(t, "kgd-7", v)
	require.Equal(t, 42, got.FrozenState.BattlesCount)
	require.Len(t, got.FrozenState.Kingdoms, 1)
}

func TestClient_ShowWorldArchive_notFound(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-archive-404")
		writeEnvelope(t, w, http.StatusNotFound, "not_found",
			"world is still live", 0)
	})

	_, err := c.ShowWorldArchive(context.Background(), "wld-x")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
	require.Equal(t, "req-archive-404", apiErr.RequestID)
}

func TestClient_ShowHallOfFame_allBoards(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/servers/srv-1/hall-of-fame"))
		require.Empty(t, r.URL.Query().Get("kind"),
			"empty kind must omit the query param")
		w.Header().Set("X-Request-Id", "req-hof")
		writeJSON(t, w, `{
			"server_id": "srv-1",
			"leaderboards": {
				"champions": {
					"snapshot_at": "2026-05-20T00:00:00Z",
					"entries": [
						{"player_profile_id": "ppl-1", "handle": "IronFist",
						 "score": 3, "secondary": 1, "title": "Champion"}
					]
				},
				"wreckers": {"snapshot_at": null, "entries": []},
				"warlords": {"snapshot_at": null, "entries": []},
				"veterans": {"snapshot_at": null, "entries": []}
			}
		}`)
	})

	got, err := c.ShowHallOfFame(context.Background(), "srv-1", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "srv-1", got.ServerID)
	require.Len(t, got.Leaderboards, 4)
	require.Len(t, got.Leaderboards["champions"].Entries, 1)
}

func TestClient_ShowHallOfFame_kindFilter(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "warlords", r.URL.Query().Get("kind"))
		w.Header().Set("X-Request-Id", "req-hof-kind")
		writeJSON(t, w, `{
			"server_id": "srv-1",
			"leaderboards": {
				"warlords": {"snapshot_at": "2026-05-20T00:00:00Z", "entries": []}
			}
		}`)
	})

	got, err := c.ShowHallOfFame(context.Background(), "srv-1", "warlords")
	require.NoError(t, err)
	require.Len(t, got.Leaderboards, 1)
	_, ok := got.Leaderboards["warlords"]
	require.True(t, ok)
}

func TestClient_ShowHallOfFame_forbidden(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-hof-403")
		writeEnvelope(t, w, http.StatusForbidden, "forbidden",
			"not a member of this server", 0)
	})

	_, err := c.ShowHallOfFame(context.Background(), "srv-1", "")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "forbidden", apiErr.Code)
}
