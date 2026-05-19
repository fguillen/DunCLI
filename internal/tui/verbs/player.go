package verbs

import (
	"context"
	"errors"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "player",
		Summary: "Read another player's profile",
		Usage:   "player show <handle>",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:    "show",
				Summary: "Show a player's profile on the in-scope server",
				Usage:   "player show <handle>",
				Run:     runPlayerShow,
				// No completer: there is no `listServerPlayers`
				// endpoint in the v1 spec, so completion would
				// either hit ResolvePlayer per keystroke (404 spam)
				// or return nothing. We leave it unset and flag the
				// missing endpoint as a backend co-evolution
				// candidate; see CLAUDE.md.
			},
		},
	})
}

func runPlayerShow(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: player show <handle>")
	}
	handle := args[0]

	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}
	prof, err := sess.API.ShowPlayerProfile(ctx, serverID, handle)
	if err != nil {
		return err
	}
	printProfileRead(sess, prof)
	return nil
}
