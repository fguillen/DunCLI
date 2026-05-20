package battles

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// One PvP battle (defender_kingdom_id non-empty) and one wilderness
// battle (defender_kingdom_id="") so render decisions both fire.
const battlesListBody = `{
	"battles": [
		{
			"id": "bat-9", "world_id": "wld-1", "region_id": "reg-1",
			"attacker_kingdom_id": "kgd-1", "defender_kingdom_id": "",
			"outcome": "attacker_victory",
			"loot": {"gold": 120, "wood": 40},
			"log": [{"round": 1, "attacker_damage_dealt": 40, "defender_damage_dealt": 10,
			         "attacker_casualties": {"levy": 2},
			         "defender_casualties": {"pikeman": 4}}],
			"started_at": "2026-05-19T21:00:00Z",
			"ended_at":   "2026-05-19T21:30:00Z"
		},
		{
			"id": "bat-7", "world_id": "wld-1", "region_id": "reg-2",
			"attacker_kingdom_id": "kgd-2", "defender_kingdom_id": "kgd-1",
			"outcome": "defender_rout",
			"loot": {},
			"log": [{"round": 1, "attacker_damage_dealt": 5, "defender_damage_dealt": 20,
			         "attacker_casualties": {"knight": 3},
			         "defender_casualties": {}}],
			"started_at": "2026-05-18T14:00:00Z",
			"ended_at":   "2026-05-18T14:02:00Z"
		}
	],
	"total_count": 2
}`

func TestRunBattlesList_rendersWildernessMarkerAndOpponentID(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/battles": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(battlesListBody))
		},
	}
	sess, out := setupBattlesSession(t, extra)
	require.NoError(t, runBattlesList(context.Background(), sess, nil, nil))
	got := out.String()
	require.Contains(t, got, "Battles (showing 1-2 of 2):")
	require.Contains(t, got, "bat-9")
	require.Contains(t, got, "(wilderness)") // defender empty → marker
	// PvP row: caller is the defender (kgd-1), so the opponent rendered
	// is the attacker (kgd-2). Short test IDs are kept verbatim.
	require.Contains(t, got, "vs kgd-2")
	require.Contains(t, got, "Greyhollow") // region name resolved via showWorldMap
	require.Contains(t, got, "Ironvale")
	require.Contains(t, got, "loot:gold=120 wood=40")
	require.Contains(t, got, "loot:(none)")
}

func TestRunBattlesList_emptyKingdom(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/battles": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"battles": [], "total_count": 0}`))
		},
	}
	sess, out := setupBattlesSession(t, extra)
	require.NoError(t, runBattlesList(context.Background(), sess, nil, nil))
	require.Contains(t, out.String(), "Battles:")
	require.Contains(t, out.String(), "(none)")
}

func TestRunBattlesList_propagatesPaginationAndShowsMoreHint(t *testing.T) {
	var seen url.Values
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/battles": func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Query()
			_, _ = w.Write([]byte(`{
				"battles": [{"id": "bat-9", "world_id": "wld-1", "region_id": "reg-1",
				 "attacker_kingdom_id": "kgd-1", "defender_kingdom_id": "kgd-2",
				 "outcome": "attacker_victory", "loot": {}, "log": [],
				 "started_at": "2026-05-19T21:00:00Z",
				 "ended_at":   "2026-05-19T21:30:00Z"}],
				"total_count": 137
			}`))
		},
	}
	sess, out := setupBattlesSession(t, extra)
	err := runBattlesList(context.Background(), sess, nil, map[string]string{"limit": "5", "offset": "10"})
	require.NoError(t, err)
	require.Equal(t, "5", seen.Get("limit"))
	require.Equal(t, "10", seen.Get("offset"))
	require.Contains(t, out.String(), "showing 11-11 of 137")
	require.Contains(t, out.String(), "more: 126 remaining")
	require.Contains(t, out.String(), "--limit 5 --offset 11")
}

