package armies

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// marchIntents mirrors gen.MarchOrderIntent. Pinned by regression test.
var marchIntents = []string{
	"attack", "reinforce", "scout", "capture", "claim_ruin", "caravan",
}

// marchIntentHelp gives the one-line description shown in the intent
// picker.
var marchIntentHelp = map[string]string{
	"attack":     "engage a defender (combat resolves on arrival)",
	"reinforce":  "join a friendly army at the target region",
	"scout":      "fast recon — observe defenders without engaging",
	"capture":    "seize a wilderness node (requires catapult)",
	"claim_ruin": "claim a ruin's reward (grants warehouse-capped cache)",
	"caravan":    "deliver a trade payload to another player",
}

func init() {
	shell.Register(&shell.Verb{
		Name:     "march",
		Summary:  "Dispatch an army to a target region",
		Usage:    "march <army> <target-region> [intent]",
		Run:      runMarchDispatch,
		Complete: shell.SuggestFunc(suggestMarchArg0),
	})

	shell.Register(&shell.Verb{
		Name:     "recall",
		Summary:  "Recall an army's active march (returns to home region)",
		Usage:    "recall <army>",
		Run:      runRecall,
		Complete: shell.SuggestFunc(suggestRecallableArmies),
	})
}

// ── march ────────────────────────────────────────────────────────────

func runMarchDispatch(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) < 2 || len(args) > 3 {
		return errors.New("usage: march <army> <target-region> [intent]")
	}
	armyName := args[0]
	regionName := args[1]
	intent := ""
	if len(args) == 3 {
		intent = args[2]
	}

	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}

	armyID, err := sess.API.ResolveArmy(ctx, kingdomID, armyName)
	if err != nil {
		return err
	}
	regionID, err := sess.API.ResolveRegion(ctx, worldID, regionName)
	if err != nil {
		return err
	}

	if intent == "" {
		items := make([]selector.Item, 0, len(marchIntents))
		for _, k := range marchIntents {
			items = append(items, selector.Item{
				Title:       k,
				Description: marchIntentHelp[k],
				Value:       k,
			})
		}
		pick, perr := selector.Pick(ctx, "Pick a march intent", items)
		if perr != nil {
			if errors.Is(perr, selector.ErrCancelled) {
				return nil
			}
			return perr
		}
		intent = pick.Value
	}
	if !isKnown(marchIntents, intent) {
		return fmt.Errorf("unknown intent %q (one of: %s)", intent, strings.Join(marchIntents, ", "))
	}

	march, err := sess.API.DispatchMarch(ctx, armyID, regionID, intent)
	if err != nil {
		return err
	}
	shared.PrintMarchOrder(sess, march, shared.PathNames(ctx, sess, march.Path))
	return nil
}

// ── recall ───────────────────────────────────────────────────────────

func runRecall(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: recall <army>")
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	armyID, err := sess.API.ResolveArmy(ctx, kingdomID, args[0])
	if err != nil {
		return err
	}
	march, err := sess.API.RecallMarch(ctx, armyID)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("recalled: army %s returning, arrives %s",
		args[0], march.ArrivesAt.Format("2006-01-02 15:04 MST")))
	shared.PrintMarchOrder(sess, march, shared.PathNames(ctx, sess, march.Path))
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────

// suggestMarchArg0 completes the first positional arg — the army name.
// It does not try to anticipate the region/intent positions because
// readline's per-token completion already routes them to the right
// suggester once a top-level completer is wired here.
func suggestMarchArg0(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	return suggestArmyNames(ctx, sess, "")
}

// suggestRecallableArmies completes `recall <Tab>` with the armies
// whose status is anything other than `home` — i.e. those that
// plausibly have an active march. Falls back to all armies if the
// kingdom lookup fails.
func suggestRecallableArmies(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	list, err := sess.API.ListKingdomArmies(ctx, kingdomID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, a := range list {
		if a.Status != gen.ArmyStatusHome {
			out = append(out, a.Name)
		}
	}
	return out, nil
}
