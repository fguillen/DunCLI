package wonders

import (
	"context"
	"errors"
	"fmt"

	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// runWonderCancel implements `wonder cancel`. Double-confirms via a
// typed-name match (the user must type the snake-case slug, e.g.
// `sky_tower`) before the destructive call. The "all paid resources
// lost" warning is in the form subtitle.
func runWonderCancel(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	cur, err := sess.API.GetWonder(ctx, kingdomID)
	if err != nil {
		return err
	}
	if cur == nil {
		return errors.New("no wonder to cancel")
	}
	slug := string(cur.Name)

	res, err := selector.Form(ctx, "Cancel wonder", []selector.Field{
		{
			Key:     "confirm",
			Label:   fmt.Sprintf("Type `%s` to confirm cancellation (paid resources will be lost)", slug),
			Initial: "",
			Validate: func(s string) error {
				if s != slug {
					return fmt.Errorf("must match %q exactly", slug)
				}
				return nil
			},
		},
	})
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			shell.Info(sess.Out, "aborted")
			return nil
		}
		return err
	}
	if res["confirm"] != slug {
		shell.Info(sess.Out, "aborted")
		return nil
	}

	w, err := sess.API.CancelWonder(ctx, kingdomID)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("cancelled: %s — paid resources lost, build queue unlocked.",
		titleName(string(w.Name))))
	return nil
}
