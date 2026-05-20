package verbs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "worlds",
		Summary: "List worlds on the in-scope server",
		Usage:   "worlds",
		Run:     runWorldsList,
	})

	shell.Register(&shell.Verb{
		Name:    "world",
		Summary: "Manage worlds on the in-scope server",
		Usage:   "world <show|join> <slug>",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:     "show",
				Summary:  "Show world detail (T0/grace, region/kingdom counts, your kingdom)",
				Usage:    "world show <slug>",
				Run:      runWorldShow,
				Complete: shell.SuggestFunc(suggestAnyWorldSlug),
			},
			"join": {
				Name:     "join",
				Summary:  "Join a world by slug — switches scope on success",
				Usage:    "world join <slug>",
				Run:      runWorldJoin,
				Complete: shell.SuggestFunc(suggestAnyWorldSlug),
			},
		},
	})
}

// runWorldsList implements `worlds`. Today's spec WorldSummary does
// not include a `my_kingdom` field, so we cannot split member-vs-
// eligible the way `servers` does — flagged as a backend co-evolution
// candidate. We render a flat list with status instead.
func runWorldsList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}
	list, err := sess.API.ListServerWorlds(ctx, serverID)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		shell.Section(sess.Out, "Worlds:", "")
		return nil
	}
	var lines []string
	for _, w := range list {
		lines = append(lines, fmt.Sprintf("%-20s %-9s %s", w.Slug, string(w.Status), w.Name))
	}
	shell.Section(sess.Out, "Worlds:", strings.Join(lines, "\n"))
	return nil
}

// runWorldShow implements `world show <slug>`.
func runWorldShow(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: world show <slug>")
	}
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}
	worldID, err := sess.API.ResolveWorld(ctx, serverID, args[0])
	if err != nil {
		return err
	}
	world, err := sess.API.ShowWorld(ctx, worldID)
	if err != nil {
		return err
	}
	printWorld(sess, world)
	return nil
}

// runWorldJoin implements `world join <slug>`.
func runWorldJoin(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: world join <slug>")
	}
	slug := args[0]
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}
	worldID, err := sess.API.ResolveWorld(ctx, serverID, slug)
	if err != nil {
		return err
	}
	kingdom, err := sess.API.JoinWorld(ctx, worldID, serverID)
	if err != nil {
		return err
	}

	sess.Context.SetWorld(slug)
	if perr := sess.State.Save(sess.Context.Snapshot()); perr != nil {
		shell.Err(sess.Out, fmt.Errorf("persist state: %w", perr))
	}
	msg := fmt.Sprintf("joined world (slug=%s)", slug)
	if h := sess.Context.KingdomHandle(); h != "" {
		msg = fmt.Sprintf("joined world (slug=%s) as kingdom %q", slug, h)
	}
	shell.Success(sess.Out, msg)
	if hr, ok := kingdom.HomeRegionID.Get(); ok {
		shell.Info(sess.Out, "home region assigned: "+hr)
	} else {
		shell.Info(sess.Out, "no home region yet — assigned when the world starts")
	}
	return nil
}

// printWorld renders a *gen.World to scrollback. The format is
// intentionally line-per-field so users can copy specific values.
func printWorld(sess *shell.Session, w *gen.World) {
	shell.Strong(sess.Out, fmt.Sprintf("%s  (slug=%s)", w.Name, w.Slug))
	_, _ = fmt.Fprintln(sess.Out, "  status:    "+string(w.Status))
	_, _ = fmt.Fprintln(sess.Out, "  T0:        "+w.T0At.Format("2006-01-02 15:04 MST"))
	if g, ok := w.GraceClosesAt.Get(); ok {
		_, _ = fmt.Fprintln(sess.Out, "  grace end: "+g.Format("2006-01-02 15:04 MST"))
	}
	_, _ = fmt.Fprintf(sess.Out, "  regions:   %d\n", w.RegionCount)
	_, _ = fmt.Fprintf(sess.Out, "  kingdoms:  %d (min %d)\n", w.KingdomCount, w.MinPlayers)
	if mk, ok := w.MyKingdom.Get(); ok {
		line := "  your kingdom: id=" + mk.ID
		if hr, ok := mk.HomeRegionID.Get(); ok {
			line += "  home_region=" + hr
		}
		_, _ = fmt.Fprintln(sess.Out, line)
	}
}

// suggestAnyWorldSlug completes against every world on the in-scope
// server. Used by both `world show` and `world join` — the join verb
// would ideally complete only joinable slugs, but WorldSummary doesn't
// carry membership info, so we keep one completer.
func suggestAnyWorldSlug(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return nil, nil
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return nil, err
	}
	worlds, err := sess.API.ListServerWorlds(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(worlds))
	for _, w := range worlds {
		out = append(out, w.Slug)
	}
	return out, nil
}
