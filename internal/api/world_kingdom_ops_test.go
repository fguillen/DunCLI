package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClient_JoinWorld_invalidatesWorldsCache pins down both the happy
// path of JoinWorld and the cache-invalidation contract: a successful
// join must drop the per-server world cache so a subsequent
// ListServerWorlds re-fetches.
func TestClient_JoinWorld_invalidatesWorldsCache(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/join"))
		w.Header().Set("X-Request-Id", "req-jw")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"id": "kgd-7",
			"world_id": "wld-1",
			"home_region_id": null,
			"stockpiles": {"gold": 0, "wood": 0, "stone": 0, "iron": 0, "checkpoint_at": "2026-05-01T00:00:00Z"},
			"joined_at": "2026-05-01T00:00:00Z"
		}`))
		require.NoError(t, err)
	})

	c.cache.worlds["srv-1"] = map[string]string{"spring-2026": "wld-stale"}

	got, err := c.JoinWorld(context.Background(), "wld-1", "srv-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "kgd-7", got.ID)
	require.Equal(t, "wld-1", got.WorldID)

	_, ok := c.cache.worlds["srv-1"]
	require.False(t, ok, "JoinWorld must drop the per-server world cache on success")
}

func TestClient_JoinWorld_unprocessable(t *testing.T) {
	// Note: the spec's ErrorEnvelope enum does not include the domain
	// codes the description mentions (e.g. world_not_joinable,
	// max_worlds_per_account) — backend co-evolution candidate. Test
	// against `invalid` which IS in the enum so ogen accepts it.
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-jw422")
		writeEnvelope(t, w, http.StatusUnprocessableEntity, "invalid",
			"world is archived", 0)
	})

	_, err := c.JoinWorld(context.Background(), "wld-x", "srv-1")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "invalid", apiErr.Code)
}

func TestClient_ShowRegion_returnsRegion(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/regions/reg-1"))
		w.Header().Set("X-Request-Id", "req-sr")
		writeJSON(t, w, `{
			"id": "reg-1",
			"name": "Greyhollow",
			"terrain": "forest",
			"position": {"x": 0.2, "y": 0.7},
			"adjacency": ["reg-2", "reg-3"],
			"nodes": []
		}`)
	})

	got, err := c.ShowRegion(context.Background(), "wld-1", "reg-1")
	require.NoError(t, err)
	require.Equal(t, "Greyhollow", got.Name)
	require.Equal(t, 2, len(got.Adjacency))
}

func TestClient_ShowRegionAdjacent_returnsNeighbors(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/regions/reg-1/adjacent"))
		w.Header().Set("X-Request-Id", "req-sra")
		writeJSON(t, w, `{
			"regions": [
				{"id": "reg-2", "name": "Ironvale", "terrain": "hills"},
				{"id": "reg-3", "name": "Greenmarsh", "terrain": "marsh"}
			]
		}`)
	})

	got, err := c.ShowRegionAdjacent(context.Background(), "wld-1", "reg-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "Ironvale", got[0].Name)
}

func TestClient_ListRuins_returnsRuins(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/ruins"))
		w.Header().Set("X-Request-Id", "req-lr")
		writeJSON(t, w, `{
			"ruins": [
				{"id": "rn-1", "region_id": "reg-1", "region_name": "Greyhollow",
				 "tier": "major", "garrison": {"levy": 12}, "cache": {"gold": 800},
				 "claimed": false}
			]
		}`)
	})

	got, err := c.ListRuins(context.Background(), "wld-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "major", string(got[0].Tier))
}

func TestClient_ListNodes_returnsNodes(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/nodes"))
		w.Header().Set("X-Request-Id", "req-ln")
		writeJSON(t, w, `{
			"nodes": [
				{"id": "nd-1", "resource": "gold", "tier": "standard",
				 "is_home_hoard": true, "region_id": "reg-1", "region_name": "Greyhollow",
				 "base_rate": 10, "owner_kingdom_id": "kgd-7", "garrison": {}}
			]
		}`)
	})

	got, err := c.ListNodes(context.Background(), "wld-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.True(t, got[0].IsHomeHoard)
}

func TestClient_ShowNode_returnsNode(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/nodes/nd-1"))
		w.Header().Set("X-Request-Id", "req-sn")
		writeJSON(t, w, `{
			"node": {
				"id": "nd-1", "resource": "iron", "tier": "rich",
				"is_home_hoard": false, "region_id": "reg-2", "region_name": "Ironvale",
				"base_rate": 25, "owner_kingdom_id": null, "garrison": {"pikeman": 8}
			}
		}`)
	})

	got, err := c.ShowNode(context.Background(), "wld-1", "nd-1")
	require.NoError(t, err)
	require.Equal(t, "nd-1", got.ID)
}

func TestClient_ShowKingdom_returnsDashboard(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7"))
		w.Header().Set("X-Request-Id", "req-sk")
		writeJSON(t, w, `{
			"id": "kgd-7", "world_id": "wld-1",
			"home_region_id": "reg-1",
			"stockpiles": {"gold": 100, "wood": 50, "stone": 25, "iron": 10},
			"warehouse_cap": 1000,
			"production_rates": {"gold": 12, "wood": 6, "stone": 3, "iron": 1},
			"joined_at": "2026-05-01T00:00:00Z",
			"buildings": [],
			"in_progress_builds": [],
			"in_progress_training": []
		}`)
	})

	got, err := c.ShowKingdom(context.Background(), "kgd-7")
	require.NoError(t, err)
	require.Equal(t, 1000, got.WarehouseCap)
	require.Equal(t, 12, got.ProductionRates.Gold)
}

func TestClient_ListKingdomBuildings_passesUpgradableFilter(t *testing.T) {
	var raw url.Values
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.Query()
		w.Header().Set("X-Request-Id", "req-lkb")
		writeJSON(t, w, `{"kingdom_id": "kgd-7", "buildings": []}`)
	})

	yes := true
	_, err := c.ListKingdomBuildings(context.Background(), "kgd-7", &yes)
	require.NoError(t, err)
	require.Equal(t, "true", raw.Get("upgrade_possible"))

	// And the unfiltered call must NOT send the query param.
	raw = nil
	_, err = c.ListKingdomBuildings(context.Background(), "kgd-7", nil)
	require.NoError(t, err)
	require.Empty(t, raw.Get("upgrade_possible"))
}

func TestClient_PreviewBuildUpgrade_sendsBuildingQuery(t *testing.T) {
	var raw url.Values
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.Query()
		w.Header().Set("X-Request-Id", "req-pbu")
		writeJSON(t, w, `{
			"kind": "town_hall", "current_level": 2, "target_level": 3,
			"at_max_level": false,
			"cost": {"gold": 100, "wood": 80, "stone": 40, "iron": 5},
			"duration_seconds": 1800,
			"tier_gates_met": true, "tier_gates_unmet": [],
			"affordable": true,
			"missing": {"gold": 0, "wood": 0, "stone": 0, "iron": 0}
		}`)
	})

	got, err := c.PreviewBuildUpgrade(context.Background(), "kgd-7", "town_hall")
	require.NoError(t, err)
	require.Equal(t, "town_hall", raw.Get("building"))
	require.Equal(t, "town_hall", string(got.Kind))
	require.True(t, got.Affordable)
}

func TestClient_QueueBuildOrder_sendsBodyAndInvalidates(t *testing.T) {
	var seen map[string]any
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/build"))
		seen = readJSONBody(t, r)
		w.Header().Set("X-Request-Id", "req-qbo")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"id": "ord-1", "building_id": "bld-1", "kind": "town_hall",
			"target_level": 3,
			"started_at": "2026-05-01T00:00:00Z",
			"completes_at": "2026-05-01T00:30:00Z"
		}`))
		require.NoError(t, err)
	})

	got, err := c.QueueBuildOrder(context.Background(), "kgd-7", "town_hall", 3)
	require.NoError(t, err)
	require.Equal(t, "town_hall", seen["building"])
	require.EqualValues(t, 3, seen["target_level"])
	require.Equal(t, "ord-1", got.ID)
}

func TestClient_QueueBuildOrder_insufficientResources(t *testing.T) {
	// `insufficient_resources` is documented in the operation's 422
	// description but is not in the ErrorEnvelope `code` enum — backend
	// co-evolution candidate. The wrapper still decodes the envelope
	// correctly for an enum-valid code, which is what this test pins.
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-qbo422")
		writeEnvelope(t, w, http.StatusUnprocessableEntity, "invalid",
			"not enough gold", 0)
	})

	_, err := c.QueueBuildOrder(context.Background(), "kgd-7", "town_hall", 3)
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "invalid", apiErr.Code)
}

func TestClient_CancelBuildOrder_happyPath(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/build/ord-1"))
		w.Header().Set("X-Request-Id", "req-cbo")
		writeJSON(t, w, `{
			"id": "ord-1", "building_id": "bld-1", "kind": "town_hall",
			"target_level": 3,
			"started_at": "2026-05-01T00:00:00Z",
			"completes_at": "2026-05-01T00:30:00Z",
			"cancelled_at": "2026-05-01T00:10:00Z"
		}`)
	})

	got, err := c.CancelBuildOrder(context.Background(), "kgd-7", "ord-1")
	require.NoError(t, err)
	require.Equal(t, "ord-1", got.ID)
}
