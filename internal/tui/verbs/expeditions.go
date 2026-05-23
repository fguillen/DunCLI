package verbs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// Phase 10 — Nodes & ruins capture flows.
//
// Three sub-verbs (`node capture`, `node attack`, `ruin claim`) that
// compose existing endpoints — `listNodes`, `listRuins`,
// `listKingdomArmies`, `dispatchMarch` — into one-line wizards. No new
// backend endpoints; the actual outcome (Nodes::Capture,
// Nodes::Attack, Ruins::Claim) resolves at march arrival and surfaces
// through the Phase 9 battles stream.
//
// Sub-verb registrations live next to the parent `node` / `ruin`
// declarations in regions.go.

// expeditionTarget is one capturable / attackable / claimable region
// distilled out of the broader listNodes / listRuins responses.
type expeditionTarget struct {
	RegionID   string
	RegionName string
	// description renders in the picker's secondary line.
	description string
	// previewLines render in the dispatch-preview block.
	previewLines []string
}

// runNodeCapture implements `node capture [<region>]`.
func runNodeCapture(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	worldID, kingdomID, err := requireWorldAndKingdom(ctx, sess)
	if err != nil {
		return err
	}
	targets, err := capturableTargets(ctx, sess, worldID)
	if err != nil {
		return err
	}
	return runExpedition(ctx, sess, args, expeditionFlow{
		worldID:         worldID,
		kingdomID:       kingdomID,
		name:            "node capture",
		intent:          "capture",
		pickTitle:       "Pick a wilderness node to capture",
		emptyError:      "no wilderness nodes to capture in this world",
		confirmSubtitle: "Wilderness garrisons don't retreat. Catapults are required to break them.",
		followUp:        "track this fight with `battles` — outcome surfaces when the march arrives",
		targets:         targets,
	})
}

// runNodeAttack implements `node attack [<region>]`.
func runNodeAttack(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	worldID, kingdomID, err := requireWorldAndKingdom(ctx, sess)
	if err != nil {
		return err
	}
	targets, err := attackableTargets(ctx, sess, worldID, kingdomID)
	if err != nil {
		return err
	}
	return runExpedition(ctx, sess, args, expeditionFlow{
		worldID:         worldID,
		kingdomID:       kingdomID,
		name:            "node attack",
		intent:          "capture",
		pickTitle:       "Pick a foreign-owned node to attack",
		emptyError:      "no foreign-owned nodes to attack in this world",
		confirmSubtitle: "This may be a walk-in or contested — the CLI can't tell if a defending army is present.",
		followUp:        "track this fight with `battles` — outcome surfaces when the march arrives",
		targets:         targets,
	})
}

// runRuinClaim implements `ruin claim [<region>]`.
func runRuinClaim(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	worldID, kingdomID, err := requireWorldAndKingdom(ctx, sess)
	if err != nil {
		return err
	}
	targets, err := claimableTargets(ctx, sess, worldID)
	if err != nil {
		return err
	}
	return runExpedition(ctx, sess, args, expeditionFlow{
		worldID:         worldID,
		kingdomID:       kingdomID,
		name:            "ruin claim",
		intent:          "claim_ruin",
		pickTitle:       "Pick an unclaimed ruin",
		emptyError:      "no unclaimed ruins in this world",
		confirmSubtitle: "Ruin garrisons defend with the same combat resolver as wilderness nodes.",
		followUp:        "track this fight with `battles` — cache lands in your home stockpile on success",
		targets:         targets,
	})
}

// expeditionFlow bundles the per-verb knobs runExpedition needs.
type expeditionFlow struct {
	worldID, kingdomID string
	name               string // user-facing verb name (for "<name> preview" header + errors)
	intent             string // wire intent for dispatchMarch
	pickTitle          string
	emptyError         string
	confirmSubtitle    string
	followUp           string
	targets            []expeditionTarget
}

// runExpedition is the shared six-step wizard skeleton. Each Phase 10
// handler differs only in its target predicate and preview wording —
// every other beat (army pick, confirm, dispatch, render) is shared.
func runExpedition(ctx context.Context, sess *shell.Session, args []string, f expeditionFlow) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: %s [<region>]", f.name)
	}
	if len(f.targets) == 0 {
		return errors.New(f.emptyError)
	}

	target, err := pickTarget(ctx, args, f)
	if err != nil {
		return err
	}

	armies, err := sess.API.ListKingdomArmies(ctx, f.kingdomID)
	if err != nil {
		return err
	}
	homeArmies := filterHomeArmies(armies)
	if len(homeArmies) == 0 {
		return fmt.Errorf("no home armies available — train units or recall a marching army first")
	}
	army, err := pickArmy(ctx, sess, homeArmies)
	if err != nil {
		return err
	}

	printExpeditionPreview(sess, f.name, f.intent, target, army)
	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Dispatch %s march to %s with %s?", f.intent, target.RegionName, army.Name),
		f.confirmSubtitle)
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

	march, err := sess.API.DispatchMarch(ctx, army.ID, target.RegionID, f.intent)
	if err != nil {
		return err
	}
	shared.PrintMarchOrder(sess, march, shared.PathNames(ctx, sess, march.Path))
	shell.Info(sess.Out, f.followUp)
	return nil
}

