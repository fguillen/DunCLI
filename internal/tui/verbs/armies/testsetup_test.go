package armies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// newTestSession builds a *shell.Session pointed at the given httptest
// handler. Duplicated from internal/tui/verbs/servers_test.go because
// the helper is package-private to `verbs` and the surface is too
// small to justify a cross-package test-support package.
func newTestSession(t *testing.T, h http.HandlerFunc) (*shell.Session, *bytes.Buffer, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := api.New(srv.URL+"/v1", api.StaticToken("t"), srv.Client())
	require.NoError(t, err)

	var out bytes.Buffer
	sess := &shell.Session{
		API:     c,
		Cfg:     shell.ConfigSnapshot{BaseURL: srv.URL + "/v1"},
		Creds:   shell.CredentialSnapshot{Email: "tester@example.com"},
		Context: shell.NewContext(),
		Out:     &out,
		State:   noopState{},
	}
	return sess, &out, srv
}

type noopState struct{}

func (noopState) Save(shell.ContextSnapshot) error { return nil }

// militaryHandler stages the resolver chain (servers, worlds,
// showWorld for my_kingdom) plus a small set of Phase 8-specific
// routes. Tests merge in `extra` for case-specific responses.
func militaryHandler(t *testing.T, extra map[string]http.HandlerFunc) http.HandlerFunc {
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
			return
		case "/v1/servers/srv-1/worlds":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"worlds": []map[string]any{{
					"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
					"name": "Spring 2026", "status": "active", "min_players": 8,
					"t0_at": "2026-05-01T00:00:00Z",
				}},
			})
			return
		case "/v1/worlds/wld-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "wld-1", "server_id": "srv-1", "slug": "spring-2026",
				"name": "Spring 2026", "status": "active", "min_players": 8,
				"t0_at":        "2026-05-01T00:00:00Z",
				"region_count": 2, "kingdom_count": 1,
				"my_kingdom": map[string]any{
					"id": "kgd-7", "world_id": "wld-1",
					"stockpiles": map[string]any{},
					"joined_at":  "2026-05-01T00:00:00Z",
				},
			})
			return
		case "/v1/worlds/wld-1/map":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"regions": []map[string]any{
					{
						"id": "reg-1", "name": "Greyhollow", "terrain": "forest",
						"position":       map[string]any{"x": 0.1, "y": 0.2},
						"adjacency":      []string{"reg-2"},
						"nodes":          []any{},
						"visible_armies": []any{},
					},
					{
						"id": "reg-2", "name": "Ironvale", "terrain": "hills",
						"position":       map[string]any{"x": 0.3, "y": 0.4},
						"adjacency":      []string{"reg-1"},
						"nodes":          []any{},
						"visible_armies": []any{},
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

// setupMilitarySession builds a session scoped at (acme, spring-2026).
func setupMilitarySession(t *testing.T, extra map[string]http.HandlerFunc) (*shell.Session, *bytes.Buffer) {
	t.Helper()
	s, o, _ := newTestSession(t, militaryHandler(t, extra))
	s.Context.SetServer("acme")
	s.Context.SetWorld("spring-2026")
	return s, o
}
