package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/auth"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func newAdminLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate via admin magic-link email and persist an admin ApiKey",
		Long: "Prompts for your email, requests an admin magic-link, then prompts\n" +
			"for the token from the email and exchanges it for an admin ApiKey.\n" +
			"The key is saved to ~/.dun/credentials (mode 0600) under the admin\n" +
			"scope, so it coexists with any player key for the same email.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := runAdminLogin(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
				return err
			}
			// Post-login handoff into the admin shell. There is no
			// membership picker — admin server verbs arrive in Phase 15.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return enterAdminShell(ctx, shell.Options{Version: Version})
		},
	}
}

func runAdminLogin(ctx context.Context, in io.Reader, out io.Writer) error {
	sess, err := loadAdminSession()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(in)

	_, _ = fmt.Fprintf(out, "Email: ")
	email, err := readLine(reader)
	if err != nil {
		return err
	}
	if email == "" {
		return errors.New("email is required")
	}

	if err := sess.client.RequestAdminMagicLink(ctx, email); err != nil {
		return fmt.Errorf("request admin magic link: %w", err)
	}
	_, _ = fmt.Fprintln(out, "Magic-link email sent. Check your inbox.")

	_, _ = fmt.Fprintf(out, "Token: ")
	token, err := readLine(reader)
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("token is required")
	}

	resp, err := sess.client.ExchangeAdminMagicLink(ctx, token)
	if err != nil {
		return fmt.Errorf("exchange admin magic link: %w", err)
	}
	// The backend echoes the owner type; refuse to persist a player key
	// under the admin scope if the token was the wrong kind.
	if resp.Owner.Type != gen.ExchangeResponseOwnerTypeAdmin {
		return fmt.Errorf("expected an admin credential but the backend issued a %q key", resp.Owner.Type)
	}

	cred := auth.Credential{
		BaseURL:   sess.cfg.BaseURL,
		Email:     resp.Owner.Email,
		APIKey:    resp.APIKey,
		ExpiresAt: resp.ExpiresAt,
		Scope:     auth.ScopeAdmin,
	}
	sess.store.Upsert(cred)
	if err := sess.store.SetCurrentAdmin(cred.BaseURL, cred.Email); err != nil {
		return fmt.Errorf("set current admin credential: %w", err)
	}
	if err := sess.store.Save(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Logged in as admin %s (expires %s).\n",
		cred.Email,
		cred.ExpiresAt.Format("2006-01-02"),
	)
	return nil
}
