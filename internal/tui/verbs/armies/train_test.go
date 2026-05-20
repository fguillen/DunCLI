package armies

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// TestTrainingBuildings_pinnedToGenEnum guards against silent drift
// between the static completion list and the generated client enum.
func TestTrainingBuildings_pinnedToGenEnum(t *testing.T) {
	want := make([]string, 0)
	for _, v := range gen.QueueTrainingOrderReqBuilding("").AllValues() {
		want = append(want, string(v))
	}
	require.ElementsMatch(t, want, trainingBuildings,
		"trainingBuildings is out of sync with gen.QueueTrainingOrderReqBuilding.AllValues()")
}

func TestUnitKinds_pinnedToGenEnum(t *testing.T) {
	want := make([]string, 0)
	for _, v := range gen.Unit("").AllValues() {
		want = append(want, string(v))
	}
	require.ElementsMatch(t, want, unitKinds,
		"unitKinds is out of sync with gen.Unit.AllValues()")
}

func TestRunTrainPreview_happyPath(t *testing.T) {
	var qp url.Values
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/train/preview": func(w http.ResponseWriter, r *http.Request) {
			qp = r.URL.Query()
			_, _ = w.Write([]byte(`{
				"building_kind": "barracks",
				"unit": "levy",
				"count": 5,
				"building_level": 2,
				"building_built": true,
				"unit_trainable_here": true,
				"per_unit_cost": {"gold": 10, "wood": 5, "stone": 0, "iron": 0},
				"total_cost": {"gold": 50, "wood": 25, "stone": 0, "iron": 0},
				"per_unit_seconds": 90,
				"total_seconds": 450,
				"affordable": true,
				"missing": {"gold": 0, "wood": 0, "stone": 0, "iron": 0},
				"max_affordable_count": 20
			}`))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runTrainPreview(context.Background(), sess, []string{"barracks", "levy", "5"}, nil))
	got := out.String()
	require.Contains(t, got, "levy training preview")
	require.Contains(t, got, "at:        barracks")
	require.Contains(t, got, "max afford: 20")
	require.Equal(t, "barracks", qp.Get("building"))
	require.Equal(t, "levy", qp.Get("unit"))
	require.Equal(t, "5", qp.Get("count"))
}

func TestRunTrainPreview_rejectsUnknownUnit(t *testing.T) {
	sess, _ := setupMilitarySession(t, nil)
	err := runTrainPreview(context.Background(), sess, []string{"barracks", "wizard", "5"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown unit "wizard"`)
}

func TestRunTrainPreview_rejectsBadCount(t *testing.T) {
	sess, _ := setupMilitarySession(t, nil)
	err := runTrainPreview(context.Background(), sess, []string{"barracks", "levy", "zero"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "count must be a positive integer")
}

func TestRunTrainCancel_resolvesUnitToOrderID(t *testing.T) {
	deletedPath := ""
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "kgd-7", "world_id": "wld-1", "home_region_id": "reg-1",
				"stockpiles": {"gold": 100, "wood": 50, "stone": 25, "iron": 10},
				"warehouse_cap": 1000,
				"production_rates": {"gold": 12, "wood": 6, "stone": 3, "iron": 1},
				"joined_at": "2026-05-01T00:00:00Z",
				"buildings": [], "in_progress_builds": [],
				"in_progress_training": [{
					"id": "trn-9", "kingdom_id": "kgd-7",
					"building_id": "bld-1", "building_kind": "barracks",
					"unit": "levy", "count": 5,
					"started_at": "2026-05-20T00:00:00Z",
					"completes_at": "2026-05-20T00:30:00Z",
					"completed_at": null, "cancelled_at": null
				}]
			}`))
		},
		"/v1/kingdoms/kgd-7/train/trn-9": func(w http.ResponseWriter, r *http.Request) {
			deletedPath = r.URL.Path
			_, _ = w.Write([]byte(`{
				"id": "trn-9", "kingdom_id": "kgd-7",
				"building_id": "bld-1", "building_kind": "barracks",
				"unit": "levy", "count": 5,
				"started_at": "2026-05-20T00:00:00Z",
				"completes_at": "2026-05-20T00:30:00Z",
				"completed_at": null,
				"cancelled_at": "2026-05-20T00:05:00Z"
			}`))
		},
	}
	sess, _ := setupMilitarySession(t, extra)

	// runTrainCancel needs an interactive confirm; tests can't drive
	// a tea program. Exercise the unit→order_id resolution path
	// directly the same way kingdom_test.go does for build cancel.
	kd, err := sess.API.ShowKingdom(context.Background(), "kgd-7")
	require.NoError(t, err)
	require.Len(t, kd.InProgressTraining, 1)
	require.Equal(t, "trn-9", kd.InProgressTraining[0].ID)
	require.Equal(t, "levy", string(kd.InProgressTraining[0].Unit))

	ord, err := sess.API.CancelTrainingOrder(context.Background(), "kgd-7", "trn-9")
	require.NoError(t, err)
	require.Equal(t, "trn-9", ord.ID)
	require.Equal(t, "/v1/kingdoms/kgd-7/train/trn-9", deletedPath)
}

func TestSuggestActiveTrainingArgs_yieldsUnitsAndIDs(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "kgd-7", "world_id": "wld-1", "home_region_id": "reg-1",
				"stockpiles": {"gold": 1, "wood": 1, "stone": 1, "iron": 1},
				"warehouse_cap": 1000,
				"production_rates": {"gold": 1, "wood": 1, "stone": 1, "iron": 1},
				"joined_at": "2026-05-01T00:00:00Z",
				"buildings": [], "in_progress_builds": [],
				"in_progress_training": [
					{"id": "trn-1", "kingdom_id": "kgd-7", "building_id": "b1",
					 "building_kind": "barracks", "unit": "levy", "count": 3,
					 "started_at": "2026-05-20T00:00:00Z",
					 "completes_at": "2026-05-20T00:10:00Z",
					 "completed_at": null, "cancelled_at": null},
					{"id": "trn-2", "kingdom_id": "kgd-7", "building_id": "b1",
					 "building_kind": "barracks", "unit": "levy", "count": 2,
					 "started_at": "2026-05-20T00:00:00Z",
					 "completes_at": "2026-05-20T00:15:00Z",
					 "completed_at": null, "cancelled_at": null}
				]
			}`))
		},
	}
	sess, _ := setupMilitarySession(t, extra)
	got, err := suggestActiveTrainingArgs(context.Background(), sess, "")
	require.NoError(t, err)
	// One "levy" (deduped) + both IDs.
	require.Contains(t, got, "levy")
	require.Contains(t, got, "trn-1")
	require.Contains(t, got, "trn-2")
}
