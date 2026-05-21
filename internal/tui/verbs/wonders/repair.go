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

// runWonderRepair implements `wonder repair [<hp>]`. With no arg, opens
// a single-field form for the HP count. The backend remains
// authoritative on the actual stone cost and the per-phase cap clamp;
// the confirm subtitle reminds the user of the §16.2 rules but does
// not pre-flight numbers — there's no `previewWonderRepair` endpoint
// (flagged in the tutorial as a backend co-evolution candidate).
func runWonderRepair(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	hp := 0
	switch len(args) {
	case 0:
		picked, perr := promptRepairHP(ctx)
		if perr != nil {
			if errors.Is(perr, selector.ErrCancelled) {
				return nil
			}
			return perr
		}
		hp = picked
	case 1:
		n, e := strconv.Atoi(strings.TrimSpace(args[0]))
		if e != nil || n < 1 {
			return errors.New("hp must be a positive integer")
		}
		hp = n
	default:
		return errors.New("usage: wonder repair [<hp>]")
	}

	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Repair %d HP?", hp),
		"Costs 8 Stone per HP. The backend clamps to the 2000 HP per-phase cap and pauses construction by 30 min per 500 HP repaired.")
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

	w, err := sess.API.RepairWonder(ctx, kingdomID, hp)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("repaired %s — HP %d/%d (status=%s).",
		titleName(string(w.Name)), w.Hp, w.TargetHp, string(w.Status)))
	if pu, ok := w.PausedUntil.Get(); ok {
		shell.Info(sess.Out, fmt.Sprintf("construction paused until %s (%s)",
			pu.UTC().Format("2006-01-02 15:04 UTC"), shared.RelTime(pu)))
	}
	return nil
}

// promptRepairHP opens the bare HP form. The validator accepts any
// positive integer; the backend handles the per-phase cap.
func promptRepairHP(ctx context.Context) (int, error) {
	res, err := selector.Form(ctx, "Repair wonder", []selector.Field{
		{
			Key:     "hp",
			Label:   "HP to repair (minimum 1; the backend clamps to the 2000 HP per-phase cap)",
			Initial: "",
			Validate: func(s string) error {
				n, e := strconv.Atoi(strings.TrimSpace(s))
				if e != nil || n < 1 {
					return errors.New("must be a positive integer")
				}
				return nil
			},
		},
	})
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(strings.TrimSpace(res["hp"]))
	return n, nil
}
