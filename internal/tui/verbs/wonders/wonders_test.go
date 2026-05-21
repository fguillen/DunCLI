package wonders

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func TestVerbsRegistered(t *testing.T) {
	wonder, ok := shell.Resolve("wonder")
	require.True(t, ok)
	require.NotNil(t, wonder.Run, "bare `wonder` must route to wonder show")
	for _, sub := range []string{"show", "start", "cancel", "repair", "milestone"} {
		v, ok := wonder.Sub[sub]
		require.True(t, ok, "missing wonder %s sub-verb", sub)
		require.NotNil(t, v.Run, "wonder %s must have a Run", sub)
	}

	wonders, ok := shell.Resolve("wonders")
	require.True(t, ok)
	require.NotNil(t, wonders.Run)
}

func TestRunWonderShow_requiresWorldScope(t *testing.T) {
	sess, _, _ := newTestSession(t, wondersHandler(t, nil))
	err := runWonderShow(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunWonderShow_noWonder(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/wonder": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"wonder": null}`))
		},
	}
	sess, out := setupWondersSession(t, extra)
	err := runWonderShow(context.Background(), sess, nil, nil)
	require.NoError(t, err)
	require.Contains(t, out.String(), "no wonder under construction")
}

func TestRunWonderShow_rendersWonder(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/wonder": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "wnd-1", "kingdom_id": "kgd-1", "world_id": "wld-1",
				"name": "sky_tower", "status": "construction",
				"hp": 4200, "target_hp": 10000,
				"milestones_paid": {"25": true, "50": false, "75": false},
				"pending_milestone_percent": null,
				"pending_milestone_cost": null,
				"repaired_hp_by_phase": {"foundation": 0, "construction": 0, "consecration": 0},
				"paused_until": null,
				"started_at": "2026-05-01T00:00:00Z",
				"construction_started_at": "2026-05-01T00:00:00Z",
				"consecration_at": null,
				"completed_at": null,
				"destroyed_at": null
			}`))
		},
	}
	sess, out := setupWondersSession(t, extra)
	require.NoError(t, runWonderShow(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Sky Tower")
	require.Contains(t, got, "hp:        4200/10000")
	require.Contains(t, got, "25=paid")
}

func TestRunWonderStart_unknownName(t *testing.T) {
	sess, _ := setupWondersSession(t, nil)
	err := runWonderStart(context.Background(), sess, []string{"Sky Tower"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown wonder name "Sky Tower"`)
}

func TestRunWonderStart_tooManyArgs(t *testing.T) {
	sess, _ := setupWondersSession(t, nil)
	err := runWonderStart(context.Background(), sess, []string{"a", "b"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: wonder start")
}

func TestRunWonderMilestone_noPending(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/wonder": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "wnd-1", "kingdom_id": "kgd-1", "world_id": "wld-1",
				"name": "sky_tower", "status": "construction",
				"hp": 4200, "target_hp": 10000,
				"milestones_paid": {"25": true, "50": false, "75": false},
				"pending_milestone_percent": null,
				"pending_milestone_cost": null,
				"repaired_hp_by_phase": {"foundation": 0, "construction": 0, "consecration": 0},
				"paused_until": null,
				"started_at": "2026-05-01T00:00:00Z",
				"construction_started_at": null,
				"consecration_at": null, "completed_at": null, "destroyed_at": null
			}`))
		},
	}
	sess, _ := setupWondersSession(t, extra)
	err := runWonderMilestone(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no milestone pending")
}

func TestRunWonderMilestone_mismatch(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/wonder": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "wnd-1", "kingdom_id": "kgd-1", "world_id": "wld-1",
				"name": "sky_tower", "status": "construction",
				"hp": 2500, "target_hp": 10000,
				"milestones_paid": {"25": false, "50": false, "75": false},
				"pending_milestone_percent": 25,
				"pending_milestone_cost": {"gold": 80000, "wood": 60000, "stone": 240000, "iron": 80000},
				"repaired_hp_by_phase": {"foundation": 0, "construction": 0, "consecration": 0},
				"paused_until": null,
				"started_at": "2026-05-01T00:00:00Z",
				"construction_started_at": null,
				"consecration_at": null, "completed_at": null, "destroyed_at": null
			}`))
		},
	}
	sess, _ := setupWondersSession(t, extra)
	err := runWonderMilestone(context.Background(), sess, []string{"50"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "milestone 50% is not pending (waiting for 25%)")
}

