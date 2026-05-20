package verbs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// kingdomHandler stages the resolver chain (servers → worlds →
// showWorld for my_kingdom) plus the Phase 7 kingdom endpoints. Extra
// path-specific handlers can be merged in via `extra`.
func kingdomHandler(t *testing.T, extra map[string]http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-"+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/servers"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{{"id": "srv-1", "slug": "acme", "name": "Acme", "member": true}},
			})
			return
		case strings.HasSuffix(r.URL.Path, "/servers/srv-1/worlds"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"worlds": []map[string]any{{
					"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
					"name": "Spring 2026", "status": "active", "min_players": 8,
					"t0_at": "2026-05-01T00:00:00Z",
				}},
			})
			return
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
				"name": "Spring 2026", "status": "active", "min_players": 8,
				"t0_at":        "2026-05-01T00:00:00Z",
				"region_count": 42, "kingdom_count": 5,
				"my_kingdom": map[string]any{
					"id": "kgd-7", "world_id": "wld-1",
					"stockpiles": map[string]any{},
					"joined_at":  "2026-05-02T00:00:00Z",
				},
			})
			return
		}
		if h, ok := extra[r.URL.Path]; ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

func setupKingdomSession(t *testing.T, extra map[string]http.HandlerFunc) (*shell.Session, *bytes.Buffer) {
	t.Helper()
	s, o, _ := newTestSession(t, kingdomHandler(t, extra))
	s.Context.SetServer("acme")
	s.Context.SetWorld("spring-2026")
	return s, o
}

// TestBuildingKinds_pinnedToGenEnum guards against spec drift: any
// new building kind added upstream must also be added to the local
// completion list. Otherwise the static completer silently lies.
func TestBuildingKinds_pinnedToGenEnum(t *testing.T) {
	want := make([]string, 0)
	for _, v := range gen.BuildingUpgradePreviewKind("").AllValues() {
		want = append(want, string(v))
	}
	require.ElementsMatch(t, want, buildingKinds,
		"buildingKinds is out of sync with gen.BuildingUpgradePreviewKind.AllValues() — regenerate the openapi client and update buildingKinds")
}

