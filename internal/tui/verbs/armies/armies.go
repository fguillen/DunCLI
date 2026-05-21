package armies

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

var armyNameRE = regexp.MustCompile(`^.{1,60}$`)

func init() {
	shell.Register(&shell.Verb{
		Name:    "armies",
		Summary: "List your kingdom's armies",
		Usage:   "armies",
		Run:     runArmiesList,
	})

	shell.Register(&shell.Verb{
		Name:    "army",
		Summary: "Inspect or mutate one army",
		Usage:   "army <show|split|rename|merge> ...",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:     "show",
				Summary:  "Show one army (composition, status, location)",
				Usage:    "army show <name>",
				Run:      runArmyShow,
				Complete: shell.SuggestFunc(suggestArmyNames),
			},
			"split": {
				Name:     "split",
				Summary:  "Peel units off a home army into a new one",
				Usage:    "army split <name>",
				Run:      runArmySplit,
				Complete: shell.SuggestFunc(suggestArmyNames),
			},
			"rename": {
				Name:     "rename",
				Summary:  "Rename an army (≤ 60 chars)",
				Usage:    "army rename <name> <new-name>",
				Run:      runArmyRename,
				Complete: shell.SuggestFunc(suggestArmyNames),
			},
			"merge": {
				Name:    "merge",
				Summary: "Merge one army into another (both home, same region)",
				Usage:   "army merge <name> --into <other-name>",
				Run:     runArmyMerge,
				Flags: []shell.FlagSpec{
					{Name: "into", HasValue: true, Help: "target army that will receive the merge"},
				},
				Complete: shell.SuggestFunc(suggestArmyNames),
			},
		},
	})
}

// ── armies (list) ────────────────────────────────────────────────────

func runArmiesList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	list, err := sess.API.ListKingdomArmies(ctx, kingdomID)
	if err != nil {
		return err
	}
	regionNames := shared.RegionNameMap(ctx, sess)
	printArmyList(sess, list, regionNames)
	return nil
}

// ── army show ────────────────────────────────────────────────────────

func runArmyShow(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: army show <name>")
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	armyID, err := sess.API.ResolveArmy(ctx, kingdomID, args[0])
	if err != nil {
		return err
	}
	a, err := sess.API.ShowArmy(ctx, armyID)
	if err != nil {
		return err
	}
	printArmy(sess, a, shared.LookupRegionName(ctx, sess, a.LocationRegionID))
	return nil
}

// ── army split ───────────────────────────────────────────────────────

func runArmySplit(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: army split <name>")
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	armyID, err := sess.API.ResolveArmy(ctx, kingdomID, args[0])
	if err != nil {
		return err
	}
	src, err := sess.API.ShowArmy(ctx, armyID)
	if err != nil {
		return err
	}
	if src.Status != gen.ArmyStatusHome {
		return fmt.Errorf("army %q is not home (status=%s)", src.Name, string(src.Status))
	}
	if len(src.Composition) == 0 {
		return fmt.Errorf("army %q has no units to split", src.Name)
	}

	fields := []selector.Field{{
		Key:   "name",
		Label: "New army name (1-60 chars)",
		Validate: func(s string) error {
			if !armyNameRE.MatchString(strings.TrimSpace(s)) {
				return errors.New("name must be 1-60 chars")
			}
			return nil
		},
	}}
	for _, k := range sortedKeys(src.Composition) {
		maxN := src.Composition[k]
		if maxN <= 0 {
			continue
		}
		kCopy := k
		maxCopy := maxN
		fields = append(fields, selector.Field{
			Key:     kCopy,
			Label:   fmt.Sprintf("%s (max %d)", kCopy, maxCopy),
			Initial: "0",
			Validate: func(s string) error {
				n, e := strconv.Atoi(strings.TrimSpace(s))
				if e != nil || n < 0 {
					return errors.New("must be a non-negative integer")
				}
				if n > maxCopy {
					return fmt.Errorf("at most %d available", maxCopy)
				}
				return nil
			},
		})
	}

	res, err := selector.Form(ctx, "Split army "+src.Name, fields)
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}

	units := make(map[string]int, len(fields)-1)
	for k := range src.Composition {
		n, _ := strconv.Atoi(strings.TrimSpace(res[k]))
		if n > 0 {
			units[k] = n
		}
	}
	if len(units) == 0 {
		return errors.New("at least one unit must be split")
	}

	out, err := sess.API.SplitArmy(ctx, armyID, strings.TrimSpace(res["name"]), units)
	if err != nil {
		return err
	}
	newArmy := out.New
	shell.Success(sess.Out, fmt.Sprintf("split: created %q (id=%s) with %s",
		newArmy.Name, newArmy.ID, compositionString(newArmy.Composition)))
	if _, ok := out.Source.Get(); !ok && out.Source.IsNull() {
		shell.Info(sess.Out, "source army was emptied and removed")
	}
	return nil
}

// ── army rename ──────────────────────────────────────────────────────

func runArmyRename(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 2 {
		return errors.New("usage: army rename <name> <new-name>")
	}
	newName := strings.TrimSpace(args[1])
	if !armyNameRE.MatchString(newName) {
		return errors.New("new name must be 1-60 chars")
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	armyID, err := sess.API.ResolveArmy(ctx, kingdomID, args[0])
	if err != nil {
		return err
	}
	updated, err := sess.API.RenameArmy(ctx, armyID, newName)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("renamed: %s (id=%s)", updated.Name, updated.ID))
	return nil
}

// ── army merge ───────────────────────────────────────────────────────

func runArmyMerge(ctx context.Context, sess *shell.Session, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: army merge <name> --into <other-name>")
	}
	target := strings.TrimSpace(flags["into"])
	if target == "" {
		return errors.New("--into <name> is required")
	}
	if target == args[0] {
		return errors.New("cannot merge an army into itself")
	}
	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	sourceID, err := sess.API.ResolveArmy(ctx, kingdomID, args[0])
	if err != nil {
		return err
	}
	targetID, err := sess.API.ResolveArmy(ctx, kingdomID, target)
	if err != nil {
		return err
	}

	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Merge %s into %s?", args[0], target),
		"Both armies must be home and in the same region. The source army will be removed.")
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

	merged, err := sess.API.MergeArmy(ctx, targetID, sourceID)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("merged into %s: %s",
		merged.Name, compositionString(merged.Composition)))
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────

func sortedKeys(c gen.Composition) []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	// stable sort via the unitKinds order if known, else alphabetical
	indexOf := func(s string) int {
		for i, u := range unitKinds {
			if u == s {
				return i
			}
		}
		return len(unitKinds)
	}
	// bubble-style stable sort — fine for ≤ 8 entries
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && indexOf(out[j]) < indexOf(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// suggestArmyNames feeds tab completion for every `army <sub> <Tab>`
// position that takes an army-name argument.
func suggestArmyNames(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
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
		out = append(out, a.Name)
	}
	return out, nil
}