func TestRunWonderMilestone_invalidPercent(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/wonder": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "wnd-1", "kingdom_id": "kgd-1", "world_id": "wld-1",
				"name": "sky_tower", "status": "construction",
				"hp": 2500, "target_hp": 10000,
				"milestones_paid": {"25": false, "50": false, "75": false},
				"pending_milestone_percent": 25,
				"pending_milestone_cost": {"stone": 100},
				"repaired_hp_by_phase": {"foundation": 0, "construction": 0, "consecration": 0},
				"paused_until": null,
				"started_at": "2026-05-01T00:00:00Z",
				"construction_started_at": null,
				"consecration_at": null, "completed_at": null, "destroyed_at": null
			}`))
		},
	}
	sess, _ := setupWondersSession(t, extra)
	err := runWonderMilestone(context.Background(), sess, []string{"42"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "percent must be one of 25/50/75")
}

func TestRunWonderRepair_argParsing(t *testing.T) {
	sess, _ := setupWondersSession(t, nil)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"non-numeric", []string{"abc"}, "hp must be a positive integer"},
		{"zero", []string{"0"}, "hp must be a positive integer"},
		{"negative", []string{"-5"}, "hp must be a positive integer"},
		{"too many args", []string{"100", "200"}, "usage: wonder repair"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runWonderRepair(context.Background(), sess, tc.args, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRunWonderCancel_noWonder(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/wonder": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"wonder": null}`))
		},
	}
	sess, _ := setupWondersSession(t, extra)
	err := runWonderCancel(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no wonder to cancel")
}

func TestRunWondersList_empty(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/wonders": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"wonders": []}`))
		},
	}
	sess, out := setupWondersSession(t, extra)
	require.NoError(t, runWondersList(context.Background(), sess, nil, nil))
	require.Contains(t, out.String(), "Wonders:")
	require.Contains(t, out.String(), "(none)")
}

func TestRunWondersList_populated(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/wonders": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"wonders": [
				{
					"id": "wnd-1", "kingdom_id": "kgd-1", "builder_handle": "IronFist",
					"name": "sky_tower", "status": "construction",
					"hp": 4200, "target_hp": 10000, "hp_pct": 42,
					"started_at": "2026-05-01T00:00:00Z",
					"consecration_at": null, "completed_at": null, "destroyed_at": null
				}
			]}`))
		},
	}
	sess, out := setupWondersSession(t, extra)
	require.NoError(t, runWondersList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "IronFist")
	require.Contains(t, got, "Sky Tower")
	require.Contains(t, got, "construction")
}

func TestSuggesters(t *testing.T) {
	got, err := suggestWonderNames(context.Background(), nil, "")
	require.NoError(t, err)
	require.Len(t, got, 6)
	require.Equal(t, "sky_tower", got[0])

	got, err = suggestMilestonePercents(context.Background(), nil, "")
	require.NoError(t, err)
	require.Equal(t, []string{"25", "50", "75"}, got)
}

func TestValidPercent(t *testing.T) {
	for _, n := range []int{25, 50, 75} {
		require.True(t, validPercent(n))
	}
	for _, n := range []int{0, 24, 26, 50_5, 100} {
		require.False(t, validPercent(n))
	}
}

func TestJoinInts(t *testing.T) {
	require.Equal(t, "25/50/75", joinInts([]int{25, 50, 75}))
	require.Equal(t, "", joinInts(nil))
}
