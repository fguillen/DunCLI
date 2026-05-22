package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// newAdminCmd builds the `dun admin` command tree. Bare `dun admin`
// drops into the `dun-admin>` REPL; `login` / `logout` are the only
// cobra subcommands (admin key management lives inside the shell as
// the `keys` verb). The admin surface is a separate mode of the same
// binary — it never shares a process with the player `dun>` shell.
func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Enter the admin shell (server / world administration)",
		Long: "Bare `dun admin` drops you into the interactive admin shell after\n" +
			"auth. `dun admin login` / `dun admin logout` manage the admin\n" +
			"credential, which is stored alongside the player credential in\n" +
			"~/.dun/credentials under a separate admin scope.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return enterAdminShell(ctx, shell.Options{Version: Version})
		},
	}
	cmd.AddCommand(newAdminLoginCmd(), newAdminLogoutCmd())
	return cmd
}

// enterAdminShell loads the admin session and hands off to shell.Run
// in admin mode. It is the shared entrypoint for the bare `dun admin`
// invocation and the post-`dun admin login` handoff. Missing admin
// credentials prints a clear hint and exits non-zero. Mode is forced
// to ModeAdmin so callers cannot wire the wrong surface.
func enterAdminShell(ctx context.Context, opts shell.Options) error {
	opts.Mode = shell.ModeAdmin
	sess, err := loadAdminSession()
	if err != nil {
		return err
	}
	cred, ok := sess.store.CurrentAdminCredential()
	if !ok {
		return errors.New("not logged in — run `dun admin login`")
	}
	return shell.Run(ctx, sess.cfg.BaseURL, cred.Email, sess.client, opts)
}
