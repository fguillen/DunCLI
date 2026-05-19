package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fguillen/dun-cli/internal/auth"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate via magic-link email and persist a player ApiKey",
		Long: "Prompts for your email, requests a magic-link, then prompts for the\n" +
			"token from the email and exchanges it for an ApiKey. The key is saved\n" +
			"to ~/.dun/credentials (mode 0600) and used by subsequent commands.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := runLogin(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
				return err
			}
			// Post-login handoff into the shell. The picker hook
			// only opens when the player has > 1 server membership;
			// the SingleMemberSlug branch handles the common one-
			// server case so the user lands already scoped.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			opts, err := postLoginShellOptions(ctx)
			if err != nil {
				// Membership probe failed but login succeeded —
				// fall back to the bare shell rather than refusing
				// to enter it.
				_, _ = fmt.Fprintln(cmd.OutOrStdout(),
					"(could not pre-fetch server memberships, continuing without picker)")
				opts = shell.Options{Version: Version}
			}
			return enterShell(ctx, opts)
		},
	}
}

// postLoginShellOptions inspects the player's server memberships and
// returns shell.Options pre-set with either a single-member scope or
// the PostLoginPicker flag.
func postLoginShellOptions(ctx context.Context) (shell.Options, error) {
	sess, err := loadSession()
	if err != nil {
		return shell.Options{}, err
	}
	list, err := sess.client.ListPlayerServers(ctx)
	if err != nil {
		return shell.Options{}, err
	}
	opts := shell.Options{Version: Version}
	var members []string
	for _, s := range list {
		if s.Member {
			members = append(members, s.Slug)
		}
	}
	switch len(members) {
	case 0:
		// No memberships yet; user will run `servers` then
		// `server join <slug>`. No flags set.
	case 1:
		opts.SingleMemberSlug = members[0]
	default:
		opts.PostLoginPicker = true
	}
	return opts, nil
}

func runLogin(ctx context.Context, in io.Reader, out io.Writer) error {
	sess, err := loadSession()
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

	if err := sess.client.RequestPlayerMagicLink(ctx, email); err != nil {
		return fmt.Errorf("request magic link: %w", err)
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

	resp, err := sess.client.ExchangePlayerMagicLink(ctx, token)
	if err != nil {
		return fmt.Errorf("exchange magic link: %w", err)
	}

	cred := auth.Credential{
		BaseURL:   sess.cfg.BaseURL,
		Email:     resp.Owner.Email,
		APIKey:    resp.APIKey,
		ExpiresAt: resp.ExpiresAt,
	}
	sess.store.Upsert(cred)
	if err := sess.store.SetCurrent(cred.BaseURL, cred.Email); err != nil {
		return fmt.Errorf("set current credential: %w", err)
	}
	if err := sess.store.Save(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Logged in as %s (expires %s).\n",
		cred.Email,
		cred.ExpiresAt.Format("2006-01-02"),
	)
	return nil
}

// readLine reads one line from r, trimming the trailing newline + CR.
// Returns "" with io.EOF only if nothing was read.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil && err != io.EOF {
		return "", err
	}
	if line == "" && err == io.EOF {
		return "", io.EOF
	}
	return strings.TrimSpace(line), nil
}
