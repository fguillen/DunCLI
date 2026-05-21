package wonders

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// resourceOrder is the canonical render order shared with kingdom /
// trade verbs (gold → wood → stone → iron). Keys outside this set are
// appended in alphabetical order for forward-compat.
var resourceOrder = []string{"gold", "wood", "stone", "iron"}

// renderResourceMap pretty-prints a per-resource cost map. Empty maps
// collapse to "(none)".
func renderResourceMap(m map[string]int) string {
	if len(m) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(m))
	seen := map[string]bool{}
	for _, r := range resourceOrder {
		if v, ok := m[r]; ok && v > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", r, v))
			seen[r] = true
		}
	}
	extras := make([]string, 0)
	for k, v := range m {
		if seen[k] || v <= 0 {
			continue
		}
		extras = append(extras, fmt.Sprintf("%s=%d", k, v))
	}
	sort.Strings(extras)
	parts = append(parts, extras...)
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " ")
}

// renderMilestones renders the 25/50/75 progress block.
func renderMilestones(p gen.WonderMilestonesPaid) string {
	mark := func(v gen.OptBool) string {
		paid, ok := v.Get()
		if ok && paid {
			return "paid"
		}
		return "pending"
	}
	return fmt.Sprintf("25=%s  50=%s  75=%s", mark(p.R25), mark(p.R50), mark(p.R75))
}

// hpPct returns a percentage clamped to [0, 100], rendered as an int.
func hpPct(hp, target int) int {
	if target <= 0 {
		return 0
	}
	p := (hp * 100) / target
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// printWonderDetail renders the block used by `wonder show` and by
// the success paths of `wonder start` / `wonder repair` /
// `wonder milestone` / `wonder cancel`. Optional fields collapse when
// unset; the pending-milestone callout only prints when something is
// actually pending.
func printWonderDetail(sess *shell.Session, w *gen.Wonder) {
	shell.Strong(sess.Out, fmt.Sprintf("%s  (%s)", titleName(string(w.Name)), string(w.Status)))
	_, _ = fmt.Fprintf(sess.Out, "  id:        %s\n", w.ID)
	_, _ = fmt.Fprintf(sess.Out, "  builder:   (you)\n")
	_, _ = fmt.Fprintf(sess.Out, "  hp:        %d/%d  (%d%%)\n", w.Hp, w.TargetHp, hpPct(w.Hp, w.TargetHp))
	_, _ = fmt.Fprintf(sess.Out, "  milestones: %s\n", renderMilestones(w.MilestonesPaid))

	if pp, ok := w.PendingMilestonePercent.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  pending milestone:\n")
		_, _ = fmt.Fprintf(sess.Out, "    percent: %d%%\n", int(pp))
		if pc, ok := w.PendingMilestoneCost.Get(); ok {
			_, _ = fmt.Fprintf(sess.Out, "    cost:    %s\n", renderResourceMap(map[string]int(pc)))
		}
		_, _ = fmt.Fprintf(sess.Out, "    note:    pay with `wonder milestone %d` to resume construction\n", int(pp))
	}

	if pu, ok := w.PausedUntil.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  paused until: %s  (%s)\n",
			pu.UTC().Format("2006-01-02 15:04 UTC"), shared.RelTime(pu))
	}

	r := w.RepairedHpByPhase
	f, _ := r.Foundation.Get()
	c, _ := r.Construction.Get()
	co, _ := r.Consecration.Get()
	if f > 0 || c > 0 || co > 0 {
		_, _ = fmt.Fprintf(sess.Out, "  repaired this phase: foundation=%d construction=%d consecration=%d  (cap 2000 each)\n",
			f, c, co)
	}

	if t, ok := w.StartedAt.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  started:        %s\n", t.UTC().Format("2006-01-02 15:04 UTC"))
	}
	if t, ok := w.ConstructionStartedAt.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  construction:   %s\n", t.UTC().Format("2006-01-02 15:04 UTC"))
	}
	if t, ok := w.ConsecrationAt.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  consecration:   %s\n", t.UTC().Format("2006-01-02 15:04 UTC"))
	}
	if t, ok := w.CompletedAt.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  completed:      %s\n", t.UTC().Format("2006-01-02 15:04 UTC"))
	}
	if t, ok := w.DestroyedAt.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  destroyed:      %s\n", t.UTC().Format("2006-01-02 15:04 UTC"))
	}
}

// printWonderList renders the world-wide list. Empty lists collapse to
// the standard `(none)` placeholder via shell.Section.
func printWonderList(sess *shell.Session, items []gen.WonderListItem) {
	if len(items) == 0 {
		shell.Section(sess.Out, "Wonders:", "")
		return
	}
	lines := make([]string, 0, len(items))
	for _, w := range items {
		started := "—"
		if !w.StartedAt.IsZero() {
			started = w.StartedAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		lines = append(lines, fmt.Sprintf("%-14s  %-18s  %-12s  hp=%d/%d (%d%%)  started %s",
			w.BuilderHandle,
			titleName(w.Name),
			string(w.Status),
			w.Hp, w.TargetHp, w.HpPct,
			started,
		))
	}
	shell.Section(sess.Out, "Wonders:", strings.Join(lines, "\n"))
}
