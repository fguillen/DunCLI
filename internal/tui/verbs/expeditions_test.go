package verbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// expeditionsHandler stages a Phase 10 baseline world: servers,
// worlds (with my_kingdom so kingdom resolution works), the world
// map (so region-name lookup works), listNodes + listRuins with a
// deliberate mix of ownership states, plus listKingdomArmies. Tests
// override `nodes` / `ruins` / `armies` as needed.
func expeditionsHandler(t *testing.T, nodes []map[string]any, ruins []map[string]any, armies []map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-"+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/servers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{
					{"id": "srv-1", "slug": "acme", "name": "Acme", "member": true},
				},
			})
		case "/v1/servers/srv-1/worlds":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"worlds": []map[string]any{{
					"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
					"name": "Spring 2026", "status": "active", "min_players": 8,
					"t0_at": "2026-05-01T00:00:00Z",
				}},
			})
		case "/v1/worlds/wld-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
				"name": "Spring 2026", "status": "active", "min_players": 8,
				"t0_at":        "2026-05-01T00:00:00Z",
				"region_count": 2, "kingdom_count": 2,
				"my_kingdom": map[string]any{
					"id": "kgd-7", "world_id": "wld-1",
					"stockpiles": map[string]any{},
					"joined_at":  "2026-05-01T00:00:00Z",
				},
			})
		case "/v1/worlds/wld-1/map":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"regions": []map[string]any{
					{
						"id": "reg-1", "name": "Greyhollow", "terrain": "forest",
						"position":  map[string]any{"x": 0.1, "y": 0.2},
						"adjacency": []string{"reg-2"},
						"nodes":     []any{},
					},
					{
						"id": "reg-2", "name": "Ironvale", "terrain": "hills",
						"position":  map[string]any{"x": 0.3, "y": 0.4},
						"adjacency": []string{"reg-1"},
						"nodes":     []any{},
					},
				},
			})
		case "/v1/worlds/wld-1/nodes":
			_ = json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
		case "/v1/worlds/wld-1/ruins":
			_ = json.NewEncoder(w).Encode(map[string]any{"ruins": ruins})
		case "/v1/kingdoms/kgd-7/armies":
			_ = json.NewEncoder(w).Encode(map[string]any{"armies": armies})
		default:
			http.NotFound(w, r)
		}
	}
}

// Four-node fixture covering every ownership case the predicates
// must discriminate:
//
//   - nd-home : home-hoard, owned by caller (kgd-7) — not capturable
//   - nd-wild : wilderness, no owner — capture target
//   - nd-mine : owned by caller (not home-hoard) — not capturable
//   - nd-foe  : owned by another kingdom (kgd-9) — capture target
//
// Each lives in a distinct region so the predicates can be observed
// as picker rows.
var nodesFixture = []map[string]any{
	{
		"id": "nd-home", "resource": "gold", "tier": "standard",
		"is_home_hoard": true, "owner_kingdom_id": "kgd-7",
		"region_id": "reg-1", "region_name": "Greyhollow",
		"base_rate": 10, "garrison": map[string]int{},
	},
	{
		"id": "nd-wild", "resource": "iron", "tier": "rich",
		"is_home_hoard": false, "owner_kingdom_id": nil,
		"region_id": "reg-2", "region_name": "Ironvale",
		"base_rate": 25, "garrison": map[string]int{"pikeman": 8},
	},
	{
		"id": "nd-mine", "resource": "wood", "tier": "poor",
		"is_home_hoard": false, "owner_kingdom_id": "kgd-7",
		"region_id": "reg-3", "region_name": "Briarwood",
		"base_rate": 5, "garrison": map[string]int{},
	},
	{
		"id": "nd-foe", "resource": "stone", "tier": "standard",
		"is_home_hoard": false, "owner_kingdom_id": "kgd-9", "owner_handle": "Ragnar",
		"region_id": "reg-4", "region_name": "Highmoor",
		"base_rate": 12, "garrison": map[string]int{},
	},
}

var ruinsFixture = []map[string]any{
	{
		"id": "rn-1", "region_id": "reg-2", "region_name": "Ironvale",
		"tier": "major", "garrison": map[string]int{"levy": 12, "archer": 4},
		"cache": map[string]int{"gold": 800, "iron": 200}, "claimed": false,
	},
	{
		"id": "rn-2", "region_id": "reg-5", "region_name": "Sunkenfields",
		"tier": "minor", "garrison": map[string]int{"levy": 4},
		"cache": map[string]int{"gold": 200}, "claimed": true,
	},
}

var armiesFixture = []map[string]any{
	{
		"id": "arm-1", "kingdom_id": "kgd-7", "name": "Garrison", "status": "home",
		"location_region_id": "reg-1", "composition": map[string]int{"levy": 12, "archer": 3},
		"total_capacity": 60,
	},
	{
		"id": "arm-2", "kingdom_id": "kgd-7", "name": "Vanguard", "status": "marching",
		"location_region_id": "reg-2", "composition": map[string]int{"knight": 4},
		"total_capacity": 80,
	},
}

// ── target predicate tests ───────────────────────────────────────────