func TestRunBattlesList_rejectsBadFlags(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string]string
		want  string
	}{
		{"limit non-numeric", map[string]string{"limit": "abc"}, "--limit must be a positive integer"},
		{"limit zero", map[string]string{"limit": "0"}, "--limit must be a positive integer"},
		{"limit too large", map[string]string{"limit": "999"}, "--limit must be ≤ 100"},
		{"offset negative", map[string]string{"offset": "-1"}, "--offset must be a non-negative integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess, _ := setupBattlesSession(t, nil)
			err := runBattlesList(context.Background(), sess, nil, tc.flags)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRunBattleShow_rendersFullDetailAndYouMarker(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/battles/bat-7": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"battle": {
					"id": "bat-7", "world_id": "wld-1", "region_id": "reg-2",
					"attacker_kingdom_id": "kgd-2", "defender_kingdom_id": "kgd-1",
					"attacker_title": "Champion",
					"march_order_id": "mrc-42",
					"outcome": "defender_rout",
					"loot": {"gold": 5, "iron": 1},
					"log": [
						{"round": 1, "attacker_atk": 120, "attacker_def": 80,
						 "defender_atk": 60, "defender_def": 100,
						 "attacker_damage_dealt": 40, "defender_damage_dealt": 10,
						 "attacker_casualties": {"levy": 2},
						 "defender_casualties": {"archer": 4, "levy": 6}},
						{"round": 2, "attacker_damage_dealt": 38, "defender_damage_dealt": 8,
						 "attacker_casualties": {"levy": 2},
						 "defender_casualties": {"archer": 3, "levy": 5},
						 "walls_damage": 15, "walls_level_after": 2}
					],
					"started_at": "2026-05-18T14:00:00Z",
					"ended_at":   "2026-05-18T14:30:00Z"
				},
				"participants": [
					{"id": "p-1", "battle_id": "bat-7", "kingdom_id": "kgd-2",
					 "side": "attacker", "army_id": "arm-2",
					 "starting_composition": {"knight": 4},
					 "ending_composition": {},
					 "casualties": {"knight": 4}},
					{"id": "p-2", "battle_id": "bat-7", "kingdom_id": "kgd-1",
					 "side": "defender", "army_id": "arm-1",
					 "starting_composition": {"archer": 10, "levy": 20},
					 "ending_composition": {"archer": 7, "levy": 14},
					 "casualties": {"archer": 3, "levy": 6}}
				]
			}`))
		},
	}
	sess, out := setupBattlesSession(t, extra)
	require.NoError(t, runBattleShow(context.Background(), sess, []string{"bat-7"}, nil))
	got := out.String()
	require.Contains(t, got, "Battle bat-7")
	require.Contains(t, got, "Ironvale")
	require.Contains(t, got, "outcome:    defender_rout")
	require.Contains(t, got, "titles:     attacker=Champion  defender=—")
	require.Contains(t, got, "march:      mrc-42")
	require.Contains(t, got, "loot:       gold=5 iron=1")
	require.Contains(t, got, "attacker  kgd-2")
	require.Contains(t, got, "defender  kgd-1 (you)") // caller marker
	require.Contains(t, got, "starting:   knight=4")
	require.Contains(t, got, "ending:     archer=7, levy=14")
	require.Contains(t, got, "Round 1")
	require.Contains(t, got, "atk:        atk=120  def=80")
	require.Contains(t, got, "def:        atk=60  def=100")
	require.Contains(t, got, "damage:     attacker→defender=40  defender→attacker=10")
	require.Contains(t, got, "casualties: attacker (levy=2)  defender (archer=4, levy=6)")
	require.Contains(t, got, "Round 2")
	require.Contains(t, got, "walls:      damage=15  walls_level_after=2")
}

func TestRunBattleShow_wildernessRendersMarker(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/battles/bat-9": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"battle": {
					"id": "bat-9", "world_id": "wld-1", "region_id": "reg-1",
					"attacker_kingdom_id": "kgd-1", "defender_kingdom_id": "",
					"outcome": "attacker_victory",
					"loot": {},
					"log": [{"round": 1, "attacker_damage_dealt": 4, "defender_damage_dealt": 1,
					         "attacker_casualties": {}, "defender_casualties": {"pikeman": 2}}],
					"started_at": "2026-05-19T21:00:00Z",
					"ended_at":   "2026-05-19T21:30:00Z"
				},
				"participants": [
					{"id": "p-1", "battle_id": "bat-9", "kingdom_id": "kgd-1",
					 "side": "attacker", "army_id": "arm-1",
					 "starting_composition": {"levy": 10},
					 "ending_composition": {"levy": 9},
					 "casualties": {"levy": 1}}
				]
			}`))
		},
	}
	sess, out := setupBattlesSession(t, extra)
	require.NoError(t, runBattleShow(context.Background(), sess, []string{"bat-9"}, nil))
	got := out.String()
	require.Contains(t, got, "Battle bat-9")
	require.Contains(t, got, "loot:       (none)")
	require.Contains(t, got, "attacker  kgd-1 (you)")
	// Single participant means only the attacker side renders; the
	// wilderness marker shows up in the list, not here. Sanity-check
	// that the rendered output doesn't accidentally show "(wilderness)
	// (you)" on the caller.
	require.NotContains(t, got, "(wilderness) (you)")
}

