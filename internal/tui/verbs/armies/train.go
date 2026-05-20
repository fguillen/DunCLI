package armies

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// trainingBuildings is the list of buildings that can train units,
// mirrored from gen.QueueTrainingOrderReqBuilding so completion works
// without an HTTP round-trip. Pinned by a regression test in
// train_test.go.
var trainingBuildings = []string{"barracks", "stable", "siege_workshop"}

// unitKinds is the full unit catalog, mirrored from gen.Unit. Pinned
// by a regression test. The CLI does not filter by building when the
// user supplies one — the backend's 422 `unit_not_trainable_here`
// surfaces as a normal error if the combo is invalid. (Backend
// co-evolution candidate: expose per-building unit mapping.)
var unitKinds = []string{
	"levy", "archer", "pikeman", "knight",
	"catapult", "royal_guard", "scout", "trebuchet",
}

func init() {
	shell.Register(&shell.Verb{
		Name:    "train",
		Summary: "Queue, preview, or cancel a unit training order",
		Usage:   "train <building> <unit> <count> | train preview ... | train cancel <id-or-unit>",
		Run:     runTrainDefault,
		Complete: shell.SuggestFunc(func(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
			out := append([]string{"preview", "cancel"}, trainingBuildings...)
			return out, nil
		}),
		Sub: map[string]*shell.Verb{
			"preview": {
				Name:    "preview",
				Summary: "Preview the cost / duration of a training order",
				Usage:   "train preview <building> <unit> <count>",
				Run:     runTrainPreview,
				Complete: shell.SuggestFunc(func(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
					return append([]string(nil), trainingBuildings...), nil
				}),
			},
			"cancel": {
				Name:     "cancel",
				Summary:  "Cancel an in-progress training order (75% refund)",
				Usage:    "train cancel <id-or-unit>",
				Run:      runTrainCancel,
				Complete: shell.SuggestFunc(suggestActiveTrainingArgs),
			},
		},
	})
}

// ── train (default = queue) ──────────────────────────────────────────

func runTrainDefault(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	building, unit, count, err := resolveTrainArgs(ctx, args)
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}

	prev, err := sess.API.PreviewTrainingOrder(ctx, kingdomID, building, unit, count)
	if err != nil {
		return err
	}
	printTrainPreview(sess, prev)
	if !prev.Affordable {
		return errors.New("can't afford this training order right now")
	}
	if !prev.BuildingBuilt {
		return fmt.Errorf("%s is not built yet", building)
	}

	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Queue training: %s × %d at %s?", unit, count, building),
		"Resources will be deducted immediately. Cancel later for a 75% refund.")
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

	ord, err := sess.API.QueueTrainingOrder(ctx, kingdomID, building, unit, count)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("queued: %s × %d at %s, completes %s",
		string(ord.Unit), ord.Count, string(ord.BuildingKind),
		ord.CompletesAt.Format("2006-01-02 15:04 MST")))
	return nil
}

