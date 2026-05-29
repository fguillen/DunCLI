package verbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// eventsHandler stages a server, a world (with my_kingdom so kingdom
// resolution works), and the kingdom's event timeline. `events` lets a
// test override the feed; nil yields a two-event default in backend
// order (oldest first, newest last).
func eventsHandler(t *testing.T, events []map[string]any) http.HandlerFunc {
	t.Helper()
	if events == nil {
		events = []map[string]any{
			{
				"occurred_at": "2026-05-20T09:00:00Z",
				"type":        "build",
				"description": `Building "barracks" finished upgrading to L3.`,
			},
			{
				"occurred_at": "2026-05-20T11:30:00Z",
				"type":        "battle",
				"description": "Defended Greyhollow against Ragnar.",
			},
		}
	}
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
		case "/v1/kingdoms/kgd-7/events":
			_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
		default:
			http.NotFound(w, r)
		}
	}
}

func TestRunEvents_rendersTimelineInOrder(t *testing.T) {
	sess, out, _ := newTestSession(t, eventsHandler(t, nil))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runEvents(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Recent events:")
	require.Contains(t, got, "2026-05-20 09:00 UTC")
	require.Contains(t, got, "build")
	require.Contains(t, got, `Building "barracks" finished upgrading to L3.`)
	require.Contains(t, got, "battle")
	require.Contains(t, got, "Defended Greyhollow against Ragnar.")
	// Oldest-first: the build line must precede the battle line.
	require.Less(t, strings.Index(got, "barracks"), strings.Index(got, "Defended"),
		"events must render oldest-first (newest last)")
}

func TestRunEvents_emptyFeedRendersNone(t *testing.T) {
	sess, out, _ := newTestSession(t, eventsHandler(t, []map[string]any{}))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runEvents(context.Background(), sess, nil, nil))
	require.Contains(t, out.String(), "(none)")
}

func TestRunEvents_acceptsCountArg(t *testing.T) {
	sess, _, _ := newTestSession(t, eventsHandler(t, nil))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	require.NoError(t, runEvents(context.Background(), sess, []string{"3"}, nil))
}

func TestRunEvents_rejectsBadCount(t *testing.T) {
	for _, arg := range []string{"abc", "0", "-1", "200"} {
		sess, _, _ := newTestSession(t, eventsHandler(t, nil))
		sess.Context.SetServer("acme")
		sess.Context.SetWorld("spring-2026")

		err := runEvents(context.Background(), sess, []string{arg}, nil)
		require.Error(t, err, "arg %q must be rejected", arg)
		require.Contains(t, err.Error(), "between 1 and 100")
	}
}

func TestRunEvents_rejectsExtraArgs(t *testing.T) {
	sess, _, _ := newTestSession(t, eventsHandler(t, nil))
	sess.Context.SetServer("acme")
	sess.Context.SetWorld("spring-2026")

	err := runEvents(context.Background(), sess, []string{"3", "extra"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage:")
}

func TestRunEvents_requiresKingdomScope(t *testing.T) {
	sess, _, _ := newTestSession(t, eventsHandler(t, nil))
	sess.Context.SetServer("acme") // no world → no kingdom

	err := runEvents(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "world scope")
}
