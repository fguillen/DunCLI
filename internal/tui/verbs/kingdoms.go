package verbs

import (
	"context"
	"fmt"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "kingdoms",
		Summary: "List every kingdom in the in-scope world (public roster)",
		Usage:   "kingdoms",
		Run:     runKingdomsList,
	})
}

// runKingdomsList implements the plural `kingdoms` verb — the
// world-scoped public roster of every kingdom: who's playing, their
// home region, territory counts, and wonder progress. Coarse by
// design (no stockpiles/armies/queues — those stay scout-only per
// game-design §16.9). Backend orders entries by join time; we keep
// that order so the roster reads as a turn order.
func runKingdomsList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	entries, err := sess.API.ListWorldKingdoms(ctx, worldID)
	if err != nil {
		return err
	}
	printKingdomRoster(sess, entries)
	return nil
}

// printKingdomRoster renders one line per kingdom. Empty rosters
// collapse to the standard `(none)` placeholder via shell.Section.
func printKingdomRoster(sess *shell.Session, entries []gen.WorldKingdomEntry) {
	if len(entries) == 0 {
		shell.Section(sess.Out, "Kingdoms:", "")
		return
	}
	lines := make([]string, 0, len(entries))
	for _, k := range entries {
		handle := k.Handle
		if k.IsYou {
			handle += " (you)"
		}
		home := "—"
		if v := optString(k.HomeRegionName); v != "" {
			home = v
		}
		wonder := "wonder=—"
		if w, ok := k.Wonder.Get(); ok {
			wonder = fmt.Sprintf("wonder=%s %s %d%%", w.Name, string(w.Status), w.HpPct)
		}
		line := fmt.Sprintf("%-18s  %-16s  nodes=%-3d  ruins=%-2d  %s",
			handle, home, k.NodesControlled, k.RuinsClaimed, wonder)
		if k.Eliminated {
			line += "  eliminated"
		}
		if t := optString(k.Title); t != "" {
			line += "  " + t
		}
		lines = append(lines, line)
	}
	shell.Section(sess.Out, "Kingdoms:", strings.Join(lines, "\n"))
}
