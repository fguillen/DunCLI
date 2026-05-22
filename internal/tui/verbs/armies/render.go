// Package armies hosts the Phase 8 military verbs: training (train,
// train preview, train cancel), army management (armies, army show,
// army split, army rename, army merge) and march dispatch / recall.
// They register into the shared shell.Registry from each file's init().
package armies

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// compositionString renders a stable, single-line composition summary
// like "levy=12, archer=4". Empty maps come out as "(empty)".
func compositionString(c gen.Composition) string {
	if len(c) == 0 {
		return "(empty)"
	}
	keys := make([]string, 0, len(c))
	for k, v := range c {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "(empty)"
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, c[k]))
	}
	return strings.Join(parts, ", ")
}

// stockpileString formats a StockpileMap as "gold=N wood=N stone=N iron=N".
func stockpileString(s gen.StockpileMap) string {
	return fmt.Sprintf("gold=%d wood=%d stone=%d iron=%d", s.Gold, s.Wood, s.Stone, s.Iron)
}

// printTrainPreview is the analog of kingdom.go's printPreview for the
// training endpoint.
func printTrainPreview(sess *shell.Session, p *gen.TrainingPreview) {
	shell.Strong(sess.Out, fmt.Sprintf("%s training preview", string(p.Unit)))
	_, _ = fmt.Fprintf(sess.Out, "  at:        %s (L%d)\n", string(p.BuildingKind), p.BuildingLevel)
	if !p.BuildingBuilt {
		_, _ = fmt.Fprintln(sess.Out, "  building:  not yet built")
	}
	if !p.UnitTrainableHere {
		_, _ = fmt.Fprintln(sess.Out, "  warning:   unit may not be trainable at this building")
	}
	_, _ = fmt.Fprintf(sess.Out, "  count:     %d\n", p.Count)
	_, _ = fmt.Fprintf(sess.Out, "  per-unit:  %s in %s\n", stockpileString(p.PerUnitCost),
		shared.RelTime(time.Now().Add(time.Duration(p.PerUnitSeconds)*time.Second)))
	_, _ = fmt.Fprintf(sess.Out, "  total:     %s in %s\n", stockpileString(p.TotalCost),
		shared.RelTime(time.Now().Add(time.Duration(p.TotalSeconds)*time.Second)))
	if p.Affordable {
		_, _ = fmt.Fprintln(sess.Out, "  affords:   yes")
	} else {
		_, _ = fmt.Fprintf(sess.Out, "  affords:   no (missing %s)\n", stockpileString(p.Missing))
	}
	_, _ = fmt.Fprintf(sess.Out, "  max afford: %d\n", p.MaxAffordableCount)
}

// compactDuration renders a whole-second count as a short label
// ("45s", "3m", "1m30s", "1h15m") for catalog tables.
func compactDuration(seconds int) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		m, s := seconds/60, seconds%60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		h, m := seconds/3600, (seconds%3600)/60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
}

// printTrainCatalog renders `train preview` / `train preview <building>`:
// one block per military building listing each trainable unit's per-unit
// cost, per-unit training time, and the count the current stockpile
// affords. Units the building can't currently train are tagged (locked).
func printTrainCatalog(sess *shell.Session, c *gen.TrainingCatalog) {
	if len(c.Buildings) == 0 {
		shell.Section(sess.Out, "Training catalog:", "")
		return
	}
	for i, b := range c.Buildings {
		if i > 0 {
			_, _ = fmt.Fprintln(sess.Out)
		}
		header := string(b.BuildingKind)
		if b.BuildingBuilt {
			header += fmt.Sprintf("  (L%d)", b.BuildingLevel)
		} else {
			header += "  (not built)"
		}
		shell.Strong(sess.Out, header)
		if len(b.Units) == 0 {
			_, _ = fmt.Fprintln(sess.Out, "  (no trainable units)")
			continue
		}
		units := make([]gen.TrainingCatalogUnit, len(b.Units))
		copy(units, b.Units)
		sort.Slice(units, func(i, j int) bool { return string(units[i].Unit) < string(units[j].Unit) })
		for _, u := range units {
			line := fmt.Sprintf("  %-12s %-40s %7s   max %d",
				string(u.Unit), stockpileString(u.PerUnitCost),
				compactDuration(u.PerUnitSeconds), u.MaxAffordableCount)
			if !u.Trainable {
				line += "   (locked)"
			}
			_, _ = fmt.Fprintln(sess.Out, line)
		}
	}
}

// printArmy is the long form of one army for `army show`.
func printArmy(sess *shell.Session, a *gen.Army, regionName string) {
	shell.Strong(sess.Out, a.Name+"  ("+string(a.Status)+")")
	loc := regionName
	if loc == "" {
		loc = a.LocationRegionID
	}
	_, _ = fmt.Fprintf(sess.Out, "  id:        %s\n", a.ID)
	_, _ = fmt.Fprintf(sess.Out, "  location:  %s\n", loc)
	_, _ = fmt.Fprintf(sess.Out, "  capacity:  %d\n", a.TotalCapacity)
	if len(a.Composition) == 0 {
		shell.Section(sess.Out, "Composition:", "")
		return
	}
	keys := make([]string, 0, len(a.Composition))
	for k, v := range a.Composition {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%-12s %d", k, a.Composition[k]))
	}
	shell.Section(sess.Out, "Composition:", strings.Join(lines, "\n"))
}

// printArmyList renders the `armies` table.
func printArmyList(sess *shell.Session, list []gen.Army, regionNameByID map[string]string) {
	if len(list) == 0 {
		shell.Section(sess.Out, "Armies:", "")
		return
	}
	rows := make([]gen.Army, len(list))
	copy(rows, list)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	lines := make([]string, 0, len(rows))
	for _, a := range rows {
		loc := regionNameByID[a.LocationRegionID]
		if loc == "" {
			loc = a.LocationRegionID
		}
		lines = append(lines, fmt.Sprintf("%-16s  %-10s  %-12s  cap=%-4d  %s",
			a.Name, string(a.Status), loc, a.TotalCapacity, compositionString(a.Composition)))
	}
	shell.Section(sess.Out, "Armies:", strings.Join(lines, "\n"))
}
