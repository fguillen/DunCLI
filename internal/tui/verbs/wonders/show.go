package wonders

import (
	"context"

	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// runWonderShow implements `wonder` and `wonder show`. Renders the
// wonder detail block if one exists; otherwise drops a friendly hint
// pointing the user at `wonder start`.
func runWonderShow(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	w, err := sess.API.GetWonder(ctx, kingdomID)
	if err != nil {
		return err
	}
	if w == nil {
		shell.Info(sess.Out, "no wonder under construction — try `wonder start <name>`")
		return nil
	}
	printWonderDetail(sess, w)
	return nil
}