func TestRunKingdomShow_rendersDashboard(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "kgd-7", "world_id": "wld-1",
				"home_region_id": "reg-1",
				"stockpiles": {"gold": 100, "wood": 50, "stone": 25, "iron": 10},
				"warehouse_cap": 1000,
				"production_rates": {"gold": 12, "wood": 6, "stone": 3, "iron": 1},
				"joined_at": "2026-05-01T00:00:00Z",
				"buildings": [],
				"in_progress_builds": [],
				"in_progress_training": []
			}`))
		},
	}
	sess, out := setupKingdomSession(t, extra)
	require.NoError(t, runKingdomShow(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Kingdom kgd-7")
	require.Contains(t, got, "gold=100")
	require.Contains(t, got, "Production (per hour):")
	require.Contains(t, got, "Builds in progress:")
}

func TestRunBuildingsList_upgradableFlag(t *testing.T) {
	var seenQuery string
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/buildings": func(w http.ResponseWriter, r *http.Request) {
			seenQuery = r.URL.Query().Get("upgrade_possible")
			_, _ = w.Write([]byte(`{
				"kingdom_id": "kgd-7",
				"buildings": [
					{
						"kind": "town_hall", "current_level": 2, "target_level": 3,
						"at_max_level": false,
						"cost": {"gold": 100, "wood": 80, "stone": 40, "iron": 5},
						"duration_seconds": 1800,
						"tier_gates_met": true, "tier_gates_unmet": [],
						"affordable": true,
						"missing": {"gold": 0, "wood": 0, "stone": 0, "iron": 0},
						"id": "bld-1", "upgrade_possible": true, "build_order": null
					}
				]
			}`))
		},
	}
	sess, out := setupKingdomSession(t, extra)
	require.NoError(t, runBuildingsList(context.Background(), sess, nil, map[string]string{"upgradable": ""}))
	require.Equal(t, "true", seenQuery)
	require.Contains(t, out.String(), "town_hall")
	require.Contains(t, out.String(), "ready")
}

func TestRunBuildPreview_rendersAffordable(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/build/preview": func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "town_hall", r.URL.Query().Get("building"))
			_, _ = w.Write([]byte(`{
				"kind": "town_hall", "current_level": 2, "target_level": 3,
				"at_max_level": false,
				"cost": {"gold": 100, "wood": 80, "stone": 40, "iron": 5},
				"duration_seconds": 1800,
				"tier_gates_met": true, "tier_gates_unmet": [],
				"affordable": true,
				"missing": {"gold": 0, "wood": 0, "stone": 0, "iron": 0}
			}`))
		},
	}
	sess, out := setupKingdomSession(t, extra)
	require.NoError(t, runBuildPreview(context.Background(), sess, []string{"town_hall"}, nil))
	got := out.String()
	require.Contains(t, got, "town_hall upgrade preview")
	require.Contains(t, got, "level:     L2 → L3")
	require.Contains(t, got, "tier:      gates met")
	require.Contains(t, got, "affords:   yes")
}

func TestRunBuildPreview_rejectsUnknownKind(t *testing.T) {
	sess, _ := setupKingdomSession(t, nil)
	err := runBuildPreview(context.Background(), sess, []string{"castle"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown building kind")
}

func TestRunBuildCancel_resolvesKindToOrderID(t *testing.T) {
	var deletedPath string
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "kgd-7", "world_id": "wld-1",
				"stockpiles": {"gold": 0, "wood": 0, "stone": 0, "iron": 0},
				"warehouse_cap": 1000,
				"production_rates": {"gold": 0, "wood": 0, "stone": 0, "iron": 0},
				"joined_at": "2026-05-01T00:00:00Z",
				"buildings": [],
				"in_progress_builds": [
					{
						"id": "ord-9", "building_id": "bld-1", "kind": "town_hall",
						"target_level": 3,
						"started_at": "2026-05-01T00:00:00Z",
						"completes_at": "2026-05-01T00:30:00Z"
					}
				],
				"in_progress_training": []
			}`))
		},
		"/v1/kingdoms/kgd-7/build/ord-9": func(w http.ResponseWriter, r *http.Request) {
			deletedPath = r.URL.Path
			_, _ = w.Write([]byte(`{
				"id": "ord-9", "building_id": "bld-1", "kind": "town_hall",
				"target_level": 3,
				"started_at": "2026-05-01T00:00:00Z",
				"completes_at": "2026-05-01T00:30:00Z",
				"cancelled_at": "2026-05-01T00:10:00Z"
			}`))
		},
	}
	sess, _ := setupKingdomSession(t, extra)

	// The cancel verb needs interactive confirmation; tests cannot
	// drive a tea program. Exercise the kind→id resolution path
	// directly: requireKingdomID resolves the kingdom, then we verify
	// the kind→order_id lookup the verb would do.
	kingdomID, err := shared.RequireKingdomID(context.Background(), sess)
	require.NoError(t, err)
	kd, err := sess.API.ShowKingdom(context.Background(), kingdomID)
	require.NoError(t, err)
	require.Len(t, kd.InProgressBuilds, 1)
	require.Equal(t, "town_hall", string(kd.InProgressBuilds[0].Kind))
	require.Equal(t, "ord-9", kd.InProgressBuilds[0].ID)

	// CancelBuildOrder round-trips with this stage.
	ord, err := sess.API.CancelBuildOrder(context.Background(), kingdomID, "ord-9")
	require.NoError(t, err)
	require.Equal(t, "ord-9", ord.ID)
	require.Equal(t, "/v1/kingdoms/kgd-7/build/ord-9", deletedPath)
}

func TestKingdomScope_requiresWorld(t *testing.T) {
	s, _, _ := newTestSession(t, kingdomHandler(t, nil))
	s.Context.SetServer("acme")
	_, err := shared.RequireKingdomID(context.Background(), s)
	require.Error(t, err)
	require.Contains(t, err.Error(), "world scope")
}
