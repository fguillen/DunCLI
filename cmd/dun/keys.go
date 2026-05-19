package main

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage player ApiKeys",
	}
	cmd.AddCommand(newKeysListCmd(), newKeysRevokeCmd())
	return cmd
}

func newKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the player ApiKeys issued for your account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runKeysList(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func runKeysList(ctx context.Context, out io.Writer) error {
	sess, err := loadSession()
	if err != nil {
		return err
	}
	keys, err := sess.client.ListPlayerAPIKeys(ctx)
	if err != nil {
		return fmt.Errorf("list api keys: %w", err)
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tLAST USED\tEXPIRES\tSTATUS")
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			k.ID,
			nameOrDash(k.Name),
			timeOrDash(k.LastUsedAt),
			k.ExpiresAt.Format("2006-01-02"),
			keyStatus(k),
		)
	}
	return w.Flush()
}

func newKeysRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a player ApiKey by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeysRevoke(cmd.Context(), cmd.OutOrStdout(), args[0])
		},
	}
}

func runKeysRevoke(ctx context.Context, out io.Writer, id string) error {
	sess, err := loadSession()
	if err != nil {
		return err
	}

	// List first so we know whether the user is revoking *their own*
	// current key — if so we also clear the local credential after the
	// server-side revoke succeeds.
	keys, err := sess.client.ListPlayerAPIKeys(ctx)
	if err != nil {
		return fmt.Errorf("list api keys: %w", err)
	}
	var revokingCurrent bool
	for _, k := range keys {
		if k.ID == id && k.Current {
			revokingCurrent = true
			break
		}
	}

	if err := sess.client.RevokePlayerAPIKey(ctx, id); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}

	if revokingCurrent {
		cur, ok := sess.store.CurrentCredential()
		if ok {
			sess.store.Delete(cur.BaseURL, cur.Email)
			if err := sess.store.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
		}
	}

	_, _ = fmt.Fprintf(out, "Revoked key %s.\n", id)
	return nil
}

func nameOrDash(n gen.NilString) string {
	v, ok := n.Get()
	if !ok || v == "" {
		return "-"
	}
	return v
}

func timeOrDash(t gen.NilDateTime) string {
	v, ok := t.Get()
	if !ok {
		return "-"
	}
	return v.Format(time.RFC3339)
}

func keyStatus(k gen.ApiKeyEntry) string {
	if _, ok := k.RevokedAt.Get(); ok {
		return "revoked"
	}
	if k.Current {
		return "current"
	}
	return "active"
}
