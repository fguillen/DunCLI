package verbs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// newTestSession builds a *shell.Session pointed at the given httptest
// handler so verb-handler tests can exercise real wrapper calls
// without standing up a backend.
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

func TestRunServersList_splitsMemberFromEligible(t *testing.T) {
	sess, out, _ := newTestSession(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-srv")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"servers": []map[string]any{
				{"id": "1", "slug": "acme", "name": "Acme", "member": true},
				{"id": "2", "slug": "beta", "name": "Beta", "member": false},
			},
		})
	})

	require.NoError(t, runServersList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Member of:")
	require.Contains(t, got, "acme")
	require.Contains(t, got, "Eligible to join:")
	require.Contains(t, got, "beta")
}

func TestRunServerJoin_setsContextOnSuccess(t *testing.T) {
	calls := 0
	sess, _, _ := newTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("X-Request-Id", "req-"+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/servers"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{
					{"id": "srv-1", "slug": "acme", "name": "Acme", "member": false},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/servers/srv-1/join"):
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"membership_id": "mem-1",
				"server":        map[string]any{"id": "srv-1", "slug": "acme", "name": "Acme", "member": true},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	require.NoError(t, runServerJoin(context.Background(), sess, []string{"acme"}, nil))
	require.Equal(t, "acme", sess.Context.ServerSlug())
	require.Equal(t, 2, calls, "expected ResolveServer + JoinServer calls")
}

func TestRunServerJoin_propagatesForbidden(t *testing.T) {
	sess, _, _ := newTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-x")
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/servers") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{
					{"id": "srv-1", "slug": "acme", "name": "Acme", "member": false},
				},
			})
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "forbidden",
				"message": "invite only",
			},
		})
	})

	err := runServerJoin(context.Background(), sess, []string{"acme"}, nil)
	require.Error(t, err)
	apiErr := api.AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "forbidden", apiErr.Code)
	require.Empty(t, sess.Context.ServerSlug(), "context must not be set on failed join")
}
