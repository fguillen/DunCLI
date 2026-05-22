package verbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// worldsHandler returns a handler that serves a small fixed set of
// world endpoints for the in-scope server "acme". Each test composes
// it via the path routing below.
func worldsHandler(t *testing.T, extra map[string]http.HandlerFunc) http.HandlerFunc {
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
			return
		case strings.HasSuffix(r.URL.Path, "/servers/srv-1/worlds"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"worlds": []map[string]any{
					{
						"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
						"name": "Spring 2026", "status": "grace", "min_players": 8,
						"t0_at": "2026-05-01T00:00:00Z",
					},
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

func TestRunWorldsList_rendersFlatList(t *testing.T) {
	sess, out, _ := newTestSession(t, worldsHandler(t, nil))
	sess.Context.SetServer("acme")

	require.NoError(t, runWorldsList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Worlds:")
	require.Contains(t, got, "spring-2026")
	require.Contains(t, got, "grace")
	require.Contains(t, got, "Spring 2026")
}

func TestRunWorldsList_requiresServerScope(t *testing.T) {
	sess, _, _ := newTestSession(t, worldsHandler(t, nil))
	err := runWorldsList(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunWorldShow_printsHeadlineFields(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
				"name": "Spring 2026", "status": "grace", "min_players": 8,
				"t0_at": "2026-05-01T00:00:00Z",
				"grace_closes_at": "2026-05-03T00:00:00Z",
				"region_count": 42, "kingdom_count": 5,
				"my_kingdom": {
					"id": "kgd-7", "world_id": "wld-1",
					"home_region_id": "reg-2",
					"stockpiles": {"gold": 0},
					"joined_at": "2026-05-02T00:00:00Z"
				}
			}`))
		},
	}
	sess, out, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	require.NoError(t, runWorldShow(context.Background(), sess, []string{"spring-2026"}, nil))
	got := out.String()
	require.Contains(t, got, "Spring 2026")
	require.Contains(t, got, "status:    grace")
	require.Contains(t, got, "regions:   42")
	require.Contains(t, got, "kingdoms:  5 (min 8)")
	require.Contains(t, got, "your kingdom: id=kgd-7")
}

func TestRunWorldJoin_setsWorldScope(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/join": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "kgd-7", "world_id": "wld-1",
				"home_region_id": null,
				"stockpiles": {"gold": 0},
				"joined_at": "2026-05-02T00:00:00Z"
			}`))
		},
	}
	sess, out, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	require.NoError(t, runWorldJoin(context.Background(), sess, []string{"spring-2026"}, nil))
	require.Equal(t, "spring-2026", sess.Context.WorldSlug())
	require.Contains(t, out.String(), "joined world (slug=spring-2026)")
	require.Contains(t, out.String(), "no home region yet")
}

func TestRunWorldJoin_propagatesForbidden(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/join": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"forbidden","message":"closed"}}`))
		},
	}
	sess, _, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	err := runWorldJoin(context.Background(), sess, []string{"spring-2026"}, nil)
	require.Error(t, err)
	require.Empty(t, sess.Context.WorldSlug(), "world scope must not be set on failure")
}

// worldShowFixture returns a handler for GET /v1/worlds/wld-1 with the
// given lifecycle status, optionally carrying a my_kingdom block (set
// iff the caller has joined).
func worldShowFixture(status string, joined bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mk := ""
		if joined {
			mk = `,"my_kingdom":{"id":"kgd-7","world_id":"wld-1",` +
				`"home_region_id":"reg-2","stockpiles":{"gold":0},` +
				`"joined_at":"2026-05-02T00:00:00Z"}`
		}
		_, _ = w.Write([]byte(`{
			"id":"wld-1","server_id":"srv-1","slug":"spring-2026",
			"name":"Spring 2026","status":"` + status + `","min_players":8,
			"t0_at":"2026-05-01T00:00:00Z","region_count":42,
			"kingdom_count":5` + mk + `}`))
	}
}

func TestRunWorldUse_setsWorldScopeForJoinedWorld(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1": worldShowFixture("grace", true),
	}
	sess, out, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	require.NoError(t, runWorldUse(context.Background(), sess, []string{"spring-2026"}, nil))
	require.Equal(t, "spring-2026", sess.Context.WorldSlug())
	got := out.String()
	require.Contains(t, got, `scope set to world "Spring 2026" (slug=spring-2026)`)
	require.Contains(t, got, "home region: reg-2")
}

func TestRunWorldUse_rejectsUnjoinedJoinableWorld(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1": worldShowFixture("grace", false),
	}
	sess, _, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	err := runWorldUse(context.Background(), sess, []string{"spring-2026"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "haven't joined")
	require.Contains(t, err.Error(), "world join spring-2026")
	require.Empty(t, sess.Context.WorldSlug(), "scope must not be set for an unjoined world")
}

func TestRunWorldUse_rejectsUnjoinedClosedWorld(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1": worldShowFixture("active", false),
	}
	sess, _, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	err := runWorldUse(context.Background(), sess, []string{"spring-2026"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "join window has closed")
	require.Empty(t, sess.Context.WorldSlug())
}

func TestRunWorldUse_requiresServerScope(t *testing.T) {
	sess, _, _ := newTestSession(t, worldsHandler(t, nil))
	err := runWorldUse(context.Background(), sess, []string{"spring-2026"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunJoinSugar_routesWorldKeyword(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/join": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "kgd-7", "world_id": "wld-1",
				"stockpiles": {"gold": 0},
				"joined_at": "2026-05-02T00:00:00Z"
			}`))
		},
	}
	sess, _, _ := newTestSession(t, worldsHandler(t, extra))
	sess.Context.SetServer("acme")

	// `join world spring-2026` should route to runWorldJoin.
	require.NoError(t, runJoinSugar(context.Background(), sess, []string{"world", "spring-2026"}, nil))
	require.Equal(t, "spring-2026", sess.Context.WorldSlug())
}
