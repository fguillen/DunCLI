// Package trade hosts the Phase 11 trade verbs: `caravan send` (split
// an escort off a home army and dispatch a payload to another player)
// and `trade ledger` (paginated world-scoped history of past
// transfers). Interception combat lands in the Phase 9 battle stream
// when the march arrives; this package never resolves combat itself.
package trade

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// resourceKinds is the canonical ordering used for payload fields and
// the preview block. Pinned to the spec — keys outside this set are
// rejected by the backend with `invalid_payload`.
var resourceKinds = []string{"gold", "wood", "stone", "iron"}

func init() {
	shell.Register(&shell.Verb{
		Name:    "caravan",
		Summary: "Dispatch a trade caravan to another player",
		Usage:   "caravan send <receiver-handle>",
		Sub: map[string]*shell.Verb{
			"send": {
				Name:    "send",
				Summary: "Send a caravan with payload + escort to another player",
				Usage:   "caravan send <receiver-handle>",
				Run:     runCaravanSend,
			},
		},
	})
}

// runCaravanSend implements `caravan send <receiver-handle>`. The
// receiver handle is required; the source army, payload, and escort
// units are gathered via a single picker + form. The backend remains
// authoritative on capacity, stockpile, and reachability — this
// handler only enforces non-negative ints and that the payload and
// escort each carry at least one positive entry.
func runCaravanSend(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: caravan send <receiver-handle>")
	}
	receiver := strings.TrimSpace(args[0])
	if receiver == "" {
		return errors.New("receiver handle is required")
	}

	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}

	armies, err := sess.API.ListKingdomArmies(ctx, kingdomID)
	if err != nil {
		return err
	}
	home := filterHomeArmies(armies)
	if len(home) == 0 {
		return errors.New("no home armies to escort a caravan")
	}
	src, err := pickSourceArmy(ctx, sess, home)
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}

	payload, escort, err := promptPayloadAndEscort(ctx, src)
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return nil
		}
		return err
	}

	regionName := shared.LookupRegionName(ctx, sess, src.LocationRegionID)
	printCaravanPreview(sess, receiver, src, regionName, payload, escort)

	ok, err := selector.Confirm(ctx,
		fmt.Sprintf("Dispatch caravan to %s with %s?", receiver, src.Name),
		"Caravans can be intercepted en route. Anything over the receiver's Warehouse cap on arrival is lost.")
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

	car, err := sess.API.DispatchCaravan(ctx, kingdomID, receiver, src.ID, payload, escort)
	if err != nil {
		return err
	}
	printCaravanOrder(sess, car, shared.RegionNameMap(ctx, sess))
	shell.Info(sess.Out, "track delivery via `trade ledger`; interceptions also appear in `battles`")
	return nil
}

