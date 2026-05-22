package verbs

import (
	"fmt"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// printProfileRead renders a PlayerProfileRead (returned by
// showOwnProfile / showPlayerProfile) to the session's scrollback. Kept
// here so both the `profile show` and `player show` verbs share the
// same layout.
func printProfileRead(sess *shell.Session, p *gen.PlayerProfileRead) {
	if p == nil {
		return
	}
	shell.Strong(sess.Out, p.Handle)
	if rn := nilOrEmpty(p.RealName); rn != "" {
		_, _ = fmt.Fprintln(sess.Out, "  real name: "+rn)
	}
	if t := nilOrEmpty(p.Title); t != "" {
		_, _ = fmt.Fprintln(sess.Out, "  title:     "+t)
	}
	_, _ = fmt.Fprintln(sess.Out, "  joined:    "+p.JoinedAt.Format("2006-01-02"))
	printStats(sess, p.Stats)
}

// printProfileWrite renders a PlayerProfileWrite (returned by
// updateOwnProfile). It mirrors the read layout minus the JoinedAt
// field, which the write response omits.
func printProfileWrite(sess *shell.Session, p *gen.PlayerProfileWrite) {
	if p == nil {
		return
	}
	shell.Strong(sess.Out, nilOrEmpty(p.Handle))
	if rn := nilOrEmpty(p.RealName); rn != "" {
		_, _ = fmt.Fprintln(sess.Out, "  real name: "+rn)
	}
	if t := nilOrEmpty(p.Title); t != "" {
		_, _ = fmt.Fprintln(sess.Out, "  title:     "+t)
	}
	printStats(sess, p.Stats)
}

func printStats(sess *shell.Session, s gen.PlayerStats) {
	_, _ = fmt.Fprintf(sess.Out,
		"  rounds:   %d played, %d won\n"+
			"  wonders:  %d completed, %d destroyed\n"+
			"  raids:    %d launched, %d defended\n",
		s.RoundsPlayed, s.RoundsWon,
		s.WondersCompleted, s.WondersDestroyed,
		s.RaidsLaunched, s.RaidsDefended,
	)
}

// nilOrEmpty unwraps a gen.NilString to "" when null, otherwise to
// the underlying value.
func nilOrEmpty(s gen.NilString) string {
	if s.Null {
		return ""
	}
	return s.Value
}
