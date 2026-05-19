// Package verbs hosts the Phase 4+ game verbs registered into the
// REPL's verb tree. Each file in this package covers one logical
// area (servers, profile, players, …) and uses init() to call
// shell.Register so cmd/dun only needs a blank import to wire them
// up.
package verbs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "servers",
		Summary: "List servers you belong to and ones you can join",
		Usage:   "servers",
		Run:     runServersList,
	})

	shell.Register(&shell.Verb{
		Name:    "server",
		Summary: "Manage server membership",
		Usage:   "server <join|...>",
		Sub: map[string]*shell.Verb{
			"join": {
				Name:     "join",
				Summary:  "Join a server by slug",
				Usage:    "server join <slug>",
				Run:      runServerJoin,
				Complete: shell.SuggestFunc(suggestEligibleServerSlugs),
			},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "join",
		Summary: "Sugar for `server join <slug>` (and `join world <slug>` later)",
		Usage:   "join <server-slug> | join world <world-slug>",
		Run:     runJoinSugar,
		Complete: shell.SuggestFunc(func(ctx context.Context, sess *shell.Session, prefix string) ([]string, error) {
			// `join world` is a future-Phase form; for now we only
			// complete server slugs (the typed positional after
			// `join`).
			return suggestEligibleServerSlugs(ctx, sess, prefix)
		}),
	})

	// Register the post-login server picker hook so the shell can
	// invoke it without importing the verbs package.
	shell.SetPostLoginPicker(postLoginServerPicker)
}

// runServersList implements `servers`.
func runServersList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	list, err := sess.API.ListPlayerServers(ctx)
	if err != nil {
		return err
	}
	var members, eligible []string
	for _, s := range list {
		row := fmt.Sprintf("%-20s %s", s.Slug, s.Name)
		if s.Member {
			members = append(members, row)
		} else {
			eligible = append(eligible, row)
		}
	}
	shell.Section(sess.Out, "Member of:", strings.Join(members, "\n"))
	_, _ = fmt.Fprintln(sess.Out)
	shell.Section(sess.Out, "Eligible to join:", strings.Join(eligible, "\n"))
	return nil
}

// runServerJoin implements `server join <slug>`.
func runServerJoin(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: server join <slug>")
	}
	slug := args[0]
	id, err := sess.API.ResolveServer(ctx, slug)
	if err != nil {
		return err
	}
	res, err := sess.API.JoinServer(ctx, id)
	if err != nil {
		return err
	}

	sess.Context.SetServer(res.Server.Slug)
	if err := sess.State.Save(sess.Context.Snapshot()); err != nil {
		// Persistence failure is non-fatal — the in-memory state is
		// correct; just surface the error so the user knows the next
		// session may not reload this scope.
		shell.Err(sess.Out, fmt.Errorf("persist state: %w", err))
	}
	shell.Success(sess.Out, fmt.Sprintf("joined %q (slug=%s)", res.Server.Name, res.Server.Slug))
	return nil
}

// runJoinSugar implements the `join` built-in. `join world <slug>` is
// reserved for the Phase 5 world-scoping verb; until then we
// explicitly reject it so the user gets a clear "not yet" message.
func runJoinSugar(ctx context.Context, sess *shell.Session, args []string, flags map[string]string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: join <server-slug>")
	}
	if args[0] == "world" {
		return fmt.Errorf("`join world` is not yet wired up (Phase 5)")
	}
	return runServerJoin(ctx, sess, args, flags)
}

// suggestEligibleServerSlugs is the dynamic Suggester for slug
// arguments — it lists every server the player can still join. The
// `member==true` case is intentionally excluded; users who type a
// server they're already a member of get an error from JoinServer
// (rendered by the dispatcher) rather than a silent no-op.
func suggestEligibleServerSlugs(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	list, err := sess.API.ListPlayerServers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, s := range list {
		if !s.Member {
			out = append(out, s.Slug)
		}
	}
	return out, nil
}

// postLoginServerPicker is the hook shell.Run calls when
// Options.PostLoginPicker is true. It opens the bubbles/list picker
// over the member servers and sets the chosen one as the current
// scope.
func postLoginServerPicker(ctx context.Context, sess *shell.Session) error {
	list, err := sess.API.ListPlayerServers(ctx)
	if err != nil {
		return err
	}
	items := make([]selector.Item, 0)
	for _, s := range list {
		if !s.Member {
			continue
		}
		items = append(items, selector.Item{
			Title:       s.Slug,
			Description: s.Name,
			Value:       s.Slug,
		})
	}
	if len(items) <= 1 {
		// Single membership or none — nothing to pick. The cmd/dun
		// caller already auto-applied the single member via
		// Options.SingleMemberSlug.
		return nil
	}
	chosen, err := selector.Pick(ctx, "Pick a server", items)
	if err != nil {
		// Cancellation is fine — the user can `server join …`
		// later. Return nil so shell.Run drops cleanly into the
		// prompt.
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}
	sess.Context.SetServer(chosen.Value)
	if err := sess.State.Save(sess.Context.Snapshot()); err != nil {
		shell.Err(sess.Out, fmt.Errorf("persist state: %w", err))
	}
	shell.Success(sess.Out, fmt.Sprintf("scope set to server %q", chosen.Value))
	return nil
}