// pickSourceArmy auto-selects when only one home army is available
// (printing a one-line info note so the user sees the choice) and
// opens the picker otherwise.
func pickSourceArmy(ctx context.Context, sess *shell.Session, armies []gen.Army) (gen.Army, error) {
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
	picked, err := selector.Pick(ctx, "Pick a source army", items)
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

// promptPayloadAndEscort opens a single form with four payload inputs
// (gold, wood, stone, iron) plus one input per unit kind present in
// the source army's composition. Re-prompts if all-zero on either
// side.
func promptPayloadAndEscort(ctx context.Context, src gen.Army) (payload, escort map[string]int, err error) {
	fields := make([]selector.Field, 0, len(resourceKinds)+len(src.Composition))
	for _, r := range resourceKinds {
		fields = append(fields, selector.Field{
			Key:      r,
			Label:    r + " to send",
			Initial:  "0",
			Validate: nonNegativeIntValidator(),
		})
	}
	unitKeys := sortedCompositionKeys(src.Composition)
	for _, k := range unitKeys {
		maxN := src.Composition[k]
		if maxN <= 0 {
			continue
		}
		kCopy := k
		maxCopy := maxN
		fields = append(fields, selector.Field{
			Key:     k,
			Label:   fmt.Sprintf("escort %s (max %d)", kCopy, maxCopy),
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

	res, err := selector.Form(ctx, "Payload + escort for caravan", fields)
	if err != nil {
		return nil, nil, err
	}

	payload = make(map[string]int, len(resourceKinds))
	for _, r := range resourceKinds {
		n, _ := strconv.Atoi(strings.TrimSpace(res[r]))
		if n > 0 {
			payload[r] = n
		}
	}
	escort = make(map[string]int, len(unitKeys))
	for _, k := range unitKeys {
		n, _ := strconv.Atoi(strings.TrimSpace(res[k]))
		if n > 0 {
			escort[k] = n
		}
	}

	if len(payload) == 0 {
		return nil, nil, errors.New("payload must include at least one positive resource")
	}
	if len(escort) == 0 {
		return nil, nil, errors.New("escort must include at least one unit")
	}
	return payload, escort, nil
}

func nonNegativeIntValidator() func(string) error {
	return func(s string) error {
		n, e := strconv.Atoi(strings.TrimSpace(s))
		if e != nil || n < 0 {
			return errors.New("must be a non-negative integer")
		}
		return nil
	}
}

// printCaravanPreview renders the block shown before the confirm. The
// shape mirrors Phase 10's expedition preview.
func printCaravanPreview(sess *shell.Session, receiver string, src gen.Army, regionName string, payload, escort map[string]int) {
	shell.Strong(sess.Out, "caravan dispatch preview")
	_, _ = fmt.Fprintf(sess.Out, "  target:    %s\n", receiver)
	_, _ = fmt.Fprintf(sess.Out, "  payload:   %s\n", formatPayload(payload))
	_, _ = fmt.Fprintf(sess.Out, "  escort:    %s\n", formatComposition(escort))
	if regionName == "" {
		regionName = src.LocationRegionID
	}
	_, _ = fmt.Fprintf(sess.Out, "  from:      %s at %s\n", src.Name, regionName)
}

// printCaravanOrder renders the success block after dispatch — same
// shape as a march order, with the receiver's home region named via
// the in-scope world map cache when possible.
func printCaravanOrder(sess *shell.Session, c *gen.Caravan, regionNameByID map[string]string) {
	shell.Strong(sess.Out, fmt.Sprintf("caravan %s  (in_transit)", c.ID))
	origin := regionName(c.OriginRegionID, regionNameByID)
	dest := regionName(c.DestinationRegionID, regionNameByID)
	_, _ = fmt.Fprintf(sess.Out, "  arrives:   %s  (ETA %s)\n",
		c.ArrivesAt.Format("2006-01-02 15:04 MST"), shared.RelTime(c.ArrivesAt))
	_, _ = fmt.Fprintf(sess.Out, "  path:      %s → %s\n", origin, dest)
	_, _ = fmt.Fprintf(sess.Out, "  payload:   %s\n", formatPayload(map[string]int(c.Payload)))
	_, _ = fmt.Fprintf(sess.Out, "  escort:    %s\n", formatComposition(map[string]int(c.EscortUnits)))
	if id, ok := c.OutboundMarchOrderID.Get(); ok && id != "" {
		_, _ = fmt.Fprintf(sess.Out, "  march:     %s\n", id)
	}
}

// formatPayload renders a resource map in `gold|wood|stone|iron`
// order, zero/missing keys stripped. Empty maps print as "(none)".
func formatPayload(m map[string]int) string {
	parts := make([]string, 0, len(resourceKinds))
	for _, r := range resourceKinds {
		if v, ok := m[r]; ok && v > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", r, v))
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " ")
}

// formatComposition renders a composition / escort map alphabetically
// with zero-valued keys stripped. Empty maps print as "(empty)".
func formatComposition(m map[string]int) string {
	if len(m) == 0 {
		return "(empty)"
	}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "(empty)"
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// sortedCompositionKeys returns the source army's unit kinds in a
// stable order (alphabetical). Zero-valued keys are dropped.
func sortedCompositionKeys(c gen.Composition) []string {
	out := make([]string, 0, len(c))
	for k, v := range c {
		if v > 0 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// filterHomeArmies keeps only `home`-status armies; sorted by name for
// stable picker order.
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

// regionName looks up a region ID → name with a fallback to the raw
// ID. Mirror of the helper in the battles package.
func regionName(id string, byID map[string]string) string {
	if name := byID[id]; name != "" {
		return name
	}
	return id
}
