package wonders

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// runWonderStart implements `wonder start [<name>]`. With no arg, opens
// a picker over the §14 fixed menu. With one arg, validates against the
// 6-slug enum client-side; the backend still enforces prereqs / cost.
// The confirm message names the foundation payment without spelling out
// numbers (no `previewWonderStart` endpoint exists today — flagged in
// the tutorial as a backend co-evolution candidate).
func runWonderStart(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	slug := ""
	switch len(args) {
	case 0:
		picked, perr := pickWonderName(ctx)
		if perr != nil {
			if errors.Is(perr, selector.ErrCancelled) {
				return nil
			}
			return perr
		}
		slug = picked
	case 1:
		slug = args[0]
	default:
		return errors.New("usage: wonder start [<name>]")
	}

	if !validWonderSlug(slug) {
		return fmt.Errorf("unknown wonder name %q (try one of: %s)",
			slug, strings.Join(wonderSlugs, ", "))
	}

	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Begin construction of %s?", titleName(slug)),
		"Deducts the 25% foundation payment and locks your build queue until the wonder completes or is destroyed.")
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}
	if !ok {
		shell.Info(sess.Out, "aborted")
		return nil
	}

	w, err := sess.API.StartWonder(ctx, kingdomID, slug)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("started: %s — HP %d/%d (status=%s). Pay milestones at 25/50/75%% as construction crosses each threshold.",
		titleName(string(w.Name)), w.Hp, w.TargetHp, string(w.Status)))
	shell.Info(sess.Out, "track progress with `wonder show`; the world sees it via `wonders`")
	return nil
}

// pickWonderName opens a list picker over the 6 wonder slugs. Item
// titles are the title-case display name; Value is the wire slug.
func pickWonderName(ctx context.Context) (string, error) {
	items := make([]selector.Item, 0, len(wonderSlugs))
	for _, s := range wonderSlugs {
		items = append(items, selector.Item{
			Title:       titleName(s),
			Description: s,
			Value:       s,
		})
	}
	picked, err := selector.Pick(ctx, "Pick a wonder", items)
	if err != nil {
		return "", err
	}
	return picked.Value, nil
}
