package wonders

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func TestRenderResourceMap(t *testing.T) {
	require.Equal(t, "(none)", renderResourceMap(nil))
	require.Equal(t, "(none)", renderResourceMap(map[string]int{}))
	require.Equal(t, "(none)", renderResourceMap(map[string]int{"gold": 0}))
	// Canonical order: gold → wood → stone → iron.
	require.Equal(t, "gold=200000 wood=150000 stone=600000 iron=200000",
		renderResourceMap(map[string]int{"iron": 200000, "stone": 600000, "gold": 200000, "wood": 150000}))
	// Unknown keys appear after the canonical order, alphabetically.
	require.Equal(t, "gold=1 wood=2 stone=3 iron=4 amber=5 platinum=6",
		renderResourceMap(map[string]int{
			"gold": 1, "wood": 2, "stone": 3, "iron": 4,
			"platinum": 6, "amber": 5,
		}))
}

func TestRenderMilestones(t *testing.T) {
	none := gen.WonderMilestonesPaid{}
	require.Equal(t, "25=pending  50=pending  75=pending", renderMilestones(none))

	paid25 := gen.WonderMilestonesPaid{R25: gen.OptBool{Value: true, Set: true}}
	require.Equal(t, "25=paid  50=pending  75=pending", renderMilestones(paid25))

	all := gen.WonderMilestonesPaid{
		R25: gen.OptBool{Value: true, Set: true},
		R50: gen.OptBool{Value: true, Set: true},
		R75: gen.OptBool{Value: true, Set: true},
	}
	require.Equal(t, "25=paid  50=paid  75=paid", renderMilestones(all))
}

func TestHpPct(t *testing.T) {
	require.Equal(t, 42, hpPct(4200, 10000))
	require.Equal(t, 0, hpPct(0, 10000))
	require.Equal(t, 100, hpPct(10000, 10000))
	// Clamping.
	require.Equal(t, 100, hpPct(15000, 10000))
	require.Equal(t, 0, hpPct(-5, 10000))
	// Defensive zero-target guard.
	require.Equal(t, 0, hpPct(50, 0))
}

func TestPrintWonderDetail_construction(t *testing.T) {
	sess := &shell.Session{Out: &bytes.Buffer{}}
	w := &gen.Wonder{
		ID: "wnd-1", KingdomID: "kgd-1", WorldID: "wld-1",
		Name:   gen.WonderName("sky_tower"),
		Status: gen.WonderStatusConstruction,
		Hp:     4200, TargetHp: 10000,
		MilestonesPaid: gen.WonderMilestonesPaid{
			R25: gen.OptBool{Value: true, Set: true},
		},
		RepairedHpByPhase: gen.WonderRepairedHpByPhase{
			Construction: gen.OptInt{Value: 500, Set: true},
		},
		StartedAt: gen.OptDateTime{Value: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Set: true},
		ConstructionStartedAt: gen.OptNilDateTime{
			Value: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Set: true,
		},
	}
	printWonderDetail(sess, w)
	got := sess.Out.(*bytes.Buffer).String()

	require.Contains(t, got, "Sky Tower")
	require.Contains(t, got, "(construction)")
	require.Contains(t, got, "id:        wnd-1")
	require.Contains(t, got, "hp:        4200/10000  (42%)")
	require.Contains(t, got, "milestones: 25=paid  50=pending  75=pending")
	require.Contains(t, got, "repaired this phase: foundation=0 construction=500 consecration=0")
	require.Contains(t, got, "started:        2026-05-01 00:00 UTC")
	require.NotContains(t, got, "pending milestone:") // no pending payment in this scenario
	require.NotContains(t, got, "paused until:")
}

func TestPrintWonderDetail_pendingMilestone(t *testing.T) {
	sess := &shell.Session{Out: &bytes.Buffer{}}
	w := &gen.Wonder{
		ID: "wnd-1", Name: gen.WonderName("eternal_citadel"),
		Status: gen.WonderStatusConstruction,
		Hp:     2500, TargetHp: 10000,
		PendingMilestonePercent: gen.OptNilWonderPendingMilestonePercent{
			Value: gen.WonderPendingMilestonePercent25, Set: true,
		},
		PendingMilestoneCost: gen.OptNilWonderPendingMilestoneCost{
			Value: gen.WonderPendingMilestoneCost(map[string]int{
				"gold": 80000, "wood": 60000, "stone": 240000, "iron": 80000,
			}),
			Set: true,
		},
	}
	printWonderDetail(sess, w)
	got := sess.Out.(*bytes.Buffer).String()

	require.Contains(t, got, "Eternal Citadel")
	require.Contains(t, got, "pending milestone:")
	require.Contains(t, got, "percent: 25%")
	require.Contains(t, got, "cost:    gold=80000 wood=60000 stone=240000 iron=80000")
	require.Contains(t, got, "pay with `wonder milestone 25`")
}

func TestPrintWonderList_empty(t *testing.T) {
	sess := &shell.Session{Out: &bytes.Buffer{}}
	printWonderList(sess, nil)
	got := sess.Out.(*bytes.Buffer).String()
	require.Contains(t, got, "Wonders:")
	require.Contains(t, got, "(none)")
}

func TestPrintWonderList_populated(t *testing.T) {
	sess := &shell.Session{Out: &bytes.Buffer{}}
	items := []gen.WonderListItem{
		{
			ID: "wnd-1", KingdomID: "kgd-1", BuilderHandle: "IronFist",
			Name:   "sky_tower",
			Status: gen.WonderListItemStatusConstruction,
			Hp:     4200, TargetHp: 10000, HpPct: 42,
			StartedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "wnd-2", KingdomID: "kgd-2", BuilderHandle: "ShadowWolf",
			Name:   "eternal_citadel",
			Status: gen.WonderListItemStatusFoundation,
			Hp:     1000, TargetHp: 1000, HpPct: 100,
			StartedAt: time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC),
		},
	}
	printWonderList(sess, items)
	got := sess.Out.(*bytes.Buffer).String()

	require.Contains(t, got, "IronFist")
	require.Contains(t, got, "Sky Tower")
	require.Contains(t, got, "construction")
	require.Contains(t, got, "hp=4200/10000 (42%)")
	require.Contains(t, got, "started 2026-05-01 00:00 UTC")
	require.Contains(t, got, "ShadowWolf")
	require.Contains(t, got, "Eternal Citadel")
	require.Contains(t, got, "foundation")
}
