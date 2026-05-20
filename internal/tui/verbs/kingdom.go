package verbs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// buildingKinds is the §17 building catalog, mirrored from the spec
// enum so we can complete kind args without an HTTP round-trip. A
// regression test in kingdom_test.go pins this against the generated
// `gen.BuildingUpgradePreviewKind.AllValues()` so spec drift fails
// loudly.
var buildingKinds = []string{
	"town_hall", "gold_mint", "lumber_camp", "quarry", "iron_mine",
	"warehouse", "barracks", "stable", "siege_workshop", "walls",
	"watchtower", "stone_mason",
}

func init() {
	shell.Register(&shell.Verb{
		Name:    "kingdom",
		Summary: "Show your kingdom dashboard on the in-scope world",
		Usage:   "kingdom [show]",
		Run:     runKingdomShow,
		Sub: map[string]*shell.Verb{
			"show": {
				Name:    "show",
				Summary: "Show your kingdom dashboard",
				Usage:   "kingdom show",
				Run:     runKingdomShow,
			},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "buildings",
		Summary: "List your kingdom's buildings with upgrade detail",
		Usage:   "buildings [--upgradable]",
		Run:     runBuildingsList,
		Flags: []shell.FlagSpec{
			{Name: "upgradable", HasValue: false, Help: "only show buildings that can be upgraded right now"},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "build",
		Summary: "Queue, preview, or cancel a building upgrade",
		Usage:   "build <kind> | build preview <kind> | build cancel <id-or-kind>",
		Run:     runBuildDefault,
		Complete: shell.SuggestFunc(func(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
			return append([]string{"preview", "cancel"}, buildingKinds...), nil
		}),
		Sub: map[string]*shell.Verb{
			"preview": {
				Name:    "preview",
				Summary: "Preview the cost of the next upgrade for a building",
				Usage:   "build preview <kind>",
				Run:     runBuildPreview,
				Complete: shell.SuggestFunc(func(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
					return append([]string(nil), buildingKinds...), nil
				}),
			},
			"cancel": {
				Name:     "cancel",
				Summary:  "Cancel an in-progress build (75% refund, time lost)",
				Usage:    "build cancel <id-or-kind>",
				Run:      runBuildCancel,
				Complete: shell.SuggestFunc(suggestActiveBuildKinds),
			},
		},
	})
}

// ── kingdom ──────────────────────────────────────────────────────────

func runKingdomShow(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	kingdomID, err := requireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	kd, err := sess.API.ShowKingdom(ctx, kingdomID)
	if err != nil {
		return err
	}
	printKingdom(sess, kd)
	return nil
}

// ── buildings ────────────────────────────────────────────────────────

func runBuildingsList(ctx context.Context, sess *shell.Session, _ []string, flags map[string]string) error {
	kingdomID, err := requireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	var upgradable *bool
	if _, ok := flags["upgradable"]; ok {
		t := true
		upgradable = &t
	}
	list, err := sess.API.ListKingdomBuildings(ctx, kingdomID, upgradable)
	if err != nil {
		return err
	}
	if list == nil || len(list.Buildings) == 0 {
		shell.Section(sess.Out, "Buildings:", "")
		return nil
	}
	rows := make([]gen.KingdomBuilding, len(list.Buildings))
	copy(rows, list.Buildings)
	sort.Slice(rows, func(i, j int) bool {
		return string(rows[i].Kind) < string(rows[j].Kind)
	})

	var lines []string
	for _, b := range rows {
		state := "—"
		if b.AtMaxLevel {
			state = "max"
		} else if b.UpgradePossible {
			state = "ready"
		} else if !b.TierGatesMet {
			state = "tier-gated"
		} else if !b.Affordable {
			state = "unaffordable"
		}
		if bo, ok := b.BuildOrder.Get(); ok {
			state = "building (ETA " + relTime(bo.CompletesAt) + ")"
		}
		lines = append(lines, fmt.Sprintf("%-15s  L%-2d  %s", string(b.Kind), b.CurrentLevel, state))
	}
	shell.Section(sess.Out, "Buildings:", strings.Join(lines, "\n"))
	return nil
}

// ── build (default = queue) ──────────────────────────────────────────

// runBuildDefault implements `build <kind>` (no sub-verb). If the
// dispatcher routes here with no args we offer an interactive picker
// over the upgradable buildings.
func runBuildDefault(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	kingdomID, err := requireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	kind := ""
	if len(args) == 1 {
		kind = args[0]
	} else if len(args) > 1 {
		return errors.New("usage: build <kind>")
	}

	if kind == "" {
		picked, perr := pickUpgradableBuilding(ctx, sess, kingdomID)
		if perr != nil {
			if errors.Is(perr, selector.ErrCancelled) {
				return nil
			}
			return perr
		}
		kind = picked
	}
	if !isKnownBuildingKind(kind) {
		return fmt.Errorf("unknown building kind %q (try one of: %s)", kind, strings.Join(buildingKinds, ", "))
	}

	prev, err := sess.API.PreviewBuildUpgrade(ctx, kingdomID, kind)
	if err != nil {
		return err
	}
	printPreview(sess, prev)
	if prev.AtMaxLevel {
		return errors.New("already at max level")
	}
	if !prev.TierGatesMet {
		return errors.New("tier gates unmet — see preview above")
	}
	if !prev.Affordable {
		return errors.New("can't afford this upgrade right now")
	}
	target, ok := prev.TargetLevel.Get()
	if !ok {
		return errors.New("preview returned no target_level — backend bug, please retry")
	}

	ok, err = selector.Confirm(ctx,
		fmt.Sprintf("Queue upgrade: %s → L%d?", kind, target),
		"Resources will be deducted immediately. You can cancel later for a 75% refund.")
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

	ord, err := sess.API.QueueBuildOrder(ctx, kingdomID, kind, target)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("queued: %s → L%d, completes %s",
		string(ord.Kind), ord.TargetLevel, ord.CompletesAt.Format("2006-01-02 15:04 MST")))
	return nil
}

// ── build preview ────────────────────────────────────────────────────

func runBuildPreview(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: build preview <kind>")
	}
	kind := args[0]
	if !isKnownBuildingKind(kind) {
		return fmt.Errorf("unknown building kind %q (try one of: %s)", kind, strings.Join(buildingKinds, ", "))
	}
	kingdomID, err := requireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	prev, err := sess.API.PreviewBuildUpgrade(ctx, kingdomID, kind)
	if err != nil {
		return err
	}
	printPreview(sess, prev)
	return nil
}

