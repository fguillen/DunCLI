package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/fguillen/dun-cli/internal/auth"
)

func newAdminLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the current admin ApiKey and clear the local admin credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAdminLogout(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func runAdminLogout(ctx context.Context, out io.Writer) error {
	sess, err := loadAdminSession()
	if err != nil {
		return err
	}

	cur, ok := sess.store.CurrentAdminCredential()
	if !ok {
		return errors.New("no active admin session — nothing to log out of")
	}

	// Discover the id of our current key. The exchange response does not
	// include the id, so we have to list and find current: true.
	keys, err := sess.client.ListAdminAPIKeys(ctx)
	if err != nil {
		return fmt.Errorf("list admin api keys: %w", err)
	}
	var currentID string
	for _, k := range keys {
		if k.Current {
			currentID = k.ID
			break
		}
	}
	if currentID == "" {
		return errors.New("server did not return a current key — your local admin credential may be stale")
	}

	if err := sess.client.RevokeAdminAPIKey(ctx, currentID); err != nil {
		return fmt.Errorf("revoke admin api key: %w", err)
	}

	// Only delete the local entry after the server-side revoke succeeds.
	sess.store.Delete(cur.BaseURL, cur.Email, auth.ScopeAdmin)
	if err := sess.store.Save(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Logged out admin %s.\n", cur.Email)
	return nil
}
