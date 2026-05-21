// Package archive hosts the Phase 13 read-only verbs: `archive`
// (the frozen end-of-round snapshot for an archived world) and
// `hall-of-fame` (the per-server leaderboard snapshots). Neither verb
// has a mutation path; both compose with the Phase 4 server / Phase 5
// world scope.
package archive

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.Register(&shell.Verb{
		Name:     "archive",
		Summary:  "Show the frozen end-of-round snapshot for an archived world",
		Usage:    "archive [<world-slug>]",
		Run:      runArchive,
		Complete: shell.SuggestFunc(suggestAnyWorldSlug),
	})
}

// runArchive implements `archive [<world-slug>]`. No arg uses the
// in-scope world; an arg looks up any archived world on the in-scope
// server. The backend's 404-while-live envelope surfaces unchanged
// as `error: not found (...)` — we translate the spec's "no archive
// row yet" semantics into a friendlier hint so users aren't left
// guessing whether the world is live or the slug is wrong.
func runArchive(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) > 1 {
		return errors.New("usage: archive [<world-slug>]")
	}

	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}

	var slug string
	if len(args) == 1 {
		slug = strings.TrimSpace(args[0])
		if slug == "" {
			return errors.New("world slug is required")
		}
	} else {
		slug = sess.Context.WorldSlug()
		if slug == "" {
			return errors.New("not in a world scope — try `world join <slug>` first, or pass `archive <world-slug>`")
		}
	}

	worldID, err := sess.API.ResolveWorld(ctx, serverID, slug)
	if err != nil {
		return err
	}

	a, err := sess.API.ShowWorldArchive(ctx, worldID)
	if err != nil {
		return err
	}
	printArchive(sess, slug, a)
	return nil
}

// suggestAnyWorldSlug is a sibling-package mirror of the helper in
// verbs/worlds.go — `package verbs` keeps it private so we re-derive
// it here against the same `ListServerWorlds` cache. Suggester is
// best-effort: any error returns nil candidates rather than spamming
// the prompt.
func suggestAnyWorldSlug(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return nil, nil
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return nil, nil
	}
	worlds, err := sess.API.ListServerWorlds(ctx, serverID)
	if err != nil {
		return nil, nil
	}
	out := make([]string, 0, len(worlds))
	for _, w := range worlds {
		out = append(out, w.Slug)
	}
	return out, nil
}

// printArchive renders one *gen.RoundArchive to scrollback. Layout
// mirrors `world show`: a strong header, then line-per-field summary,
// followed by aggregate counts and final-state highlights. Kingdoms
// list is capped at the top 5 by `final_node_count` to keep output
// scannable; the full roster surfaces via the per-kingdom `peak_nodes`
// stats once a future verb exposes them.
func printArchive(sess *shell.Session, slug string, a *gen.RoundArchive) {
	shell.Strong(sess.Out, fmt.Sprintf("World archive  (slug=%s)", slug))
	_, _ = fmt.Fprintf(sess.Out, "  ended:     %s\n", a.EndedAt.UTC().Format("2006-01-02 15:04 UTC"))

	if v, ok := a.WinnerKingdomID.Get(); ok && v != "" {
		_, _ = fmt.Fprintf(sess.Out, "  winner:    %s\n", v)
	} else {
		_, _ = fmt.Fprintln(sess.Out, "  winner:    (none)")
	}
	if v, ok := a.WonderName.Get(); ok && v != "" {
		_, _ = fmt.Fprintf(sess.Out, "  wonder:    %s\n", v)
	} else {
		_, _ = fmt.Fprintln(sess.Out, "  wonder:    (none)")
	}

	fs := a.FrozenState
	_, _ = fmt.Fprintf(sess.Out, "  regions:   %d\n", len(fs.Regions))
	_, _ = fmt.Fprintf(sess.Out, "  kingdoms:  %d\n", len(fs.Kingdoms))
	_, _ = fmt.Fprintf(sess.Out, "  nodes:     %d\n", fs.NodesCount)
	_, _ = fmt.Fprintf(sess.Out, "  battles:   %d\n", fs.BattlesCount)
	_, _ = fmt.Fprintf(sess.Out, "  caravans:  %d\n", fs.CaravansCount)

	if w, ok := fs.Wonder.Get(); ok {
		shell.Section(sess.Out, "Wonder:", fmt.Sprintf(
			"%s  status=%s  hp=%d/%d  damage_events=%d",
			w.Name, string(w.Status), w.Hp, w.TargetHp, w.DamageEventsCount.Or(0)))
	}

	shell.Section(sess.Out, "Top kingdoms (by final nodes):", renderTopKingdoms(fs.Kingdoms, 5))
}

// renderTopKingdoms sorts kingdoms by FinalNodeCount desc and returns
// up to `top` lines. Eliminated kingdoms are kept in the ranking but
// flagged. Empty kingdom list returns an empty string so Section can
// render the standard "(none)" placeholder.
func renderTopKingdoms(items []gen.FrozenStateKingdomsItem, top int) string {
	if len(items) == 0 {
		return ""
	}
	sorted := make([]gen.FrozenStateKingdomsItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].FinalNodeCount != sorted[j].FinalNodeCount {
			return sorted[i].FinalNodeCount > sorted[j].FinalNodeCount
		}
		return sorted[i].PeakNodes > sorted[j].PeakNodes
	})
	if top > 0 && len(sorted) > top {
		sorted = sorted[:top]
	}
	lines := make([]string, 0, len(sorted))
	for i, k := range sorted {
		handle := "(unknown)"
		if v, ok := k.Handle.Get(); ok && v != "" {
			handle = v
		}
		suffix := ""
		if v, ok := k.EliminatedAt.Get(); ok && !v.IsZero() {
			suffix = "  eliminated " + v.UTC().Format("2006-01-02 15:04 UTC")
		}
		lines = append(lines, fmt.Sprintf("%d. %-14s  final_nodes=%d  peak=%d%s",
			i+1, handle, k.FinalNodeCount, k.PeakNodes, suffix))
	}
	return strings.Join(lines, "\n")
}
