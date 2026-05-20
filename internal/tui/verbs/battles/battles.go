// Package battles hosts the Phase 9 combat verbs: `battles` (list the
// caller's kingdom's battle history) and `battle show <id>` (full
// round-by-round detail for one fight). It uses the kingdom scope set
// by Phase 7 and pairs with Phase 8 marches as the producer of new
// battles.
package battles

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// listLimitMax mirrors the spec's `maximum: 100` on listKingdomBattles.
const listLimitMax = 100

func init() {
	shell.Register(&shell.Verb{
		Name:    "battles",
		Summary: "List your kingdom's battle history (newest first)",
		Usage:   "battles [--limit N] [--offset N]",
		Run:     runBattlesList,
		Flags: []shell.FlagSpec{
			{Name: "limit", HasValue: true, Help: "page size (1-100, default 25)"},
			{Name: "offset", HasValue: true, Help: "pagination offset (default 0)"},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "battle",
		Summary: "Inspect one battle",
		Usage:   "battle show <id>",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:     "show",
				Summary:  "Show one battle with round-by-round log + participants",
				Usage:    "battle show <id>",
				Run:      runBattleShow,
				Complete: shell.SuggestFunc(suggestBattleIDs),
			},
		},
	})
}

// ── battles (list) ───────────────────────────────────────────────────

func runBattlesList(ctx context.Context, sess *shell.Session, _ []string, flags map[string]string) error {
	limit, offset, err := parseListFlags(flags)
	if err != nil {
		return err
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	battles, total, err := sess.API.ListKingdomBattles(ctx, kingdomID, limit, offset)
	if err != nil {
		return err
	}
	printBattleList(sess, battles, total, effectiveLimit(limit), offset, kingdomID, regionNameMap(ctx, sess))
	return nil
}

// ── battle show ──────────────────────────────────────────────────────

func runBattleShow(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: battle show <id>")
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	battle, parts, err := sess.API.ShowBattle(ctx, strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	printBattleDetail(sess, battle, parts, kingdomID, regionNameMap(ctx, sess))
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────

func parseListFlags(flags map[string]string) (limit, offset int, err error) {
	if s := strings.TrimSpace(flags["limit"]); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 {
			return 0, 0, fmt.Errorf("--limit must be a positive integer (got %q)", s)
		}
		if n > listLimitMax {
			return 0, 0, fmt.Errorf("--limit must be ≤ %d (got %d)", listLimitMax, n)
		}
		limit = n
	}
	if s := strings.TrimSpace(flags["offset"]); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 0 {
			return 0, 0, fmt.Errorf("--offset must be a non-negative integer (got %q)", s)
		}
		offset = n
	}
	return limit, offset, nil
}

// effectiveLimit returns the value the backend will actually have used
// — 25 (the spec default) when the user passed nothing. Drives the
// list footer's "showing X-Y of Z".
func effectiveLimit(requested int) int {
	if requested <= 0 {
		return 25
	}
	return requested
}

// regionNameMap resolves region ID → name for the in-scope world if
// possible. Returns an empty map on any error (the caller falls back
// to IDs without complaining). Mirrors armies/armies.go on purpose so
// cross-world history just renders the raw region_id.
//
// Backend co-evolution candidate: embedding `region_name` on Battle
// would close the gap for archived worlds the caller isn't in any
// more.
func regionNameMap(ctx context.Context, sess *shell.Session) map[string]string {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return map[string]string{}
	}
	regions, err := sess.API.ShowWorldMap(ctx, worldID)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(regions))
	for _, r := range regions {
		out[r.ID] = r.Name
	}
	return out
}

// suggestBattleIDs feeds tab completion for `battle show <Tab>`. It
// fetches the first page of the caller's battle history and returns
// each ID. Honors ctx (the completion engine times us out at 800 ms).
func suggestBattleIDs(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	battles, _, err := sess.API.ListKingdomBattles(ctx, kingdomID, 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(battles))
	for _, b := range battles {
		out = append(out, b.ID)
	}
	return out, nil
}