func TestRunBattleShow_404SurfacesAsError(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/battles/bat-x": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": {"code": "not_found", "message": "no such battle"}}`))
		},
	}
	sess, _ := setupBattlesSession(t, extra)
	err := runBattleShow(context.Background(), sess, []string{"bat-x"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such battle")
}

func TestRunBattleShow_usageError(t *testing.T) {
	sess, _ := setupBattlesSession(t, nil)
	err := runBattleShow(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: battle show <id>")
}

func TestRunBattles_requiresKingdomScope(t *testing.T) {
	// Fresh session with no server scope set.
	sess, _, _ := newTestSession(t, battlesHandler(t, nil))
	err := runBattlesList(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestSuggestBattleIDs_returnsIDsFromList(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/kingdoms/kgd-1/battles": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(battlesListBody))
		},
	}
	sess, _ := setupBattlesSession(t, extra)
	ids, err := suggestBattleIDs(context.Background(), sess, "")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"bat-9", "bat-7"}, ids)
}

func TestShortKingdom(t *testing.T) {
	require.Equal(t, "(wilderness)", shortKingdom(""))
	require.Equal(t, "kgd-7", shortKingdom("kgd-7"))
	require.Equal(t, "01JQ…6KZA", shortKingdom("01JQABCDEFGHJKMNPQRST6KZA"))
}

func TestLootString(t *testing.T) {
	require.Equal(t, "(none)", lootString(nil))
	require.Equal(t, "(none)", lootString(gen.BattleLoot{"gold": 0}))
	require.Equal(t, "gold=10 wood=2 stone=1 iron=3",
		lootString(gen.BattleLoot{"iron": 3, "gold": 10, "stone": 1, "wood": 2}))
}

// Sanity: the verbs register themselves and are reachable via
// shell.Resolve so the blank import in main.go is enough to wire them
// up. Mirror of armies_test patterns.
func TestVerbsRegistered(t *testing.T) {
	v, ok := shell.Resolve("battles")
	require.True(t, ok)
	require.NotNil(t, v.Run)
	require.True(t, hasFlag(v, "limit"))
	require.True(t, hasFlag(v, "offset"))

	parent, ok := shell.Resolve("battle")
	require.True(t, ok)
	show, ok := parent.Sub["show"]
	require.True(t, ok)
	require.NotNil(t, show.Run)
	require.NotNil(t, show.Complete)
}

func hasFlag(v *shell.Verb, name string) bool {
	for _, f := range v.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

// Sanity assertion that BattleOutcome enum values still match the
// strings we render verbatim. If the spec drifts, this test pings the
// renderer authors.
func TestOutcomeEnumPinned(t *testing.T) {
	for _, o := range gen.BattleOutcome("").AllValues() {
		s := string(o)
		require.False(t, strings.ContainsAny(s, " \t"), "outcome %q should be printable verbatim", s)
	}
}
