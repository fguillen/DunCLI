package archive

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunHallOfFame_requiresServerScope(t *testing.T) {
	sess, _, _ := newTestSession(t, archiveHandler(t, nil))
	err := runHallOfFame(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunHallOfFame_rejectsBadKind(t *testing.T) {
	sess, _ := setupServerOnly(t, nil)
	err := runHallOfFame(context.Background(), sess, nil, map[string]string{"kind": "nope"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--kind must be one of")
}

func TestRunHallOfFame_rejectsPositionalArgs(t *testing.T) {
	sess, _ := setupServerOnly(t, nil)
	err := runHallOfFame(context.Background(), sess, []string{"champions"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: hall-of-fame")
}

func TestRunHallOfFame_summary(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/servers/srv-1/hall-of-fame": func(w http.ResponseWriter, r *http.Request) {
			require.Empty(t, r.URL.Query().Get("kind"))
			_, _ = w.Write([]byte(`{
				"server_id": "srv-1",
				"leaderboards": {
					"champions": {
						"snapshot_at": "2026-05-20T00:00:00Z",
						"entries": [
							{"player_profile_id": "ppl-1", "handle": "IronFist",
							 "score": 3, "secondary": 1, "title": "Champion"},
							{"player_profile_id": "ppl-2", "handle": "ShadowWolf",
							 "score": 2, "secondary": 0, "title": null}
						]
					},
					"wreckers": {"snapshot_at": null, "entries": []},
					"warlords": {
						"snapshot_at": "2026-05-20T00:00:00Z",
						"entries": [
							{"player_profile_id": null, "handle": null,
							 "score": 99, "secondary": 50, "title": null}
						]
					},
					"veterans": {"snapshot_at": "2026-05-20T00:00:00Z", "entries": []}
				}
			}`))
		},
	}
	sess, out := setupServerOnly(t, extra)
	require.NoError(t, runHallOfFame(context.Background(), sess, nil, nil))
	s := out.String()
	require.Contains(t, s, "Hall of Fame")
	require.Contains(t, s, "Champions")
	require.Contains(t, s, "Wreckers")
	require.Contains(t, s, "Warlords")
	require.Contains(t, s, "Veterans")
	require.Contains(t, s, "IronFist")
	require.Contains(t, s, "title=Champion")
	require.Contains(t, s, "(deleted)") // null-handle entry on warlords
	require.Contains(t, s, "wonders=3")
	require.Contains(t, s, "(none)")              // wreckers + veterans empty
	require.Contains(t, s, "no snapshot yet")     // wreckers
	require.Contains(t, s, "full list per board") // summary hint
}

func TestRunHallOfFame_kindFilter(t *testing.T) {
	called := false
	extra := map[string]http.HandlerFunc{
		"/v1/servers/srv-1/hall-of-fame": func(w http.ResponseWriter, r *http.Request) {
			called = true
			require.Equal(t, "warlords", r.URL.Query().Get("kind"))
			_, _ = w.Write([]byte(`{
				"server_id": "srv-1",
				"leaderboards": {
					"warlords": {
						"snapshot_at": "2026-05-20T00:00:00Z",
						"entries": [
							{"player_profile_id": "ppl-9", "handle": "RedTalon",
							 "score": 42, "secondary": 12, "title": null}
						]
					}
				}
			}`))
		},
	}
	sess, out := setupServerOnly(t, extra)
	require.NoError(t, runHallOfFame(context.Background(), sess, nil, map[string]string{"kind": "warlords"}))
	require.True(t, called)
	s := out.String()
	require.Contains(t, s, "Warlords")
	require.Contains(t, s, "RedTalon")
	require.NotContains(t, s, "Champions")
	require.NotContains(t, s, "full list per board",
		"summary hint must not render when --kind is set")
}

func TestRunHallOfFame_emptyResponse(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/servers/srv-1/hall-of-fame": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"server_id": "srv-1", "leaderboards": {}}`))
		},
	}
	sess, out := setupServerOnly(t, extra)
	require.NoError(t, runHallOfFame(context.Background(), sess, nil, nil))
	require.Contains(t, out.String(), "no leaderboards available yet")
}

func TestSuggestKinds(t *testing.T) {
	out, err := suggestKinds(context.Background(), nil, "")
	require.NoError(t, err)
	require.Equal(t, []string{"champions", "wreckers", "warlords", "veterans"}, out)
}

func TestIsValidKind(t *testing.T) {
	for _, k := range []string{"champions", "wreckers", "warlords", "veterans"} {
		require.True(t, isValidKind(k))
	}
	for _, k := range []string{"", "Champions", "rookies", "champion"} {
		require.False(t, isValidKind(k))
	}
}