// pickTarget resolves the optional <region> arg against the eligible
// list, or opens a selector when no arg was passed.
func pickTarget(ctx context.Context, args []string, f expeditionFlow) (expeditionTarget, error) {
	if len(args) == 1 {
		want := strings.ToLower(strings.TrimSpace(args[0]))
		for _, t := range f.targets {
			if strings.ToLower(t.RegionName) == want {
				return t, nil
			}
		}
		names := make([]string, 0, len(f.targets))
		for _, t := range f.targets {
			names = append(names, t.RegionName)
		}
		return expeditionTarget{}, fmt.Errorf("region %q not eligible for %s (try: %s)",
			args[0], f.name, strings.Join(names, ", "))
	}
	items := make([]selector.Item, 0, len(f.targets))
	for _, t := range f.targets {
		items = append(items, selector.Item{
			Title:       t.RegionName,
			Description: t.description,
			Value:       t.RegionID,
		})
	}
	picked, err := selector.Pick(ctx, f.pickTitle, items)
	if err != nil {
		return expeditionTarget{}, err
	}
	for _, t := range f.targets {
		if t.RegionID == picked.Value {
			return t, nil
		}
	}
	return expeditionTarget{}, fmt.Errorf("picker returned unknown region %q", picked.Value)
}

// pickArmy chooses a home army. Auto-selects when only one is
// available (with a one-line info note so the user sees the choice).
func pickArmy(ctx context.Context, sess *shell.Session, armies []gen.Army) (gen.Army, error) {
	if len(armies) == 1 {
		shell.Info(sess.Out, "army: "+armies[0].Name+" (only home army)")
		return armies[0], nil
	}
	items := make([]selector.Item, 0, len(armies))
	for _, a := range armies {
		items = append(items, selector.Item{
			Title:       a.Name,
			Description: fmt.Sprintf("cap=%d  %s", a.TotalCapacity, formatComposition(map[string]int(a.Composition))),
			Value:       a.ID,
		})
	}
	picked, err := selector.Pick(ctx, "Pick a home army", items)
	if err != nil {
		return gen.Army{}, err
	}
	for _, a := range armies {
		if a.ID == picked.Value {
			return a, nil
		}
	}
	return gen.Army{}, fmt.Errorf("picker returned unknown army %q", picked.Value)
}

