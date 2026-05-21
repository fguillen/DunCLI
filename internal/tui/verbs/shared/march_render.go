package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// RegionNameMap resolves region ID → name for the in-scope world.
// Returns an empty map on any error so callers can fall back to raw
// IDs without complaining (the user still sees something usable).
func RegionNameMap(ctx context.Context, sess *shell.Session) map[string]string {
	worldID, err := RequireWorldID(ctx, sess)
	if err != nil {
		return map[string]string{}
	}
	regions, err := sess.API.ShowWorldMap(ctx, worldID)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(regions))
	for _, r := range regions {
		out[r.ID] = r.Name
	}
	return out
}

// LookupRegionName resolves one region ID → name, returning "" when
// the ID is empty or unknown.
func LookupRegionName(ctx context.Context, sess *shell.Session, regionID string) string {
	if regionID == "" {
		return ""
	}
	return RegionNameMap(ctx, sess)[regionID]
}

// PathNames resolves a march path ([]regionID) into a parallel slice
// of region names, falling back to the raw ID for any region that
// can't be resolved (e.g. another world).
func PathNames(ctx context.Context, sess *shell.Session, path []string) []string {
	if len(path) == 0 {
		return nil
	}
	names := RegionNameMap(ctx, sess)
	out := make([]string, len(path))
	for i, id := range path {
		if n, ok := names[id]; ok {
			out[i] = n
		} else {
			out[i] = id
		}
	}
	return out
}

// PrintMarchOrder renders one dispatched / recalled march into
// scrollback. pathNames is the parallel slice from PathNames; pass nil
// to render the raw region IDs from m.Path.
func PrintMarchOrder(sess *shell.Session, m *gen.MarchOrder, pathNames []string) {
	shell.Strong(sess.Out, fmt.Sprintf("march %s  (%s)", m.ID, string(m.Intent)))
	_, _ = fmt.Fprintf(sess.Out, "  army:      %s\n", m.ArmyID)
	_, _ = fmt.Fprintf(sess.Out, "  arrives:   %s  (ETA %s)\n",
		m.ArrivesAt.Format("2006-01-02 15:04 MST"), RelTime(m.ArrivesAt))
	if len(pathNames) > 0 {
		_, _ = fmt.Fprintf(sess.Out, "  path:      %s\n", strings.Join(pathNames, " → "))
	} else if len(m.Path) > 0 {
		_, _ = fmt.Fprintf(sess.Out, "  path:      %s\n", strings.Join(m.Path, " → "))
	}
}
