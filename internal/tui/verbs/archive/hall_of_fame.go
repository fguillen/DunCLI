package archive

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// leaderboardKinds is the canonical render order for the four §17.4
// leaderboards. Keys outside this set are appended after, alphabetical,
// so a backend-added kind doesn't vanish.
var leaderboardKinds = []string{"champions", "wreckers", "warlords", "veterans"}

// kindHelp annotates each leaderboard at render time. Mirrors the
// game-design §17.4 categorization so the table column for `secondary`
// has a label.
var kindHelp = map[string]struct {
	title        string
	score        string // label for the `score` column
	second       string // label for the `secondary` column ("" hides it)
	defaultLimit int
}{
	"champions": {"Champions", "wonders", "destroyed", 5},
	"wreckers":  {"Wreckers", "destroyed", "rounds", 5},
	"warlords":  {"Warlords", "raids", "victories", 5},
	"veterans":  {"Veterans", "rounds", "won", 5},
}

func init() {
	shell.Register(&shell.Verb{
		Name:    "hall-of-fame",
		Summary: "Per-server leaderboards (champions, wreckers, warlords, veterans)",
		Usage:   "hall-of-fame [--kind champions|wreckers|warlords|veterans]",
		Run:     runHallOfFame,
		Flags: []shell.FlagSpec{{
			Name:     "kind",
			HasValue: true,
			Help:     "restrict to a single leaderboard",
			Suggest:  shell.SuggestFunc(suggestKinds),
		}},
	})
}

// runHallOfFame implements `hall-of-fame [--kind ...]`. Empty `--kind`
// returns all four leaderboards. Validation of the value is
// client-side so a typo doesn't burn an HTTP round-trip; the spec
// returns 422 on an unknown kind, which we never want to see.
func runHallOfFame(ctx context.Context, sess *shell.Session, args []string, flags map[string]string) error {
	if len(args) > 0 {
		return errors.New("usage: hall-of-fame [--kind champions|wreckers|warlords|veterans]")
	}
	kind := strings.TrimSpace(flags["kind"])
	if kind != "" && !isValidKind(kind) {
		return fmt.Errorf("--kind must be one of %s (got %q)",
			strings.Join(leaderboardKinds, "/"), kind)
	}

	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}

	hof, err := sess.API.ShowHallOfFame(ctx, serverID, kind)
	if err != nil {
		return err
	}
	printHallOfFame(sess, serverSlug, hof, kind == "")
	return nil
}

func isValidKind(k string) bool {
	for _, v := range leaderboardKinds {
		if v == k {
			return true
		}
	}
	return false
}

// suggestKinds feeds tab completion for `--kind <Tab>`. The set is
// fixed by the spec; the verb still rejects unknown values
// client-side before any HTTP call.
func suggestKinds(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
	out := make([]string, len(leaderboardKinds))
	copy(out, leaderboardKinds)
	return out, nil
}

// printHallOfFame renders one or more leaderboards. When `summary` is
// true (no --kind passed) each board is capped at its `defaultLimit`
// entries and a hint points at `--kind <name>` for the full list. When
// `summary` is false the full board is rendered.
func printHallOfFame(sess *shell.Session, serverSlug string, hof *gen.HallOfFame, summary bool) {
	shell.Strong(sess.Out, fmt.Sprintf("Hall of Fame  (server=%s)", serverSlug))

	if len(hof.Leaderboards) == 0 {
		shell.Info(sess.Out, "no leaderboards available yet — snapshots are recomputed at round end")
		return
	}

	for _, kind := range orderedKinds(hof.Leaderboards) {
		board := hof.Leaderboards[kind]
		printLeaderboard(sess, kind, board, summary)
	}
	if summary {
		shell.Info(sess.Out, "full list per board: `hall-of-fame --kind <champions|wreckers|warlords|veterans>`")
	}
}

// orderedKinds returns the boards present in the response in the
// canonical order, followed by any unknown kinds alphabetically.
func orderedKinds(boards gen.HallOfFameLeaderboards) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(boards))
	for _, k := range leaderboardKinds {
		if _, ok := boards[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	extras := make([]string, 0)
	for k := range boards {
		if !seen[k] {
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)
	return append(out, extras...)
}

// printLeaderboard renders one board: a strong section header with the
// snapshot date, then either "(none)" or a numbered list. The
// secondary column is labeled per `kindHelp` for the canonical kinds;
// unknown kinds fall back to `score` / `secondary`.
func printLeaderboard(sess *shell.Session, kind string, board gen.Leaderboard, summary bool) {
	meta, known := kindHelp[kind]
	title := kind
	if known {
		title = meta.title
	}

	header := title
	if t, ok := board.SnapshotAt.Get(); ok {
		header = fmt.Sprintf("%s  (snapshot %s)", title, t.UTC().Format("2006-01-02 15:04 UTC"))
	} else {
		header += "  (no snapshot yet)"
	}

	entries := board.Entries
	limit := len(entries)
	if summary && known && meta.defaultLimit > 0 && limit > meta.defaultLimit {
		entries = entries[:meta.defaultLimit]
	}

	if len(entries) == 0 {
		shell.Section(sess.Out, header+":", "")
		return
	}

	lines := make([]string, 0, len(entries))
	scoreLabel, secondaryLabel := "score", "secondary"
	if known {
		if meta.score != "" {
			scoreLabel = meta.score
		}
		if meta.second != "" {
			secondaryLabel = meta.second
		}
	}
	for i, e := range entries {
		handle := "(deleted)"
		if v, ok := e.Handle.Get(); ok && v != "" {
			handle = v
		}
		titleSuffix := ""
		if v, ok := e.Title.Get(); ok && v != "" {
			titleSuffix = "  title=" + v
		}
		lines = append(lines, fmt.Sprintf("%2d. %-14s  %s=%d  %s=%d%s",
			i+1, handle, scoreLabel, e.Score, secondaryLabel, e.Secondary, titleSuffix))
	}
	body := strings.Join(lines, "\n")
	if summary && len(board.Entries) > len(entries) {
		body += fmt.Sprintf("\n... %d more — `hall-of-fame --kind %s`", len(board.Entries)-len(entries), kind)
	}
	shell.Section(sess.Out, header+":", body)
}
