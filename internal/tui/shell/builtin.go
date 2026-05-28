package shell

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fguillen/dun-cli/internal/tui/theme"
)

// defaultLoopSeconds is the interval `loop` uses when called without an
// explicit argument.
const defaultLoopSeconds = 5

// registerBuiltins is called by Run() before the prompt loop, against
// whichever registry the active mode uses. The built-in verbs are
// simple enough that they live here rather than in internal/tui/verbs/*
// — they have no API dependencies and are part of the shell's contract
// regardless of which game verbs are linked, in either shell surface.
func registerBuiltins(reg *registry, version string) {
	reg.add(&Verb{
		Name:    "help",
		Summary: "List commands, or print usage for one (`help <verb>`)",
		Usage:   "help [verb]",
		Run:     runHelp,
		Complete: SuggestFunc(func(_ context.Context, sess *Session, _ string) ([]string, error) {
			return verbNamesFor(sess.registry()), nil
		}),
	})

	exitFn := func(_ context.Context, _ *Session, _ []string, _ map[string]string) error {
		return ErrExit
	}
	reg.add(&Verb{Name: "quit", Summary: "Exit the shell", Run: exitFn})
	reg.add(&Verb{Name: "exit", Summary: "Exit the shell", Run: exitFn})

	reg.add(&Verb{
		Name:    "clear",
		Summary: "Clear the screen",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			// ANSI clear + home; works on every terminal readline supports.
			_, _ = fmt.Fprint(sess.Out, "\033[H\033[2J")
			return nil
		},
	})

	reg.add(&Verb{
		Name:    "version",
		Summary: "Print the dun CLI version",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			_, _ = fmt.Fprintln(sess.Out, version)
			return nil
		},
	})

	reg.add(&Verb{
		Name:    "whoami",
		Summary: "Show the active credential",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			st := theme.Active()
			_, _ = fmt.Fprintln(sess.Out, st.Strong.Render(sess.Creds.Email))
			_, _ = fmt.Fprintln(sess.Out, st.Subtle.Render("  base_url: "+sess.Cfg.BaseURL))
			return nil
		},
	})

	reg.add(&Verb{
		Name:    "loop",
		Summary: "Re-run the last command every N seconds (Ctrl-C to stop)",
		Usage:   "loop [seconds]   (default 5)",
		Run:     runLoop,
	})

	reg.add(&Verb{
		Name:    "where",
		Summary: "Show the active scope",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			c := sess.Context
			st := theme.Active()
			line := func(label, val string) {
				if val == "" {
					val = st.Subtle.Render("(none)")
				} else {
					val = st.Strong.Render(val)
				}
				_, _ = fmt.Fprintf(sess.Out, "  %-8s %s\n", label+":", val)
			}
			line("server", c.ServerSlug())
			line("world", c.WorldSlug())
			// Admins manage servers and worlds but never hold a kingdom,
			// so the kingdom line is player-only.
			if sess.Mode != ModeAdmin {
				line("kingdom", c.KingdomHandle())
			}
			return nil
		},
	})
}

// runHelp implements the `help` verb. It draws verbs from the session's
// own registry so the admin shell lists admin verbs.
func runHelp(_ context.Context, sess *Session, args []string, _ map[string]string) error {
	reg := sess.registry()
	if len(args) == 0 {
		verbs := reg.list()
		_, _ = fmt.Fprintln(sess.Out, theme.Active().Strong.Render("commands:"))
		_, _ = fmt.Fprint(sess.Out, helpFormat(verbs))
		_, _ = fmt.Fprintln(sess.Out)
		_, _ = fmt.Fprintln(sess.Out, theme.Active().Hint.Render(
			"Tab completes verbs, slugs, and flags. Ctrl-D exits."))
		return nil
	}
	v, ok := reg.resolve(args[0])
	if !ok {
		return fmt.Errorf("no such verb: %s", args[0])
	}
	_, _ = fmt.Fprintln(sess.Out, theme.Active().Strong.Render(v.Name))
	if v.Summary != "" {
		_, _ = fmt.Fprintln(sess.Out, "  "+v.Summary)
	}
	if v.Usage != "" {
		_, _ = fmt.Fprintln(sess.Out, "  usage: "+v.Usage)
	}
	if len(v.Sub) > 0 {
		_, _ = fmt.Fprintln(sess.Out)
		_, _ = fmt.Fprintln(sess.Out, "  subcommands:")
		names := subVerbNames(v)
		for _, n := range names {
			_, _ = fmt.Fprintf(sess.Out, "    %-12s %s\n", n, v.Sub[n].Summary)
		}
	}
	if len(v.Flags) > 0 {
		_, _ = fmt.Fprintln(sess.Out)
		_, _ = fmt.Fprintln(sess.Out, "  flags:")
		for _, f := range v.Flags {
			suffix := ""
			if f.HasValue {
				suffix = " <value>"
			}
			_, _ = fmt.Fprintf(sess.Out, "    --%s%s  %s\n", f.Name, suffix, f.Help)
		}
	}
	return nil
}

// runLoop implements the `loop` verb: it re-runs the last typed command
// (sess.LastCommand) every N seconds until the user presses Ctrl-C. The
// ctx is the per-command SIGINT-scoped context the shell.Run loop wires
// up, so it cancels on Ctrl-C without poisoning later commands.
func runLoop(ctx context.Context, sess *Session, args []string, _ map[string]string) error {
	seconds := defaultLoopSeconds
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return errors.New("loop: seconds must be a positive whole number")
		}
		seconds = n
	}

	last := strings.TrimSpace(sess.LastCommand)
	if last == "" {
		return errors.New("loop: no previous command to repeat")
	}

	interval := time.Duration(seconds) * time.Second
	st := theme.Active()
	Info(sess.Out, fmt.Sprintf("Looping `%s` every %ds — press Ctrl-C to stop.", last, seconds))

	first := true
	for ctx.Err() == nil {
		if !first {
			_, _ = fmt.Fprintln(sess.Out, st.Hint.Render(strings.Repeat("─", 16)))
		}
		first = false
		if errors.Is(Dispatch(ctx, sess, last), ErrExit) {
			// The repeated command asked the shell to quit; honor it.
			return ErrExit
		}
		select {
		case <-ctx.Done():
			// Ctrl-C during the wait — stop quietly.
		case <-time.After(interval):
			continue
		}
		break
	}
	Info(sess.Out, "loop stopped")
	return nil
}
