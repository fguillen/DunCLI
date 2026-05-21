package archive

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

// newTestSession wires a Session pointed at an httptest backend. Trimmed
// copy of the helper in sibling subpackages (battles/, wonders/, trade/)
// — each subpackage keeps its own copy because the helpers are
// package-private.
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

// archiveHandler stages the resolver chain (servers, worlds) so the
// archive/hall-of-fame verbs can resolve their scope; tests merge in
// the actual Phase 13 routes via `extra`.
func archiveHandler(t *testing.T, extra map[string]http.HandlerFunc) http.HandlerFunc {
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
					"name": "Spring 2026", "status": "archived", "min_players": 8,
					"t0_at": "2026-05-01T00:00:00Z",
				}},
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

// setupSession returns a session scoped at (acme, spring-2026).
func setupSession(t *testing.T, extra map[string]http.HandlerFunc) (*shell.Session, *bytes.Buffer) {
	t.Helper()
	s, o, _ := newTestSession(t, archiveHandler(t, extra))
	s.Context.SetServer("acme")
	s.Context.SetWorld("spring-2026")
	return s, o
}

// setupServerOnly returns a session scoped at server `acme` only (no
// world scope). Used by archive arg-form tests where the arg must
// override the missing world scope.
func setupServerOnly(t *testing.T, extra map[string]http.HandlerFunc) (*shell.Session, *bytes.Buffer) {
	t.Helper()
	s, o, _ := newTestSession(t, archiveHandler(t, extra))
	s.Context.SetServer("acme")
	return s, o
}
