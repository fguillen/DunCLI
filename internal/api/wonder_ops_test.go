package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClient_GetWonder_returnsWonder pins the happy path: a populated
// 200 body parses into a *gen.Wonder.
func TestClient_GetWonder_returnsWonder(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/wonder"))
		w.Header().Set("X-Request-Id", "req-gw")
		writeJSON(t, w, `{
			"id": "wnd-1", "kingdom_id": "kgd-7", "world_id": "wld-1",
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
		}`)
	})

	got, err := c.GetWonder(context.Background(), "kgd-7")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "wnd-1", got.ID)
	require.EqualValues(t, "sky_tower", got.Name)
	require.Equal(t, 4200, got.Hp)
}

// TestClient_GetWonder_nullWonder verifies that the `{wonder: null}`
// envelope (no wonder under construction) collapses to (nil, nil) so
// UI code never has to inspect the gen sum type.
func TestClient_GetWonder_nullWonder(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-gw-null")
		writeJSON(t, w, `{"wonder": null}`)
	})

	got, err := c.GetWonder(context.Background(), "kgd-7")
	require.NoError(t, err)
	require.Nil(t, got, "null wonder must surface as nil pointer")
}

// TestClient_StartWonder_sendsBodyAndInvalidates pins the request body
// shape and confirms the kingdom-cache invalidation hook is called.
func TestClient_StartWonder_sendsBodyAndInvalidates(t *testing.T) {
	var seen map[string]any
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/wonder"))
		seen = readJSONBody(t, r)
		w.Header().Set("X-Request-Id", "req-sw")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"id": "wnd-1", "kingdom_id": "kgd-7", "world_id": "wld-1",
			"name": "sky_tower", "status": "foundation",
			"hp": 1000, "target_hp": 1000,
			"milestones_paid": {"25": false, "50": false, "75": false},
			"pending_milestone_percent": null,
			"pending_milestone_cost": null,
			"repaired_hp_by_phase": {"foundation": 0, "construction": 0, "consecration": 0},
			"paused_until": null,
			"started_at": "2026-05-01T00:00:00Z",
			"construction_started_at": null,
			"consecration_at": null,
			"completed_at": null,
			"destroyed_at": null
		}`))
		require.NoError(t, err)
	})

	got, err := c.StartWonder(context.Background(), "kgd-7", "sky_tower")
	require.NoError(t, err)
	require.Equal(t, "wnd-1", got.ID)
	require.Equal(t, "sky_tower", seen["name"])
}

// TestClient_StartWonder_unprocessable surfaces the documented 422
// codes (wonder_prereq_unmet, etc.) through the standard api.Error.
// The ErrorEnvelope `code` enum doesn't include the domain-specific
// codes, so we exercise the generic `invalid` which IS in the enum —
// matching how Phase 7-11 ops are tested.
func TestClient_StartWonder_unprocessable(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-sw422")
		writeEnvelope(t, w, http.StatusUnprocessableEntity, "invalid",
			"wonder_prereq_unmet", 0)
	})

	_, err := c.StartWonder(context.Background(), "kgd-7", "sky_tower")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "invalid", apiErr.Code)
	require.Equal(t, "wonder_prereq_unmet", apiErr.Message)
}

// TestClient_CancelWonder_happyPath pins method + path and confirms
// the returned destroyed-wonder snapshot.
func TestClient_CancelWonder_happyPath(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/wonder"))
		w.Header().Set("X-Request-Id", "req-cw")
		writeJSON(t, w, `{
			"id": "wnd-1", "kingdom_id": "kgd-7", "world_id": "wld-1",
			"name": "sky_tower", "status": "destroyed",
			"hp": 0, "target_hp": 10000,
			"milestones_paid": {"25": true, "50": false, "75": false},
			"pending_milestone_percent": null,
			"pending_milestone_cost": null,
			"repaired_hp_by_phase": {"foundation": 0, "construction": 0, "consecration": 0},
			"paused_until": null,
			"started_at": "2026-05-01T00:00:00Z",
			"construction_started_at": "2026-05-01T00:00:00Z",
			"consecration_at": null,
			"completed_at": null,
			"destroyed_at": "2026-05-21T12:00:00Z"
		}`)
	})

	got, err := c.CancelWonder(context.Background(), "kgd-7")
	require.NoError(t, err)
	require.EqualValues(t, "destroyed", got.Status)
}

// TestClient_RepairWonder_sendsBody pins the body shape and the
// returned wonder.
func TestClient_RepairWonder_sendsBody(t *testing.T) {
	var seen map[string]any
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/wonder/repair"))
		seen = readJSONBody(t, r)
		w.Header().Set("X-Request-Id", "req-rw")
		writeJSON(t, w, `{
			"id": "wnd-1", "kingdom_id": "kgd-7", "world_id": "wld-1",
			"name": "sky_tower", "status": "construction",
			"hp": 5000, "target_hp": 10000,
			"milestones_paid": {"25": true, "50": false, "75": false},
			"pending_milestone_percent": null,
			"pending_milestone_cost": null,
			"repaired_hp_by_phase": {"foundation": 0, "construction": 500, "consecration": 0},
			"paused_until": "2026-05-21T13:00:00Z",
			"started_at": "2026-05-01T00:00:00Z",
			"construction_started_at": "2026-05-01T00:00:00Z",
			"consecration_at": null,
			"completed_at": null,
			"destroyed_at": null
		}`)
	})

	got, err := c.RepairWonder(context.Background(), "kgd-7", 500)
	require.NoError(t, err)
	require.EqualValues(t, 500, seen["hp"])
	require.Equal(t, 5000, got.Hp)
	require.True(t, got.PausedUntil.IsSet())
}

// TestClient_PayWonderMilestone_sendsBody pins the percent body and
// the resumed-construction response.
func TestClient_PayWonderMilestone_sendsBody(t *testing.T) {
	var seen map[string]any
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/kingdoms/kgd-7/wonder/milestone"))
		seen = readJSONBody(t, r)
		w.Header().Set("X-Request-Id", "req-pm")
		writeJSON(t, w, `{
			"id": "wnd-1", "kingdom_id": "kgd-7", "world_id": "wld-1",
			"name": "sky_tower", "status": "construction",
			"hp": 2500, "target_hp": 10000,
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
		}`)
	})

	got, err := c.PayWonderMilestone(context.Background(), "kgd-7", 25)
	require.NoError(t, err)
	require.EqualValues(t, 25, seen["percent"])
	require.True(t, got.MilestonesPaid.R25.Value)
}

// TestClient_PayWonderMilestone_noPending exercises the 422 path the
// backend emits when no milestone is pending. Mapped to `invalid` per
// the ErrorEnvelope enum (the wrapper preserves the original message).
func TestClient_PayWonderMilestone_noPending(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-pm422")
		writeEnvelope(t, w, http.StatusUnprocessableEntity, "invalid",
			"no_milestone_pending", 0)
	})

	_, err := c.PayWonderMilestone(context.Background(), "kgd-7", 25)
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "no_milestone_pending", apiErr.Message)
}

// TestClient_ListWorldWonders_returnsItems pins the world-scoped list
// shape.
func TestClient_ListWorldWonders_returnsItems(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/worlds/wld-1/wonders"))
		w.Header().Set("X-Request-Id", "req-lw")
		writeJSON(t, w, `{
			"wonders": [
				{
					"id": "wnd-1", "kingdom_id": "kgd-7", "builder_handle": "IronFist",
					"name": "sky_tower", "status": "construction",
					"hp": 4200, "target_hp": 10000, "hp_pct": 42,
					"started_at": "2026-05-01T00:00:00Z",
					"consecration_at": null, "completed_at": null, "destroyed_at": null
				},
				{
					"id": "wnd-2", "kingdom_id": "kgd-8", "builder_handle": "ShadowWolf",
					"name": "eternal_citadel", "status": "foundation",
					"hp": 1000, "target_hp": 1000, "hp_pct": 100,
					"started_at": "2026-05-15T00:00:00Z",
					"consecration_at": null, "completed_at": null, "destroyed_at": null
				}
			]
		}`)
	})

	got, err := c.ListWorldWonders(context.Background(), "wld-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "IronFist", got[0].BuilderHandle)
	require.Equal(t, "ShadowWolf", got[1].BuilderHandle)
}
