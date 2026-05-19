// Command dun is the terminal client (TUI) for the dun async multiplayer
// strategy backend. See PRODUCT.md for positioning and TODO.md for the
// implementation roadmap.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	dunlog "github.com/fguillen/dun-cli/internal/log"
	"github.com/fguillen/dun-cli/internal/tui"
)

// Version is the CLI version string. Overridden via -ldflags at release time.
const Version = "0.0.0-dev"

func main() {
	if err := newRootCmd(os.Stdout).Execute(); err != nil {
		// Cobra already printed the error; just signal failure.
		os.Exit(1)
	}
}

func newRootCmd(out io.Writer) *cobra.Command {
	var logLevel string

	root := &cobra.Command{
		Use:           "dun",
		Short:         "Terminal client for the dun async multiplayer strategy game",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.SetOut(out)
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info",
		"log level: debug, info, warn, error")

	// logger holds the per-invocation slog.Logger, opened in PersistentPreRunE.
	var (
		logger    *slog.Logger
		logCloser io.Closer
	)

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// `dun version` and `dun completion` are intentionally
		// side-effect-free; no logger needed.
		switch cmd.Name() {
		case "version", "completion":
			return nil
		}
		l, c, err := dunlog.Init(logLevel)
		if err != nil {
			return fmt.Errorf("init logger: %w", err)
		}
		logger = l
		logCloser = c
		slog.SetDefault(logger)
		return nil
	}
	root.PersistentPostRunE = func(_ *cobra.Command, _ []string) error {
		if logCloser != nil {
			return logCloser.Close()
		}
		return nil
	}

	root.AddCommand(newVersionCmd(out))
	root.AddCommand(newTUICmd())
	root.AddCommand(newLoginCmd())
	root.AddCommand(newLogoutCmd())
	root.AddCommand(newKeysCmd())
	root.AddCommand(newAccountCmd())
	return root
}

func newVersionCmd(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the dun CLI version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(out, Version)
			return err
		},
	}
}

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the Bubble Tea TUI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return tui.Run(ctx)
		},
	}
}
