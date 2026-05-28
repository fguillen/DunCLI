package armies

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api"
)

const armiesListBody = `{"armies": [
	{"id": "arm-1", "kingdom_id": "kgd-7", "name": "Garrison", "status": "home",
	 "location_region_id": "reg-1", "composition": {"levy": 12, "archer": 3},
	 "total_capacity": 60},
	{"id": "arm-2", "kingdom_id": "kgd-7", "name": "Vanguard", "status": "marching",
	 "location_region_id": "reg-2", "composition": {"knight": 4},
	 "total_capacity": 80,
	 "active_march": {"march_order_id": "mo-9", "intent": "attack",
	  "target_region_id": "reg-1", "arrives_at": "2030-01-01T00:00:00Z",
	  "dispatched_at": "2026-05-28T18:25:23Z"}}
]}`

func TestRunArmiesList_rendersTable(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runArmiesList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Garrison")
	require.Contains(t, got, "Vanguard")
	require.Contains(t, got, "Greyhollow") // region name resolved from reg-1
	require.Contains(t, got, "marching")
	// Vanguard's active march surfaces inline: target (Greyhollow,
	// resolved from reg-1), intent, and the ETA marker.
	require.Contains(t, got, "→ Greyhollow attack ETA")
}

func TestRunArmyShow_rendersActiveMarch(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-2": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "arm-2", "kingdom_id": "kgd-7", "name": "Vanguard", "status": "marching",
				"location_region_id": "reg-2", "composition": {"knight": 4},
				"total_capacity": 80,
				"active_march": {"march_order_id": "mo-9", "intent": "attack",
					"target_region_id": "reg-1", "arrives_at": "2030-01-01T00:00:00Z",
					"dispatched_at": "2026-05-28T18:25:23Z"}
			}`))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runArmyShow(context.Background(), sess, []string{"Vanguard"}, nil))
	got := out.String()
	require.Contains(t, got, "March:")
	require.Contains(t, got, "intent:    attack")
	require.Contains(t, got, "target:    Greyhollow") // resolved from reg-1
	require.Contains(t, got, "arrives:")
	require.Contains(t, got, "ETA")
}

func TestRunArmyShow_resolvesNameAndRendersComposition(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-1": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"id": "arm-1", "kingdom_id": "kgd-7", "name": "Garrison", "status": "home",
				"location_region_id": "reg-1",
				"composition": {"levy": 12, "archer": 3},
				"total_capacity": 60
			}`))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runArmyShow(context.Background(), sess, []string{"Garrison"}, nil))
	got := out.String()
	require.Contains(t, got, "Garrison")
	require.Contains(t, got, "Greyhollow")
	require.Contains(t, got, "levy")
	require.Contains(t, got, "archer")
}

func TestRunArmyRename_postsNewName(t *testing.T) {
	var body map[string]any
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-1/rename": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&body)
			_, _ = w.Write([]byte(`{
				"id": "arm-1", "kingdom_id": "kgd-7", "name": "Royal Guard", "status": "home",
				"location_region_id": "reg-1",
				"composition": {"levy": 12, "archer": 3},
				"total_capacity": 60
			}`))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runArmyRename(context.Background(), sess, []string{"Garrison", "Royal Guard"}, nil))
	require.Equal(t, "Royal Guard", body["name"])
	require.Contains(t, out.String(), "renamed: Royal Guard")
}

func TestRunArmyRename_rejectsEmptyName(t *testing.T) {
	sess, _ := setupMilitarySession(t, nil)
	err := runArmyRename(context.Background(), sess, []string{"Garrison", ""}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "1-60 chars")
}

func TestRunArmyMerge_rejectsMissingInto(t *testing.T) {
	sess, _ := setupMilitarySession(t, nil)
	err := runArmyMerge(context.Background(), sess, []string{"Garrison"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--into")
}

func TestRunArmyMerge_rejectsSelfMerge(t *testing.T) {
	sess, _ := setupMilitarySession(t, nil)
	err := runArmyMerge(context.Background(), sess, []string{"Garrison"}, map[string]string{"into": "Garrison"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "into itself")
}

func TestRunArmyMerge_surfacesIncompatibleArmies(t *testing.T) {
	// Drive the MergeArmy wrapper directly with a 422 — the verb
	// runs selector.Confirm before reaching the API, which a test
	// cannot exercise.
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-2/merge": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			// The OpenAPI ErrorEnvelope.code enum is the small set
			// listed in gen/oas_schemas_gen.go; the spec uses
			// `invalid` as the umbrella 422 code and packs the
			// specific reason ("incompatible_armies") into message.
			// Backend co-evolution candidate: expand the enum so the
			// CLI can switch on the specific cause.
			_, _ = w.Write([]byte(`{"error": {"code": "invalid", "message": "incompatible_armies"}}`))
		},
	}
	sess, _ := setupMilitarySession(t, extra)
	_, err := sess.API.MergeArmy(context.Background(), "arm-2", "arm-1")
	require.Error(t, err)
	apiErr := api.AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "invalid", apiErr.Code)
	require.Contains(t, apiErr.Message, "incompatible_armies")
}

func TestCompositionString(t *testing.T) {
	require.Equal(t, "(empty)", compositionString(nil))
	require.Equal(t, "(empty)", compositionString(map[string]int{"levy": 0}))
	require.Equal(t, "archer=2, levy=5", compositionString(map[string]int{"levy": 5, "archer": 2}))
}