// resolveTrainArgs collects (building, unit, count) from the positional
// args or, when any are missing, prompts the user via selectors and a
// numeric form.
func resolveTrainArgs(ctx context.Context, args []string) (building, unit string, count int, err error) {
	if len(args) > 3 {
		err = errors.New("usage: train [<building>] [<unit>] [<count>]")
		return
	}
	if len(args) >= 1 {
		building = args[0]
	}
	if len(args) >= 2 {
		unit = args[1]
	}
	if len(args) == 3 {
		count, err = strconv.Atoi(args[2])
		if err != nil {
			err = fmt.Errorf("count must be a positive integer, got %q", args[2])
			return
		}
		if count <= 0 {
			err = errors.New("count must be a positive integer")
			return
		}
	}

	if building == "" {
		items := make([]selector.Item, 0, len(trainingBuildings))
		for _, b := range trainingBuildings {
			items = append(items, selector.Item{Title: b, Value: b})
		}
		pick, perr := selector.Pick(ctx, "Pick a training building", items)
		if perr != nil {
			err = perr
			return
		}
		building = pick.Value
	}
	if !isKnown(trainingBuildings, building) {
		err = fmt.Errorf("unknown building %q (one of: %s)", building, strings.Join(trainingBuildings, ", "))
		return
	}

	if unit == "" {
		items := make([]selector.Item, 0, len(unitKinds))
		for _, u := range unitKinds {
			items = append(items, selector.Item{Title: u, Value: u})
		}
		pick, perr := selector.Pick(ctx, "Pick a unit", items)
		if perr != nil {
			err = perr
			return
		}
		unit = pick.Value
	}
	if !isKnown(unitKinds, unit) {
		err = fmt.Errorf("unknown unit %q (one of: %s)", unit, strings.Join(unitKinds, ", "))
		return
	}

	if count == 0 {
		res, ferr := selector.Form(ctx, "How many?", []selector.Field{{
			Key:     "count",
			Label:   "Count (positive integer)",
			Initial: "1",
			Validate: func(s string) error {
				n, e := strconv.Atoi(strings.TrimSpace(s))
				if e != nil || n <= 0 {
					return errors.New("must be a positive integer")
				}
				return nil
			},
		}})
		if ferr != nil {
			err = ferr
			return
		}
		count, _ = strconv.Atoi(strings.TrimSpace(res["count"]))
	}
	return
}

// ── train preview ────────────────────────────────────────────────────

func runTrainPreview(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 3 {
		return errors.New("usage: train preview <building> <unit> <count>")
	}
	building, unit := args[0], args[1]
	if !isKnown(trainingBuildings, building) {
		return fmt.Errorf("unknown building %q (one of: %s)", building, strings.Join(trainingBuildings, ", "))
	}
	if !isKnown(unitKinds, unit) {
		return fmt.Errorf("unknown unit %q (one of: %s)", unit, strings.Join(unitKinds, ", "))
	}
	count, err := strconv.Atoi(args[2])
	if err != nil || count <= 0 {
		return fmt.Errorf("count must be a positive integer, got %q", args[2])
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	prev, err := sess.API.PreviewTrainingOrder(ctx, kingdomID, building, unit, count)
	if err != nil {
		return err
	}
	printTrainPreview(sess, prev)
	return nil
}

// ── train cancel ─────────────────────────────────────────────────────

func runTrainCancel(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: train cancel <id-or-unit>")
	}
	arg := args[0]
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	orderID := arg
	if isKnown(unitKinds, arg) {
		kd, err := sess.API.ShowKingdom(ctx, kingdomID)
		if err != nil {
			return err
		}
		matches := make([]gen.TrainingOrder, 0, 1)
		for _, t := range kd.InProgressTraining {
			if string(t.Unit) == arg {
				matches = append(matches, t)
			}
		}
		switch len(matches) {
		case 0:
			return fmt.Errorf("no in-progress training order for unit %q", arg)
		case 1:
			orderID = matches[0].ID
		default:
			ids := make([]string, 0, len(matches))
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
			return fmt.Errorf("multiple in-progress orders for unit %q — pick by id: %s",
				arg, strings.Join(ids, ", "))
		}
	}

	ok, err := selector.Confirm(ctx,
		"Cancel this training order?",
		"This refunds 75% of resources spent. Elapsed time is lost.")
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

	ord, err := sess.API.CancelTrainingOrder(ctx, kingdomID, orderID)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("cancelled training %s (%s × %d)",
		ord.ID, string(ord.Unit), ord.Count))
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────

func isKnown(set []string, s string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// suggestActiveTrainingArgs is the dynamic Suggester for
// `train cancel <Tab>` — returns the unit kinds currently in flight
// plus the order IDs themselves so users can disambiguate either way.
func suggestActiveTrainingArgs(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	kd, err := sess.API.ShowKingdom(ctx, kingdomID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(kd.InProgressTraining)*2)
	for _, t := range kd.InProgressTraining {
		u := string(t.Unit)
		if !seen[u] {
			out = append(out, u)
			seen[u] = true
		}
		out = append(out, t.ID)
	}
	return out, nil
}
