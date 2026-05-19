package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage your player account",
	}
	cmd.AddCommand(newAccountDeleteCmd())
	return cmd
}

func newAccountDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Irreversibly delete your player account",
		Long: "Deletes the calling player account. Real name is purged immediately,\n" +
			"per-server handle is anonymized, stats are zeroed, and every ApiKey is\n" +
			"revoked. Irreversible. The local credentials file is cleared on success.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAccountDelete(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

func runAccountDelete(ctx context.Context, in io.Reader, out io.Writer) error {
	sess, err := loadSession()
	if err != nil {
		return err
	}

	cur, ok := sess.store.CurrentCredential()
	if !ok {
		return errors.New("no active session — log in first")
	}

	_, _ = fmt.Fprintf(out, "Account: %s  (key …%s)\n", cur.Email, lastFour(cur.APIKey))
	_, _ = fmt.Fprintln(out, "This will permanently delete your account on "+cur.BaseURL+".")
	_, _ = fmt.Fprintf(out, "delete account %s? [y/N]: ", cur.Email)

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return errors.New("aborted")
	}

	if err := sess.client.DeleteAccount(ctx); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	sess.store.Clear()
	if err := sess.store.Save(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	_, _ = fmt.Fprintln(out, "Account deleted.")
	return nil
}

func lastFour(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}
