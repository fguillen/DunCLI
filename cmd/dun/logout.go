package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the current player ApiKey and clear the local credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogout(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func runLogout(ctx context.Context, out io.Writer) error {
	sess, err := loadSession()
	if err != nil {
		return err
	}

	cur, ok := sess.store.CurrentCredential()
	if !ok {
		return errors.New("no active session — nothing to log out of")
	}

	// Discover the id of our current key. The exchange response does not
	// include the id, so we have to list and find current: true.
	keys, err := sess.client.ListPlayerAPIKeys(ctx)
	if err != nil {
		return fmt.Errorf("list api keys: %w", err)
	}
	var currentID string
	for _, k := range keys {
		if k.Current {
			currentID = k.ID
			break
		}
	}
	if currentID == "" {
		return errors.New("server did not return a current key — your local credential may be stale")
	}

	if err := sess.client.RevokePlayerAPIKey(ctx, currentID); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}

	// Only delete the local entry after the server-side revoke succeeds.
	sess.store.Delete(cur.BaseURL, cur.Email)
	if err := sess.store.Save(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Logged out %s.\n", cur.Email)
	return nil
}
