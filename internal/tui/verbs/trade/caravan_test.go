package trade

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func TestRunCaravanSend_rejectsBadUsage(t *testing.T) {
	sess, _ := setupTradeSession(t, nil)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, "usage: caravan send <receiver-handle>"},
		{"too many args", []string{"a", "b"}, "usage: caravan send <receiver-handle>"},
		{"empty handle", []string{"   "}, "receiver handle is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runCaravanSend(context.Background(), sess, tc.args, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRunCaravanSend_requiresWorldScope(t *testing.T) {
	sess, _, _ := newTestSession(t, tradeHandler(t, nil))
	err := runCaravanSend(context.Background(), sess, []string{"ShadowWolf"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunCaravanSend_errorsWhenNoHomeArmies(t *testing.T) {
	// Single army present, but status=marching → no home armies left
	// for the caravan to escort.
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/armies": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"armies": [{
					"id": "arm-1", "kingdom_id": "kgd-1", "world_id": "wld-1",
					"name": "Vanguard", "status": "marching",
					"location_region_id": "reg-1", "total_capacity": 40,
					"composition": {"knight": 4}
				}]
			}`))
		},
	}
	sess, _ := setupTradeSession(t, extra)
	err := runCaravanSend(context.Background(), sess, []string{"ShadowWolf"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no home armies")
}

// TestDispatchCaravan_happyPath drives the underlying api.Client
// wrapper directly because the verb interleaves a confirm + form that
// the test runner can't push through. Verifies request body + response
// rendering via printCaravanOrder.
func TestDispatchCaravan_happyPath(t *testing.T) {
	var seenBody []byte
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/caravans": func(w http.ResponseWriter, r *http.Request) {
			seenBody = readBody(t, r)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "car-1", "world_id": "wld-1",
				"sender_kingdom_id": "kgd-1", "receiver_kingdom_id": "kgd-2",
				"origin_region_id": "reg-1", "destination_region_id": "reg-2",
				"payload": {"gold": 100, "wood": 50},
				"escort_units": {"levy": 5, "archer": 2},
				"status": "in_transit",
				"dispatched_at": "2026-05-21T12:00:00Z",
				"arrives_at": "2030-05-21T15:00:00Z",
				"outbound_march_order_id": "mrc-1"
			}`))
		},
	}
	sess, out := setupTradeSession(t, extra)
	c, err := sess.API.DispatchCaravan(
		context.Background(),
		"kgd-1", "ShadowWolf", "arm-1",
		map[string]int{"gold": 100, "wood": 50},
		map[string]int{"levy": 5, "archer": 2},
	)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, "car-1", c.ID)
	require.Equal(t, gen.CaravanStatusInTransit, c.Status)
	require.Contains(t, string(seenBody), `"receiver_handle":"ShadowWolf"`)
	require.Contains(t, string(seenBody), `"source_army_id":"arm-1"`)

	// Render the success block and check the formatting.
	regionNames := map[string]string{"reg-1": "Greyhollow", "reg-2": "Ironvale"}
	printCaravanOrder(sess, c, regionNames)
	got := out.String()
	require.Contains(t, got, "caravan car-1  (in_transit)")
	require.Contains(t, got, "Greyhollow → Ironvale")
	require.Contains(t, got, "payload:   gold=100 wood=50")
	require.Contains(t, got, "escort:    archer=2, levy=5")
	require.Contains(t, got, "march:     mrc-1")
}

func TestDispatchCaravan_surfaces422(t *testing.T) {
	cases := []struct {
		code string
		body string
	}{
		{"invalid", `{"error":{"code":"invalid","message":"insufficient_capacity"}}`},
		{"invalid", `{"error":{"code":"invalid","message":"army_not_home"}}`},
		{"invalid", `{"error":{"code":"invalid","message":"cross_world"}}`},
		{"not_found", `{"error":{"code":"not_found","message":"receiver_not_found"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.code+"-"+tc.body, func(t *testing.T) {
			status := http.StatusUnprocessableEntity
			if tc.code == "not_found" {
				status = http.StatusNotFound
			}
			extra := map[string]http.HandlerFunc{
				"/v1/kingdoms/kgd-1/caravans": func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = w.Write([]byte(tc.body))
				},
			}
			sess, _ := setupTradeSession(t, extra)
			_, err := sess.API.DispatchCaravan(
				context.Background(),
				"kgd-1", "ShadowWolf", "arm-1",
				map[string]int{"gold": 1},
				map[string]int{"levy": 1},
			)
			require.Error(t, err)
			apiErr := api.AsError(err)
			require.NotNil(t, apiErr)
			require.Equal(t, tc.code, apiErr.Code)
		})
	}
}

func TestPrintCaravanPreview(t *testing.T) {
	src := gen.Army{
		ID: "arm-1", Name: "Garrison",
		LocationRegionID: "reg-1",
		Composition:      gen.Composition{"archer": 3, "levy": 12},
		TotalCapacity:    60,
	}
	sess := &shell.Session{Out: &bytes.Buffer{}}
	printCaravanPreview(sess, "ShadowWolf", src, "Greyhollow",
		map[string]int{"gold": 100},
		map[string]int{"levy": 5})
	got := sess.Out.(*bytes.Buffer).String()
	require.Contains(t, got, "caravan dispatch preview")
	require.Contains(t, got, "target:    ShadowWolf")
	require.Contains(t, got, "payload:   gold=100")
	require.Contains(t, got, "escort:    levy=5")
	require.Contains(t, got, "from:      Garrison at Greyhollow")
}

func TestFormatPayload(t *testing.T) {
	require.Equal(t, "(none)", formatPayload(nil))
	require.Equal(t, "(none)", formatPayload(map[string]int{"gold": 0}))
	require.Equal(t, "gold=100 stone=5",
		formatPayload(map[string]int{"gold": 100, "stone": 5, "wood": 0}))
	// Canonical resource order: gold, wood, stone, iron — not alphabetical.
	require.Equal(t, "gold=1 wood=2 stone=3 iron=4",
		formatPayload(map[string]int{"iron": 4, "gold": 1, "stone": 3, "wood": 2}))
}

func TestFormatComposition(t *testing.T) {
	require.Equal(t, "(empty)", formatComposition(nil))
	require.Equal(t, "(empty)", formatComposition(map[string]int{"levy": 0}))
	require.Equal(t, "archer=2, levy=5",
		formatComposition(map[string]int{"levy": 5, "archer": 2}))
}

func TestFilterHomeArmies(t *testing.T) {
	in := []gen.Army{
		{Name: "Vanguard", Status: gen.ArmyStatusMarching},
		{Name: "Garrison", Status: gen.ArmyStatusHome},
		{Name: "Alpha", Status: gen.ArmyStatusHome},
		{Name: "Returning", Status: gen.ArmyStatusReturning},
	}
	out := filterHomeArmies(in)
	require.Len(t, out, 2)
	require.Equal(t, "Alpha", out[0].Name) // sorted by name
	require.Equal(t, "Garrison", out[1].Name)
}

func TestVerbsRegistered_caravan(t *testing.T) {
	parent, ok := shell.Resolve("caravan")
	require.True(t, ok)
	send, ok := parent.Sub["send"]
	require.True(t, ok)
	require.NotNil(t, send.Run)
}

// readBody slurps a request body for assertion. Defined locally so the
// test stays self-contained.
func readBody(t *testing.T, r *http.Request) []byte {
	t.Helper()
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	require.NoError(t, err)
	return buf.Bytes()
}
