package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// muxRoute matches a request against a method+path prefix and writes
// the configured JSON body. It exists so the resolver tests can stack
// multiple route handlers behind a single httptest.Server without
// importing a full router for one file.
type muxRoute struct {
	method   string
	contains string // request URL must contain this substring
	body     string
}

func mux(t *testing.T, hits *atomic.Int32, routes ...muxRoute) http.HandlerFunc {
	t.Helper()
	// Routes are checked in order; place longer/more-specific paths
	// first when multiple could match.
	return func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		for _, route := range routes {
			if r.Method == route.method && strings.Contains(r.URL.Path, route.contains) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-Id", "req-test")
				_, _ = w.Write([]byte(route.body))
				return
			}
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}

// fullPlayerStats produces a JSON literal with every PlayerStats field set
// — the schema marks all ten as required, so an incomplete fixture fails
// ogen's strict response validation.
const fullPlayerStats = `{"rounds_played":0,"rounds_won":0,"wonders_completed":0,"wonders_destroyed":0,"peak_nodes":0,"raids_launched":0,"raids_defended":0,"raids_won_offense":0,"raids_won_defense":0,"resources_looted":0}`

func TestResolveServer_cachesAfterFirstFetch(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/servers",
			body:     `{"servers":[{"id":"srv-1","slug":"acme","name":"Acme","member":true}]}`,
		},
	))

	id1, err := c.ResolveServer(context.Background(), "acme")
	require.NoError(t, err)
	require.Equal(t, "srv-1", id1)

	id2, err := c.ResolveServer(context.Background(), "acme")
	require.NoError(t, err)
	require.Equal(t, "srv-1", id2)

	require.EqualValues(t, 1, hits.Load(), "second resolve must hit the cache, not the network")
}

func TestResolveServer_missReturnsNotFound(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/servers",
			body:     `{"servers":[{"id":"srv-1","slug":"acme","name":"Acme","member":true}]}`,
		},
	))

	_, err := c.ResolveServer(context.Background(), "ghost")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
}

func TestResolveServer_invalidateRefetches(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/servers",
			body:     `{"servers":[{"id":"srv-1","slug":"acme","name":"Acme","member":true}]}`,
		},
	))

	_, err := c.ResolveServer(context.Background(), "acme")
	require.NoError(t, err)
	c.InvalidateServers()
	_, err = c.ResolveServer(context.Background(), "acme")
	require.NoError(t, err)
	require.EqualValues(t, 2, hits.Load())
}

func TestResolveWorld_scopedByServer(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Request-Id", "req-w")
		// Return a world named "spring" with a server-specific ULID so the
		// test can distinguish results across two server scopes.
		var serverID string
		switch {
		case strings.Contains(r.URL.Path, "/servers/srv-A/worlds"):
			serverID = "wld-A"
		case strings.Contains(r.URL.Path, "/servers/srv-B/worlds"):
			serverID = "wld-B"
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		body := map[string]any{
			"worlds": []map[string]any{
				{
					"id":          serverID,
					"server_id":   "srv-X",
					"name":        "Spring",
					"slug":        "spring",
					"status":      "proposed",
					"min_players": 8,
					"t0_at":       "2026-06-01T00:00:00Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(body))
	})

	idA, err := c.ResolveWorld(context.Background(), "srv-A", "spring")
	require.NoError(t, err)
	require.Equal(t, "wld-A", idA)

	idB, err := c.ResolveWorld(context.Background(), "srv-B", "spring")
	require.NoError(t, err)
	require.Equal(t, "wld-B", idB)

	// Each scope cached independently — both should be a hit on repeat.
	_, _ = c.ResolveWorld(context.Background(), "srv-A", "spring")
	_, _ = c.ResolveWorld(context.Background(), "srv-B", "spring")
	require.EqualValues(t, 2, hits.Load())
}

func TestResolveWorld_invalidateScoped(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/servers/srv-1/worlds",
			body:     `{"worlds":[{"id":"wld-1","server_id":"srv-1","name":"S","slug":"spring","status":"proposed","min_players":8,"t0_at":"2026-06-01T00:00:00Z"}]}`,
		},
	))

	_, err := c.ResolveWorld(context.Background(), "srv-1", "spring")
	require.NoError(t, err)
	c.InvalidateWorlds("srv-1")
	_, err = c.ResolveWorld(context.Background(), "srv-1", "spring")
	require.NoError(t, err)
	require.EqualValues(t, 2, hits.Load())

	// Invalidating an unrelated server does not affect srv-1's cache.
	c.InvalidateWorlds("srv-other")
	_, err = c.ResolveWorld(context.Background(), "srv-1", "spring")
	require.NoError(t, err)
	require.EqualValues(t, 2, hits.Load())
}