// ── build cancel ─────────────────────────────────────────────────────

func runBuildCancel(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: build cancel <id-or-kind>")
	}
	arg := args[0]
	kingdomID, err := requireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	orderID := arg
	if isKnownBuildingKind(arg) {
		// Resolve the active build order for this kind via ShowKingdom.
		kd, err := sess.API.ShowKingdom(ctx, kingdomID)
		if err != nil {
			return err
		}
		matches := make([]gen.BuildOrder, 0, 1)
		for _, b := range kd.InProgressBuilds {
			if string(b.Kind) == arg {
				matches = append(matches, b)
			}
		}
		switch len(matches) {
		case 0:
			return fmt.Errorf("no in-progress build order for %q", arg)
		case 1:
			orderID = matches[0].ID
		default:
			ids := make([]string, 0, len(matches))
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
			return fmt.Errorf("multiple in-progress orders for %q — pick by id: %s",
				arg, strings.Join(ids, ", "))
		}
	}

	ok, err := selector.Confirm(ctx,
		"Cancel this build order?",
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

	ord, err := sess.API.CancelBuildOrder(ctx, kingdomID, orderID)
	if err != nil {
		return err
	}
	shell.Success(sess.Out, fmt.Sprintf("cancelled build %s (%s)", ord.ID, string(ord.Kind)))
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────

// requireKingdomID is the Phase 7 equivalent of requireWorldID; it
// additionally resolves the caller's kingdom in the in-scope world.
func requireKingdomID(ctx context.Context, sess *shell.Session) (string, error) {
	worldID, err := requireWorldID(ctx, sess)
	if err != nil {
		return "", err
	}
	id, err := sess.API.ResolveKingdom(ctx, worldID)
	if err != nil {
		if apiErr := api.AsError(err); apiErr != nil && apiErr.Code == "not_found" {
			return "", errors.New("you have no kingdom in this world — try `world join <slug>` first")
		}
		return "", err
	}
	return id, nil
}

func isKnownBuildingKind(s string) bool {
	for _, k := range buildingKinds {
		if k == s {
			return true
		}
	}
	return false
}

func relTime(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return "ready"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func pickUpgradableBuilding(ctx context.Context, sess *shell.Session, kingdomID string) (string, error) {
	yes := true
	list, err := sess.API.ListKingdomBuildings(ctx, kingdomID, &yes)
	if err != nil {
		return "", err
	}
	if list == nil || len(list.Buildings) == 0 {
		return "", errors.New("no upgradable buildings right now")
	}
	items := make([]selector.Item, 0, len(list.Buildings))
	for _, b := range list.Buildings {
		target, _ := b.TargetLevel.Get()
		items = append(items, selector.Item{
			Title:       string(b.Kind),
			Description: fmt.Sprintf("L%d → L%d", b.CurrentLevel, target),
			Value:       string(b.Kind),
		})
	}
	pick, err := selector.Pick(ctx, "Pick a building to upgrade", items)
	if err != nil {
		return "", err
	}
	return pick.Value, nil
}

// suggestActiveBuildKinds is the dynamic Suggester for
// `build cancel <Tab>` — returns the kinds with an active build order
// plus the order IDs themselves so users can disambiguate either way.
func suggestActiveBuildKinds(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	kingdomID, err := requireKingdomID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	kd, err := sess.API.ShowKingdom(ctx, kingdomID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(kd.InProgressBuilds)*2)
	for _, b := range kd.InProgressBuilds {
		if !seen[string(b.Kind)] {
			out = append(out, string(b.Kind))
			seen[string(b.Kind)] = true
		}
		out = append(out, b.ID)
	}
	return out, nil
}

// ── printers ─────────────────────────────────────────────────────────

func printKingdom(sess *shell.Session, kd *gen.KingdomDetail) {
	sp := kd.Stockpiles
	pr := kd.ProductionRates
	shell.Strong(sess.Out, "Kingdom "+kd.ID)
	shell.Section(sess.Out, "Stockpile (cap "+itoa(kd.WarehouseCap)+"):",
		fmt.Sprintf("gold=%d  wood=%d  stone=%d  iron=%d", sp.Gold, sp.Wood, sp.Stone, sp.Iron))
	shell.Section(sess.Out, "Production (per hour):",
		fmt.Sprintf("gold=%d  wood=%d  stone=%d  iron=%d", pr.Gold, pr.Wood, pr.Stone, pr.Iron))

	var ipb []string
	for _, b := range kd.InProgressBuilds {
		ipb = append(ipb, fmt.Sprintf("%s → L%d  (ETA %s)",
			string(b.Kind), b.TargetLevel, relTime(b.CompletesAt)))
	}
	shell.Section(sess.Out, "Builds in progress:", strings.Join(ipb, "\n"))

	var ipt []string
	for _, t := range kd.InProgressTraining {
		ipt = append(ipt, fmt.Sprintf("%s x%d at %s (ETA %s)",
			string(t.Unit), t.Count, string(t.BuildingKind), relTime(t.CompletesAt)))
	}
	shell.Section(sess.Out, "Training in progress:", strings.Join(ipt, "\n"))
}

func printPreview(sess *shell.Session, p *gen.BuildingUpgradePreview) {
	shell.Strong(sess.Out, fmt.Sprintf("%s upgrade preview", string(p.Kind)))
	if p.AtMaxLevel {
		_, _ = fmt.Fprintln(sess.Out, "  at max level (L"+itoa(p.CurrentLevel)+")")
		return
	}
	target, _ := p.TargetLevel.Get()
	_, _ = fmt.Fprintf(sess.Out, "  level:     L%d → L%d\n", p.CurrentLevel, target)
	if cost, ok := p.Cost.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  cost:      gold=%d wood=%d stone=%d iron=%d\n",
			cost.Gold, cost.Wood, cost.Stone, cost.Iron)
	}
	if d, ok := p.DurationSeconds.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  duration:  %s\n", relTime(time.Now().Add(time.Duration(d)*time.Second)))
	}
	if p.TierGatesMet {
		_, _ = fmt.Fprintln(sess.Out, "  tier:      gates met")
	} else {
		var parts []string
		for _, g := range p.TierGatesUnmet {
			parts = append(parts, fmt.Sprintf("%s L%d (have L%d)", g.Kind, g.RequiredLevel, g.CurrentLevel))
		}
		_, _ = fmt.Fprintln(sess.Out, "  tier:      unmet — needs "+strings.Join(parts, ", "))
	}
	if p.Affordable {
		_, _ = fmt.Fprintln(sess.Out, "  affords:   yes")
	} else {
		_, _ = fmt.Fprintf(sess.Out, "  affords:   no (missing gold=%d wood=%d stone=%d iron=%d)\n",
			p.Missing.Gold, p.Missing.Wood, p.Missing.Stone, p.Missing.Iron)
	}
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }
