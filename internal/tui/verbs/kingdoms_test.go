package verbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// kingdomsHandler stages a server, a world, and a two-kingdom roster
// covering the rendering cases: the caller's own kingdom (is_you, with
// a wonder), and an eliminated, titled rival with no wonder.
func kingdomsHandler(t *testing.T) http.HandlerFunc {
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
		case strings.HasSuffix(r.URL.Path, "/worlds/wld-1/kingdoms"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"kingdoms": []map[string]any{
					{
						"kingdom_id": "kgd-7", "handle": "IronFist", "title": nil,
						"is_you": true, "home_region_id": "reg-1", "home_region_name": "Greyhollow",
						"nodes_controlled": 4, "ruins_claimed": 1,
						"wonder":     map[string]any{"name": "great_library", "status": "construction", "hp_pct": 42},
						"eliminated": false, "joined_at": "2026-05-20T10:00:00Z",
					},
					{
						"kingdom_id": "kgd-9", "handle": "Ragnar", "title": "[Champion of Eldoria]",
						"is_you": false, "home_region_id": "reg-4", "home_region_name": "Highmoor",
						"nodes_controlled": 2, "ruins_claimed": 0,
						"wonder":     nil,
						"eliminated": true, "joined_at": "2026-05-20T11:00:00Z",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}
}

func TestRunKingdomsList_rendersRoster(t *testing.T) {
	sess, out, _ := newTestSession(t, kingdomsHandler(t))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runKingdomsList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Kingdoms:")
	require.Contains(t, got, "IronFist (you)", "the caller's own kingdom must be marked")
	require.Contains(t, got, "Greyhollow")
	require.Contains(t, got, "nodes=4")
	require.Contains(t, got, "ruins=1")
	require.Contains(t, got, "wonder=great_library construction 42%")
	require.Contains(t, got, "Ragnar")
	require.Contains(t, got, "eliminated")
	require.Contains(t, got, "[Champion of Eldoria]")
}

func TestRunKingdomsList_requiresWorldScope(t *testing.T) {
	sess, _, _ := newTestSession(t, kingdomsHandler(t))
	sess.Context.SetServer("acme")

	err := runKingdomsList(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "world scope")
}