func TestResolveArmy_invalidateRefetches(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/kingdoms/k-1/armies",
			body:     `{"armies":[{"id":"a-1","kingdom_id":"k-1","name":"Vanguard","status":"home","location_region_id":"r-1","composition":{},"total_capacity":100}]}`,
		},
	))

	id1, err := c.ResolveArmy(context.Background(), "k-1", "Vanguard")
	require.NoError(t, err)
	require.Equal(t, "a-1", id1)

	c.InvalidateArmies("k-1")
	_, err = c.ResolveArmy(context.Background(), "k-1", "Vanguard")
	require.NoError(t, err)
	require.EqualValues(t, 2, hits.Load())
}

func TestResolveRegion_lookupByName(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/worlds/wld-1/map",
			body: `{"regions":[
				{"id":"r-1","name":"Greyhollow","terrain":"forest","position":{"x":0,"y":0},"adjacency":[],"nodes":[],"visible_armies":[]},
				{"id":"r-2","name":"Ironpeak","terrain":"mountain","position":{"x":1,"y":0},"adjacency":[],"nodes":[],"visible_armies":[]}
			]}`,
		},
	))

	id, err := c.ResolveRegion(context.Background(), "wld-1", "Ironpeak")
	require.NoError(t, err)
	require.Equal(t, "r-2", id)

	// Second resolve in same world: cache hit, no network.
	_, err = c.ResolveRegion(context.Background(), "wld-1", "Greyhollow")
	require.NoError(t, err)
	require.EqualValues(t, 1, hits.Load())
}

func TestResolvePlayer_returnsHandle(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/servers/srv-1/players/IronFist",
			body:     `{"handle":"IronFist","real_name":null,"stats":` + fullPlayerStats + `,"title":null,"joined_at":"2026-01-01T00:00:00Z"}`,
		},
	))

	got, err := c.ResolvePlayer(context.Background(), "srv-1", "IronFist")
	require.NoError(t, err)
	require.Equal(t, "IronFist", got)

	// Cached on repeat.
	_, err = c.ResolvePlayer(context.Background(), "srv-1", "IronFist")
	require.NoError(t, err)
	require.EqualValues(t, 1, hits.Load())
}

func TestResolveKingdom_readsMyKingdomFromWorld(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, StaticToken("t"), mux(t, &hits,
		muxRoute{
			method:   "GET",
			contains: "/v1/worlds/wld-1",
			body: `{
				"id":"wld-1","server_id":"srv-1","name":"Spring","slug":"spring",
				"status":"active","min_players":8,
				"t0_at":"2026-06-01T00:00:00Z","grace_closes_at":null,
				"region_count":12,"kingdom_count":4,
				"my_kingdom":{"id":"k-self","world_id":"wld-1","home_region_id":null,"stockpiles":{},"joined_at":"2026-01-01T00:00:00Z"}
			}`,
		},
	))

	id, err := c.ResolveKingdom(context.Background(), "wld-1")
	require.NoError(t, err)
	require.Equal(t, "k-self", id)

	// Cached on repeat.
	_, err = c.ResolveKingdom(context.Background(), "wld-1")
	require.NoError(t, err)
	require.EqualValues(t, 1, hits.Load())
}
