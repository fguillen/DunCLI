package wonders

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// validMilestonePercents mirrors the spec enum and is used for the
// client-side arg validation.
var validMilestonePercents = []int{25, 50, 75}

// runWonderMilestone implements `wonder milestone [25|50|75]`. With
// no arg the verb auto-uses the pending percent (skipping the picker —
// only one threshold is ever pending at a time, mirroring Phase 11's
// "only home army" auto-pick). With an arg the verb validates the
// percent is in the spec enum AND matches the wonder's pending state
// before sending. The cost shown in the confirm comes from
// `pending_milestone_cost` (backend-provided, no client-side table).
func runWonderMilestone(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	cur, err := sess.API.GetWonder(ctx, kingdomID)
	if err != nil {
		return err
	}
	if cur == nil {
		return errors.New("no wonder under construction — try `wonder start <name>` first")
	}

	pending, hasPending := cur.PendingMilestonePercent.Get()
	if !hasPending {
		return errors.New("no milestone pending — construction is not paused at a threshold")
	}

	percent := 0
	switch len(args) {
	case 0:
		percent = int(pending)
	case 1:
		n, e := strconv.Atoi(strings.TrimSpace(args[0]))
		if e != nil || !validPercent(n) {
			return fmt.Errorf("percent must be one of %s (got %q)", joinInts(validMilestonePercents), args[0])
		}
		if n != int(pending) {
			return fmt.Errorf("milestone %d%% is not pending (waiting for %d%%)", n, int(pending))
		}
		percent = n
	default:
		return errors.New("usage: wonder milestone [25|50|75]")
	}

	costStr := "(unspecified)"
	if pc, ok := cur.PendingMilestoneCost.Get(); ok {
		costStr = renderResourceMap(map[string]int(pc))
	}

	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Pay the %d%% milestone for %s?", percent, titleName(string(cur.Name))),
		fmt.Sprintf("Cost: %s. Construction resumes immediately on success.", costStr))
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}
	if !ok {
		shell.Info(sess.Out, "aborted")
		return nil
	}

	w, err := sess.API.PayWonderMilestone(ctx, kingdomID, percent)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("paid milestone %d%% on %s — construction resumed (HP %d/%d).",
		percent, titleName(string(w.Name)), w.Hp, w.TargetHp))
	return nil
}

func validPercent(n int) bool {
	for _, p := range validMilestonePercents {
		if p == n {
			return true
		}
	}
	return false
}

func joinInts(ns []int) string {
	parts := make([]string, 0, len(ns))
	for _, n := range ns {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, "/")
}