func TestCapturableTargets_returnsWildAndForeignOwned(t *testing.T) {
	// Nodes::Attack merged into Nodes::Capture, so capturableTargets
	// returns both the wilderness region (Ironvale) and the foreign-owned
	// one (Highmoor) — but never the caller's own or home-hoard nodes.
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	targets, err := capturableTargets(context.Background(), sess, "wld-1", "kgd-7")
	require.NoError(t, err)
	require.Len(t, targets, 2)
	// Sorted by region name: Highmoor, Ironvale.
	require.Equal(t, "Highmoor", targets[0].RegionName)
	require.Equal(t, "reg-4", targets[0].RegionID)
	require.Contains(t, targets[0].description, "owner=Ragnar")
	require.Equal(t, "Ironvale", targets[1].RegionName)
	require.Equal(t, "reg-2", targets[1].RegionID)
	require.Contains(t, targets[1].description, "iron/rich")
}

func TestClaimableTargets_skipsClaimed(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	targets, err := claimableTargets(context.Background(), sess, "wld-1")
	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.Equal(t, "Ironvale", targets[0].RegionName)
	require.Contains(t, targets[0].description, "tier=major")
	require.Contains(t, targets[0].description, "gold=800")
}

// ── runExpedition outer-shell tests ──────────────────────────────────

func TestRunNodeCapture_requiresWorldScope(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme") // no world

	err := runNodeCapture(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "world scope")
}

func TestRunNodeCapture_emptyEligibleErrorsBeforePicker(t *testing.T) {
	// Keep only the caller's own home-hoard + own node — neither is a
	// capture target, so nothing is capturable.
	nodes := []map[string]any{nodesFixture[0], nodesFixture[2]}
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodes, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	err := runNodeCapture(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no capturable nodes")
}

func TestRunRuinClaim_emptyEligibleErrorsBeforePicker(t *testing.T) {
	// Keep only the claimed one — nothing claimable left.
	ruins := []map[string]any{ruinsFixture[1]}
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruins, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	err := runRuinClaim(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no unclaimed ruins")
}

func TestRunNodeCapture_rejectsUneligibleRegionArg(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	// Greyhollow holds the caller's own home-hoard — never a capture target.
	err := runNodeCapture(context.Background(), sess, []string{"Greyhollow"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not eligible")
	require.Contains(t, err.Error(), "Highmoor") // foreign-owned candidate
	require.Contains(t, err.Error(), "Ironvale") // wilderness candidate
}

func TestRunNodeCapture_rejectsExtraArgs(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	err := runNodeCapture(context.Background(), sess, []string{"a", "b"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage:")
}

func TestRunNodeCapture_regionArgMatchesCaseInsensitively(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture,
		[]map[string]any{ /* no home armies — triggers the next-step error */ }))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	// Lowercase region arg should resolve, then hit the no-home-armies
	// guard (proving target resolution reached step 4 of the wizard).
	err := runNodeCapture(context.Background(), sess, []string{"ironvale"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no home armies")
}

func TestRunRuinClaim_rejectsUneligibleRegionArg(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	// Sunkenfields ruin is claimed — not eligible.
	err := runRuinClaim(context.Background(), sess, []string{"Sunkenfields"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not eligible")
	require.Contains(t, err.Error(), "Ironvale")
}

// ── pure function tests ──────────────────────────────────────────────

func TestFilterHomeArmies_keepsOnlyHome(t *testing.T) {
	in := []gen.Army{
		{ID: "a", Name: "Alpha", Status: gen.ArmyStatusHome},
		{ID: "b", Name: "Bravo", Status: gen.ArmyStatusMarching},
		{ID: "c", Name: "Charlie", Status: gen.ArmyStatusHome},
		{ID: "d", Name: "Delta", Status: gen.ArmyStatusEngaged},
	}
	out := filterHomeArmies(in)
	require.Len(t, out, 2)
	require.Equal(t, "Alpha", out[0].Name)
	require.Equal(t, "Charlie", out[1].Name)
}

func TestTargetNames_extractsRegionNamesInOrder(t *testing.T) {
	targets := []expeditionTarget{
		{RegionName: "Alpha"},
		{RegionName: "Bravo"},
	}
	require.Equal(t, []string{"Alpha", "Bravo"}, targetNames(targets))
}

// ── tab completer tests ──────────────────────────────────────────────

func TestSuggestCapturableRegions(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	got, err := suggestCapturableRegions(context.Background(), sess, "")
	require.NoError(t, err)
	require.Equal(t, []string{"Highmoor", "Ironvale"}, got)
}

func TestSuggestClaimableRegions(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	got, err := suggestClaimableRegions(context.Background(), sess, "")
	require.NoError(t, err)
	require.Equal(t, []string{"Ironvale"}, got)
}

func TestSuggestCapturableRegions_silentOnMissingScope(t *testing.T) {
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	// No server / world scope — completer must not raise an error, it
	// just returns nothing (consistent with the rest of the suite).
	got, err := suggestCapturableRegions(context.Background(), sess, "")
	require.NoError(t, err)
	require.Empty(t, got)
}

// ── preview-block rendering ──────────────────────────────────────────

func TestRuinPreviewBlock_warnsAboutWarehouseCap(t *testing.T) {
	// The preview-line builder must include the warehouse-cap warning
	// line (per §16.11). Verifies it through the public projection.
	sess, _, _ := newTestSession(t, expeditionsHandler(t, nodesFixture, ruinsFixture, armiesFixture))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	targets, err := claimableTargets(context.Background(), sess, "wld-1")
	require.NoError(t, err)
	require.Len(t, targets, 1)
	body := strings.Join(targets[0].previewLines, "\n")
	require.Contains(t, body, "Warehouse cap is lost")
	require.Contains(t, body, "tier:       major")
}
