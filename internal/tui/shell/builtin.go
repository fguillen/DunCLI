package shell

import (
	"context"
	"fmt"

	"github.com/fguillen/dun-cli/internal/tui/theme"
)

// registerBuiltins is called by Run() before the prompt loop. The
// built-in verbs are simple enough that they live here rather than in
// internal/tui/verbs/* — they have no API dependencies and are part
// of the shell's contract regardless of which game verbs are linked.
func registerBuiltins(version string) {
	Register(&Verb{
		Name:    "help",
		Summary: "List commands, or print usage for one (`help <verb>`)",
		Usage:   "help [verb]",
		Run:     runHelp,
		Complete: SuggestFunc(func(_ context.Context, _ *Session, _ string) ([]string, error) {
			return verbNames(), nil
		}),
	})

	exitFn := func(_ context.Context, _ *Session, _ []string, _ map[string]string) error {
		return ErrExit
	}
	Register(&Verb{Name: "quit", Summary: "Exit the shell", Run: exitFn})
	Register(&Verb{Name: "exit", Summary: "Exit the shell", Run: exitFn})

	Register(&Verb{
		Name:    "clear",
		Summary: "Clear the screen",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			// ANSI clear + home; works on every terminal readline supports.
			_, _ = fmt.Fprint(sess.Out, "\033[H\033[2J")
			return nil
		},
	})

	Register(&Verb{
		Name:    "version",
		Summary: "Print the dun CLI version",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			_, _ = fmt.Fprintln(sess.Out, version)
			return nil
		},
	})

	Register(&Verb{
		Name:    "whoami",
		Summary: "Show the active credential",
		Run: func(_ context.Context, sess *Session, _ []string, _ map[string]string) error {
			st := theme.Active()
			_, _ = fmt.Fprintln(sess.Out, st.Strong.Render(sess.Creds.Email))
			_, _ = fmt.Fprintln(sess.Out, st.Subtle.Render("  base_url: "+sess.Cfg.BaseURL))
			return nil
		},
	})

	Register(&Verb{
		Name:    "where",
		Summary: "Show the active (server, world, kingdom) scope",
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
			line("kingdom", c.KingdomHandle())
			return nil
		},
	})
}

// runHelp implements the `help` verb.
func runHelp(_ context.Context, sess *Session, args []string, _ map[string]string) error {
	if len(args) == 0 {
		verbs := Verbs()
		_, _ = fmt.Fprintln(sess.Out, theme.Active().Strong.Render("commands:"))
		_, _ = fmt.Fprint(sess.Out, helpFormat(verbs))
		_, _ = fmt.Fprintln(sess.Out)
		_, _ = fmt.Fprintln(sess.Out, theme.Active().Hint.Render(
			"Tab completes verbs, slugs, and flags. Ctrl-D exits."))
		return nil
	}
	v, ok := Resolve(args[0])
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
