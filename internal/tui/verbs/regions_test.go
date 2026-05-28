package verbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/stretchr/testify/require"
)

// regionsHandler stages a small worldful of regions, ruins, and nodes
// that the Phase 6 verb tests share.
func regionsHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-"+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/servers"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{
					{"id": "srv-1", "slug": "acme", "name": "Acme", "member": true},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/servers/srv-1/worlds"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"worlds": []map[string]any{
					{
						"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
						"name": "Spring 2026", "status": "active", "min_players": 8,
						"t0_at": "2026-05-01T00:00:00Z",
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1/map"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"regions": []map[string]any{
					{
						"id": "reg-1", "name": "Greyhollow", "terrain": "forest",
						"position":         map[string]any{"x": 0.1, "y": 0.2},
						"adjacency":        []string{"reg-2"},
						"owner_kingdom_id": "kgd-7", "owner_handle": "IronFist",
						"nodes": []map[string]any{{"id": "nd-1", "resource": "gold", "tier": "standard", "is_home_hoard": true}},
					},
					{
						"id": "reg-2", "name": "Ironvale", "terrain": "hills",
						"position":  map[string]any{"x": 0.3, "y": 0.4},
						"adjacency": []string{"reg-1"},
						"nodes":     []map[string]any{},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1/regions/reg-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "reg-1", "name": "Greyhollow", "terrain": "forest",
				"position":  map[string]any{"x": 0.1, "y": 0.2},
				"adjacency": []string{"reg-2"},
				"nodes": []map[string]any{
					{
						"id": "nd-1", "resource": "gold", "tier": "standard",
						"is_home_hoard": true, "owner_kingdom_id": "kgd-7",
						"region_id": "reg-1", "region_name": "Greyhollow",
						"base_rate": 10, "garrison": map[string]int{},
					},
				},
				"owner_kingdom_id": "kgd-7",
				"owner_handle":     "IronFist",
			})
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1/regions/reg-1/adjacent"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"regions": []map[string]any{
					{"id": "reg-2", "name": "Ironvale", "terrain": "hills"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1/ruins"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ruins": []map[string]any{
					{
						"id": "rn-1", "region_id": "reg-2", "region_name": "Ironvale",
						"tier": "major", "garrison": map[string]int{"levy": 12, "archer": 4},
						"cache": map[string]int{"gold": 800}, "claimed": false,
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1/nodes"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"nodes": []map[string]any{
					{
						"id": "nd-1", "resource": "gold", "tier": "standard",
						"is_home_hoard": true, "owner_kingdom_id": "kgd-7", "owner_handle": "IronFist",
						"region_id": "reg-1", "region_name": "Greyhollow",
						"base_rate": 10, "garrison": map[string]int{},
					},
					{
						"id": "nd-2", "resource": "iron", "tier": "rich",
						"is_home_hoard": false, "owner_kingdom_id": nil,
						"region_id": "reg-2", "region_name": "Ironvale",
						"base_rate": 25, "garrison": map[string]int{"pikeman": 8},
					},
					{
						"id": "nd-3", "resource": "stone", "tier": "standard",
						"is_home_hoard": true, "owner_kingdom_id": nil,
						"region_id": "reg-3", "region_name": "Mossgrove",
						"base_rate": 10, "garrison": map[string]int{"levy": 25, "archer": 10, "pikeman": 5},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}
}

func TestRunMap_listsRegionsWithGlyph(t *testing.T) {
	sess, out, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runMap(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Map:")
	require.Contains(t, got, "Greyhollow")
	require.Contains(t, got, "Ironvale")
	require.Contains(t, got, "T  ", "forest glyph must appear")
	require.Contains(t, got, "^  ", "hills glyph must appear")
	require.Contains(t, got, "adj=Ironvale")
	require.Contains(t, got, "IronFist", "owned region must show the owner handle")
	require.Contains(t, got, "(wild)", "unclaimed region must show (wild)")
}

func TestRunRegionShow_includesAdjacent(t *testing.T) {
	sess, out, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runRegionShow(context.Background(), sess, []string{"Greyhollow"}, nil))
	got := out.String()
	require.Contains(t, got, "Greyhollow")
	require.Contains(t, got, "owner:     IronFist")
	require.Contains(t, got, "adjacent:  Ironvale")
}

func TestRunRuinsList_rendersUnclaimed(t *testing.T) {
	sess, out, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runRuinsList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Ruins:")
	require.Contains(t, got, "Ironvale")
	require.Contains(t, got, "tier=major")
	require.Contains(t, got, "unclaimed")
	require.Contains(t, got, "garrison=archer=4, levy=12")
}

func TestRunNodesList_unfiltered(t *testing.T) {
	sess, out, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runNodesList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Nodes:")
	require.Contains(t, got, "Greyhollow")
	require.Contains(t, got, "Ironvale")
	require.Contains(t, got, "Mossgrove")
	require.Contains(t, got, "owner=IronFist (home-hoard)", "a held home hoard must name its holder, not just the tag")
	require.Contains(t, got, "owner=unclaimed (home-hoard)", "an unclaimed home hoard must read as unclaimed")
	require.Contains(t, got, "owner=wild", "a wilderness node must read as wild")
}

func TestRunNodesList_wildOnly(t *testing.T) {
	sess, out, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runNodesList(context.Background(), sess, nil, map[string]string{"owner": "wild"}))
	got := out.String()
	require.NotContains(t, got, "Greyhollow", "home-hoard node must be excluded by --owner wild")
	require.Contains(t, got, "Ironvale")
}

func TestRunNodesList_invalidOwner(t *testing.T) {
	sess, _, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	err := runNodesList(context.Background(), sess, nil, map[string]string{"owner": "bogus"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --owner")
}

func TestRunMap_requiresWorldScope(t *testing.T) {
	sess, _, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	err := runMap(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "world scope")
}

func TestRunNodeShow_byRegionName(t *testing.T) {
	sess, out, _ := newTestSession(t, regionsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runNodeShow(context.Background(), sess, []string{"Greyhollow"}, nil))
	got := out.String()
	require.Contains(t, got, "Node nd-1")
	require.Contains(t, got, "region:    Greyhollow")
}

func TestNodeOwnerLabel(t *testing.T) {
	tests := []struct {
		name string
		node gen.Node
		wild string
		want string
	}{
		{
			name: "held home hoard names its holder and keeps the tag",
			node: gen.Node{IsHomeHoard: true, OwnerHandle: gen.NewOptNilString("IronFist")},
			wild: "wild",
			want: "IronFist (home-hoard)",
		},
		{
			name: "held home hoard falls back to the kingdom id when no handle",
			node: gen.Node{IsHomeHoard: true, OwnerKingdomID: gen.NewOptNilString("kgd-7")},
			wild: "wild",
			want: "kgd-7 (home-hoard)",
		},
		{
			name: "unclaimed home hoard reads as unclaimed",
			node: gen.Node{IsHomeHoard: true},
			wild: "wild",
			want: "unclaimed (home-hoard)",
		},
		{
			name: "owned ordinary node shows the handle alone",
			node: gen.Node{OwnerHandle: gen.NewOptNilString("Ragnar")},
			wild: "wild",
			want: "Ragnar",
		},
		{
			name: "wilderness node uses the caller's wild label",
			node: gen.Node{},
			wild: "(wild)",
			want: "(wild)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, nodeOwnerLabel(tt.node, tt.wild))
		})
	}
}