// filterHomeArmies keeps only armies in `home` status — the only
// status from which a fresh march can dispatch.
func filterHomeArmies(armies []gen.Army) []gen.Army {
	out := make([]gen.Army, 0, len(armies))
	for _, a := range armies {
		if a.Status == gen.ArmyStatusHome {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// requireWorldAndKingdom is a tiny shim — Phase 10 verbs always need
// both the world ID (for listNodes / listRuins) and the kingdom ID
// (for listKingdomArmies / "owned by other" filtering).
func requireWorldAndKingdom(ctx context.Context, sess *shell.Session) (worldID, kingdomID string, err error) {
	kingdomID, err = shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return "", "", err
	}
	worldID, err = shared.RequireWorldID(ctx, sess)
	if err != nil {
		return "", "", err
	}
	return worldID, kingdomID, nil
}

// ── target predicates ────────────────────────────────────────────────

// capturableTargets returns one entry per region that contains at
// least one wilderness node (no owner, not a home-hoard).
func capturableTargets(ctx context.Context, sess *shell.Session, worldID string) ([]expeditionTarget, error) {
	nodes, err := sess.API.ListNodes(ctx, worldID)
	if err != nil {
		return nil, err
	}
	byRegion := make(map[string][]gen.Node)
	for _, n := range nodes {
		_, owned := n.OwnerKingdomID.Get()
		if n.IsHomeHoard || owned {
			continue
		}
		regionID, _ := n.RegionID.Get()
		if regionID == "" {
			continue
		}
		byRegion[regionID] = append(byRegion[regionID], n)
	}
	return projectTargets(byRegion, func(group []gen.Node) (string, []string) {
		desc := describeNodeGroup(group, "")
		return desc, previewLinesForNodes(group)
	}), nil
}

// attackableTargets returns one entry per region that contains at
// least one node owned by another kingdom (not us, not home-hoard).
func attackableTargets(ctx context.Context, sess *shell.Session, worldID, myKingdomID string) ([]expeditionTarget, error) {
	nodes, err := sess.API.ListNodes(ctx, worldID)
	if err != nil {
		return nil, err
	}
	byRegion := make(map[string][]gen.Node)
	for _, n := range nodes {
		v, owned := n.OwnerKingdomID.Get()
		if !owned || n.IsHomeHoard || v == myKingdomID {
			continue
		}
		regionID, _ := n.RegionID.Get()
		if regionID == "" {
			continue
		}
		byRegion[regionID] = append(byRegion[regionID], n)
	}
	return projectTargets(byRegion, func(group []gen.Node) (string, []string) {
		return describeNodeGroup(group, nodeOwner(group[0], "")), previewLinesForNodes(group)
	}), nil
}

// claimableTargets returns one entry per unclaimed ruin.
func claimableTargets(ctx context.Context, sess *shell.Session, worldID string) ([]expeditionTarget, error) {
	ruins, err := sess.API.ListRuins(ctx, worldID)
	if err != nil {
		return nil, err
	}
	out := make([]expeditionTarget, 0, len(ruins))
	for _, r := range ruins {
		if r.Claimed {
			continue
		}
		out = append(out, expeditionTarget{
			RegionID:     r.RegionID,
			RegionName:   r.RegionName,
			description:  fmt.Sprintf("tier=%s  cache=%s", string(r.Tier), formatComposition(map[string]int(r.Cache))),
			previewLines: previewLinesForRuin(r),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RegionName < out[j].RegionName })
	return out, nil
}

// projectTargets converts a regionID → []Node grouping into the
// sorted, deduplicated target slice the selector consumes. The
// describe callback supplies the picker description string + the
// preview-block body for that region.
func projectTargets(byRegion map[string][]gen.Node, describe func([]gen.Node) (string, []string)) []expeditionTarget {
	out := make([]expeditionTarget, 0, len(byRegion))
	for regionID, group := range byRegion {
		name := ""
		if v, ok := group[0].RegionName.Get(); ok {
			name = v
		}
		if name == "" {
			name = regionID
		}
		desc, lines := describe(group)
		out = append(out, expeditionTarget{
			RegionID:     regionID,
			RegionName:   name,
			description:  desc,
			previewLines: lines,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RegionName < out[j].RegionName })
	return out
}

// ── per-flow render helpers ──────────────────────────────────────────

// describeNodeGroup builds the picker secondary-line description. When
// owner is non-empty (attack flow) it's prepended.
func describeNodeGroup(group []gen.Node, owner string) string {
	parts := make([]string, 0, len(group))
	for _, n := range group {
		parts = append(parts, fmt.Sprintf("%s/%s", string(n.Resource), string(n.Tier)))
	}
	body := strings.Join(parts, ", ")
	if owner != "" {
		return fmt.Sprintf("owner=%s  nodes=%s", owner, body)
	}
	return "nodes=" + body
}

// previewLinesForNodes builds the per-region "nodes:" preview body
// for both capture and attack flows.
func previewLinesForNodes(group []gen.Node) []string {
	lines := make([]string, 0, len(group))
	for _, n := range group {
		owner := nodeOwner(n, "(wild)")
		garrison := ""
		if g, ok := n.Garrison.Get(); ok && len(g) > 0 {
			garrison = "  garrison=" + formatComposition(map[string]int(g))
		}
		lines = append(lines, fmt.Sprintf("    %s/%s  owner=%s%s",
			string(n.Resource), string(n.Tier), owner, garrison))
	}
	return lines
}

// previewLinesForRuin builds the preview body for the ruin flow.
func previewLinesForRuin(r gen.Ruin) []string {
	lines := []string{
		"    tier:       " + string(r.Tier),
		"    garrison:   " + formatComposition(map[string]int(r.Garrison)),
		"    cache:      " + formatComposition(map[string]int(r.Cache)),
		"    warning:    cache is granted to your home stockpile on success — anything over your Warehouse cap is lost",
	}
	return lines
}

// printExpeditionPreview renders the "<verb> preview" block before
// the confirm prompt. Format mirrors `build preview` /
// `train preview` (line-per-field, narrow).
func printExpeditionPreview(sess *shell.Session, name, intent string, t expeditionTarget, army gen.Army) {
	shell.Strong(sess.Out, name+" preview")
	_, _ = fmt.Fprintf(sess.Out, "  target:    %s\n", t.RegionName)
	if len(t.previewLines) > 0 {
		_, _ = fmt.Fprintln(sess.Out, "  detail:")
		for _, line := range t.previewLines {
			_, _ = fmt.Fprintln(sess.Out, line)
		}
	}
	_, _ = fmt.Fprintf(sess.Out, "  army:      %s (cap=%d, %s)\n",
		army.Name, army.TotalCapacity, formatComposition(map[string]int(army.Composition)))
	_, _ = fmt.Fprintf(sess.Out, "  intent:    %s\n", intent)
}

// ── tab completion ───────────────────────────────────────────────────

func suggestCapturableRegions(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	targets, err := capturableTargets(ctx, sess, worldID)
	if err != nil {
		return nil, nil
	}
	return targetNames(targets), nil
}

func suggestAttackableRegions(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	worldID, kingdomID, err := requireWorldAndKingdom(ctx, sess)
	if err != nil {
		return nil, nil
	}
	targets, err := attackableTargets(ctx, sess, worldID, kingdomID)
	if err != nil {
		return nil, nil
	}
	return targetNames(targets), nil
}

func suggestClaimableRegions(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	targets, err := claimableTargets(ctx, sess, worldID)
	if err != nil {
		return nil, nil
	}
	return targetNames(targets), nil
}

func targetNames(targets []expeditionTarget) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.RegionName)
	}
	return out
}
