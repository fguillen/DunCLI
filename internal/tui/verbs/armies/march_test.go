package armies

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/api/gen"
)

func TestMarchIntents_pinnedToGenEnum(t *testing.T) {
	want := make([]string, 0)
	for _, v := range gen.MarchOrderIntent("").AllValues() {
		want = append(want, string(v))
	}
	require.ElementsMatch(t, want, marchIntents,
		"marchIntents is out of sync with gen.MarchOrderIntent.AllValues()")
}

func TestRunMarchDispatch_postsTargetAndIntent(t *testing.T) {
	var body map[string]any
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-1/march": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "mrc-1", "army_id": "arm-1", "intent": "scout",
				"origin_region_id": "reg-1", "target_region_id": "reg-2",
				"path": ["reg-1", "reg-2"],
				"dispatched_at": "2026-05-20T00:00:00Z",
				"arrives_at": "2026-05-20T02:00:00Z",
				"arrived_at": null, "recalled_at": null
			}`))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runMarchDispatch(context.Background(), sess, []string{"Garrison", "Ironvale", "scout"}, nil))
	require.Equal(t, "reg-2", body["target_region_id"])
	require.Equal(t, "scout", body["intent"])
	require.Contains(t, out.String(), "march mrc-1")
	// Path should render with region names, not IDs.
	require.Contains(t, out.String(), "Greyhollow → Ironvale")
}

func TestRunMarchDispatch_rejectsUnknownIntent(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
	}
	sess, _ := setupMilitarySession(t, extra)
	err := runMarchDispatch(context.Background(), sess, []string{"Garrison", "Ironvale", "yoink"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown intent "yoink"`)
}

func TestRunRecall_404SurfacesNoActiveMarch(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-1/recall": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	}
	sess, _ := setupMilitarySession(t, extra)
	err := runRecall(context.Background(), sess, []string{"Garrison"}, nil)
	require.Error(t, err)
	apiErr := api.AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
	require.Contains(t, apiErr.Message, "no active march")
}

func TestRunRecall_happyPath(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
		"/v1/armies/arm-2/recall": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "mrc-2", "army_id": "arm-2", "intent": "reinforce",
				"origin_region_id": "reg-2", "target_region_id": "reg-1",
				"path": ["reg-2", "reg-1"],
				"dispatched_at": "2026-05-20T00:00:00Z",
				"arrives_at": "2026-05-20T03:00:00Z",
				"arrived_at": null, "recalled_at": "2026-05-20T01:00:00Z"
			}`))
		},
	}
	sess, out := setupMilitarySession(t, extra)
	require.NoError(t, runRecall(context.Background(), sess, []string{"Vanguard"}, nil))
	got := out.String()
	require.Contains(t, got, "recalled: army Vanguard returning")
	require.Contains(t, got, "Ironvale → Greyhollow")
}

func TestSuggestRecallableArmies_filtersHome(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-7/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(armiesListBody))
		},
	}
	sess, _ := setupMilitarySession(t, extra)
	got, err := suggestRecallableArmies(context.Background(), sess, "")
	require.NoError(t, err)
	require.Equal(t, []string{"Vanguard"}, got,
		"only the marching army should be recallable")
}
