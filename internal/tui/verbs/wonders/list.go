package wonders

import (
	"context"

	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// runWondersList implements the plural top-level `wonders` verb —
// world-scoped, public view of every wonder visible to members of the
// in-scope server. Backend ordering is creation-time.
func runWondersList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	items, err := sess.API.ListWorldWonders(ctx, worldID)
	if err != nil {
		return err
	}
	printWonderList(sess, items)
	return nil
}
