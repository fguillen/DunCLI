package archive

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func TestVerbsRegistered(t *testing.T) {
	archive, ok := shell.Resolve("archive")
	require.True(t, ok)
	require.NotNil(t, archive.Run)

	hof, ok := shell.Resolve("hall-of-fame")
	require.True(t, ok)
	require.NotNil(t, hof.Run)
	// `--kind` flag must be declared so the dispatcher accepts it.
	spec, ok := hof.FlagByName("kind")
	require.True(t, ok)
	require.True(t, spec.HasValue)
	require.NotNil(t, spec.Suggest)
}

func TestRunArchive_requiresServerScope(t *testing.T) {
	sess, _, _ := newTestSession(t, archiveHandler(t, nil))
	err := runArchive(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunArchive_requiresWorldOrArg(t *testing.T) {
	sess, _ := setupServerOnly(t, nil)
	err := runArchive(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a world scope")
}

func TestRunArchive_rejectsTooManyArgs(t *testing.T) {
	sess, _ := setupSession(t, nil)
	err := runArchive(context.Background(), sess, []string{"a", "b"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: archive")
}

func TestRunArchive_rejectsBlankArg(t *testing.T) {
	sess, _ := setupServerOnly(t, nil)
	err := runArchive(context.Background(), sess, []string{"   "}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "world slug is required")
}

func TestRunArchive_inScopeWorld(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/archive": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"world_id": "wld-1",
				"winner_kingdom_id": "kgd-7",
				"wonder_name": "sky_tower",
				"ended_at": "2026-05-21T12:00:00Z",
				"frozen_state": {
					"ended_at": "2026-05-21T12:00:00Z",
					"regions": [],
					"kingdoms": [
						{"id": "kgd-7", "handle": "IronFist",
						 "home_region_id": "reg-1",
						 "final_stockpiles": {"gold": 0, "wood": 0, "stone": 0, "iron": 0,
						                      "checkpoint_at": "2026-05-21T12:00:00Z"},
						 "building_levels": {},
						 "peak_nodes": 8, "final_node_count": 6,
						 "joined_at": "2026-05-01T00:00:00Z"},
						{"id": "kgd-8", "handle": "ShadowWolf",
						 "home_region_id": "reg-2",
						 "final_stockpiles": {"gold": 0, "wood": 0, "stone": 0, "iron": 0,
						                      "checkpoint_at": "2026-05-21T12:00:00Z"},
						 "building_levels": {},
						 "peak_nodes": 4, "final_node_count": 2,
						 "joined_at": "2026-05-01T00:00:00Z",
						 "eliminated_at": "2026-05-15T08:00:00Z"}
					],
					"wonder": {"kingdom_id": "kgd-7", "name": "sky_tower",
					           "status": "completed", "hp": 10000, "target_hp": 10000,
					           "damage_events_count": 3},
					"battles_count": 42, "caravans_count": 17, "nodes_count": 9
				}
			}`))
		},
	}
	sess, out := setupSession(t, extra)
	require.NoError(t, runArchive(context.Background(), sess, nil, nil))
	s := out.String()
	require.Contains(t, s, "World archive")
	require.Contains(t, s, "winner:    kgd-7")
	require.Contains(t, s, "wonder:    sky_tower")
	require.Contains(t, s, "battles:   42")
	require.Contains(t, s, "IronFist")
	require.Contains(t, s, "eliminated 2026-05-15")
}

func TestRunArchive_argOverridesScope(t *testing.T) {
	called := false
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/archive": func(w http.ResponseWriter, _ *http.Request) {
			called = true
			_, _ = w.Write([]byte(`{
				"world_id": "wld-1",
				"winner_kingdom_id": null,
				"wonder_name": null,
				"ended_at": "2026-05-21T12:00:00Z",
				"frozen_state": {
					"ended_at": "2026-05-21T12:00:00Z",
					"regions": [], "kingdoms": [],
					"battles_count": 0, "caravans_count": 0, "nodes_count": 0
				}
			}`))
		},
	}
	sess, out := setupServerOnly(t, extra)
	require.NoError(t, runArchive(context.Background(), sess, []string{"spring-2026"}, nil))
	require.True(t, called)
	s := out.String()
	require.Contains(t, s, "winner:    (none)")
	require.Contains(t, s, "wonder:    (none)")
}

func TestRunArchive_notFoundSurfacesAsError(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/archive": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": {"code": "not_found", "message": "world is still live"}}`))
		},
	}
	sess, _ := setupSession(t, extra)
	err := runArchive(context.Background(), sess, nil, nil)
	require.Error(t, err)
}
