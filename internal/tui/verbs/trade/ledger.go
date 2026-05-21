package trade

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// ledgerLimitMax mirrors the spec's `maximum: 100` on listTradeLedger.
const ledgerLimitMax = 100

// ledgerDefaultLimit mirrors the spec's `default: 25` on listTradeLedger.
const ledgerDefaultLimit = 25

// sinceRE matches the duration shapes the backend's `since` parser
// accepts (numeric magnitude + unit). Checked client-side so an obvious
// typo fails before any HTTP round-trip; the backend remains the
// source of truth on which exact suffixes are valid.
var sinceRE = regexp.MustCompile(`^\d+[smhd]$|^\d+h\d+m$`)

func init() {
	shell.Register(&shell.Verb{
		Name:    "trade",
		Summary: "Inspect the world's trade ledger",
		Usage:   "trade ledger [--player H] [--since 24h] [--limit N] [--page N]",
		Sub: map[string]*shell.Verb{
			"ledger": {
				Name:    "ledger",
				Summary: "List the in-scope world's trade ledger (newest first)",
				Usage:   "trade ledger [--player H] [--since 24h] [--limit N] [--page N]",
				Run:     runTradeLedger,
				Flags: []shell.FlagSpec{
					{Name: "player", HasValue: true, Help: "filter to a sender/receiver/attacker handle"},
					{
						Name:     "since",
						HasValue: true,
						Help:     "only entries within the last duration (e.g. 24h, 7d)",
						Suggest:  shell.SuggestFunc(suggestSinceValues),
					},
					{
						Name:     "limit",
						HasValue: true,
						Help:     fmt.Sprintf("page size (1-%d, default %d)", ledgerLimitMax, ledgerDefaultLimit),
						Suggest:  shell.SuggestFunc(suggestLimitValues),
					},
					{Name: "page", HasValue: true, Help: "1-based page number (default 1)"},
				},
			},
		},
	})
}

func runTradeLedger(ctx context.Context, sess *shell.Session, _ []string, flags map[string]string) error {
	player, since, limit, page, err := parseLedgerFlags(flags)
	if err != nil {
		return err
	}
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	entries, pagy, err := sess.API.ListTradeLedger(ctx, worldID, player, since, limit, page)
	if err != nil {
		return err
	}
	printLedger(sess, entries, pagy, player, since)
	return nil
}

// parseLedgerFlags pulls the four flags off the parsed map with
// client-side bounds checks. Returns the empty string for absent
// player/since.
func parseLedgerFlags(flags map[string]string) (player, since string, limit, page int, err error) {
	player = strings.TrimSpace(flags["player"])
	since = strings.TrimSpace(flags["since"])
	if since != "" && !sinceRE.MatchString(since) {
		err = fmt.Errorf("--since must look like 24h, 7d, 30m, or 1h30m (got %q)", since)
		return
	}
	if s := strings.TrimSpace(flags["limit"]); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 {
			err = fmt.Errorf("--limit must be a positive integer (got %q)", s)
			return
		}
		if n > ledgerLimitMax {
			err = fmt.Errorf("--limit must be ≤ %d (got %d)", ledgerLimitMax, n)
			return
		}
		limit = n
	}
	if s := strings.TrimSpace(flags["page"]); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 {
			err = fmt.Errorf("--page must be a positive integer (got %q)", s)
			return
		}
		page = n
	}
	return
}

// printLedger renders the table. Empty pages collapse to "(none)";
// otherwise we print the "showing X-Y of Z" header, one row per entry,
// and a follow-up "more: N remaining" hint when there are further
// pages.
func printLedger(sess *shell.Session, entries []gen.TradeLedgerEntry, pagy gen.ListTradeLedgerOKPagy, player, since string) {
	if len(entries) == 0 {
		shell.Section(sess.Out, "Trade ledger:", "(none)")
		return
	}
	start := (pagy.Page-1)*pagy.Limit + 1
	end := start + len(entries) - 1
	filters := make([]string, 0, 2)
	if player != "" {
		filters = append(filters, "player="+player)
	}
	if since != "" {
		filters = append(filters, "since="+since)
	}
	header := fmt.Sprintf("Trade ledger (page %d of %d, showing %d-%d of %d)",
		pagy.Page, pagy.Pages, start, end, pagy.Count)
	if len(filters) > 0 {
		header += " [" + strings.Join(filters, " ") + "]"
	}
	header += ":"

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		note := ""
		if e.Status == gen.TradeLedgerEntryStatusIntercepted {
			if h, ok := e.AttackerHandle.Get(); ok && h != "" {
				note = "attacker=" + h
			}
		}
		lines = append(lines, fmt.Sprintf("%-20s  %-12s  %-20s  %-8s  %-7d  %s",
			e.RecordedAt.UTC().Format("2006-01-02 15:04 UTC"),
			string(e.Status),
			fmt.Sprintf("%s → %s", e.SenderHandle, e.ReceiverHandle),
			string(e.Resource),
			e.Amount,
			note))
	}
	shell.Section(sess.Out, header, strings.Join(lines, "\n"))

	if pagy.Page < pagy.Pages {
		remaining := pagy.Count - (start + len(entries) - 1)
		shell.Info(sess.Out, fmt.Sprintf("more: %d remaining — `trade ledger --page %d`",
			remaining, pagy.Page+1))
	}
}

// suggestSinceValues returns the static set of recommended duration
// strings for `--since <Tab>`. Aligned with what the backend's parser
// accepts in §16 trade flow.
func suggestSinceValues(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
	return []string{"1h", "24h", "7d", "30d"}, nil
}

// suggestLimitValues returns the static set of common page sizes for
// `--limit <Tab>`.
func suggestLimitValues(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
	return []string{"10", "25", "50", "100"}, nil
}
