package verbs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "map",
		Summary: "List every region on the in-scope world",
		Usage:   "map",
		Run:     runMap,
	})

	shell.Register(&shell.Verb{
		Name:    "region",
		Summary: "Inspect a region",
		Usage:   "region show <name>",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:     "show",
				Summary:  "Show region detail (terrain, owner, nodes, ruin, adjacency)",
				Usage:    "region show <name>",
				Run:      runRegionShow,
				Complete: shell.SuggestFunc(suggestRegionName),
			},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "ruins",
		Summary: "List ruins on the in-scope world",
		Usage:   "ruins",
		Run:     runRuinsList,
	})

	shell.Register(&shell.Verb{
		Name:    "nodes",
		Summary: "List nodes on the in-scope world",
		Usage:   "nodes [--owner mine|wild|captured|home-hoard]",
		Run:     runNodesList,
		Flags: []shell.FlagSpec{
			{
				Name:     "owner",
				HasValue: true,
				Help:     "filter: mine, wild, captured, home-hoard",
				Suggest: shell.SuggestFunc(func(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
					return []string{"mine", "wild", "captured", "home-hoard"}, nil
				}),
			},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "node",
		Summary: "Inspect or act on a node",
		Usage:   "node <show|capture|attack> ...",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:     "show",
				Summary:  "Show one node by ULID or by the region name it sits in",
				Usage:    "node show <id-or-region>",
				Run:      runNodeShow,
				Complete: shell.SuggestFunc(suggestRegionName),
			},
			"capture": {
				Name:     "capture",
				Summary:  "Dispatch a capture march against a wilderness node",
				Usage:    "node capture [<region>]",
				Run:      runNodeCapture,
				Complete: shell.SuggestFunc(suggestCapturableRegions),
			},
			"attack": {
				Name:     "attack",
				Summary:  "Dispatch a capture march against a foreign-owned node",
				Usage:    "node attack [<region>]",
				Run:      runNodeAttack,
				Complete: shell.SuggestFunc(suggestAttackableRegions),
			},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "ruin",
		Summary: "Act on a ruin",
		Usage:   "ruin claim [<region>]",
		Sub: map[string]*shell.Verb{
			"claim": {
				Name:     "claim",
				Summary:  "Dispatch a claim_ruin march against an unclaimed ruin",
				Usage:    "ruin claim [<region>]",
				Run:      runRuinClaim,
				Complete: shell.SuggestFunc(suggestClaimableRegions),
			},
		},
	})
}

// terrainGlyph maps a terrain enum value to a one-character glyph for
// `map` rendering. Falls back to "?" for an unknown enum (e.g. a new
// terrain the spec gained that the CLI hasn't been regenerated for).
func terrainGlyph(t string) string {
	switch strings.ToLower(t) {
	case "plains":
		return "."
	case "forest":
		return "T"
	case "hills":
		return "^"
	case "mountain":
		return "M"
	case "marsh":
		return "~"
	}
	return "?"
}

// runMap implements `map` — prints one styled line per region. The
// adjacency list is rendered by name (resolved via the same map
// response, since RegionSummary.adjacency is a []string of region
// IDs).
func runMap(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	regions, err := sess.API.ShowWorldMap(ctx, worldID)
	if err != nil {
		return err
	}
	if len(regions) == 0 {
		shell.Section(sess.Out, "Map:", "")
		return nil
	}

	byID := make(map[string]string, len(regions))
	for _, r := range regions {
		byID[r.ID] = r.Name
	}

	lines := make([]string, 0, len(regions))
	for _, r := range regions {
		adj := make([]string, 0, len(r.Adjacency))
		for _, id := range r.Adjacency {
			if name, ok := byID[id]; ok {
				adj = append(adj, name)
			} else {
				adj = append(adj, id)
			}
		}
		lines = append(lines, fmt.Sprintf("%s  %-16s  nodes=%d  adj=%s",
			terrainGlyph(string(r.Terrain)),
			r.Name,
			len(r.Nodes),
			strings.Join(adj, ", "),
		))
	}
	sort.Strings(lines)
	shell.Section(sess.Out, "Map:", strings.Join(lines, "\n"))
	return nil
}

// runRegionShow implements `region show <name>`. Fires showRegion +
// showRegionAdjacent sequentially (small calls; saving the cost of
// errgroup keeps the code straightforward).
func runRegionShow(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: region show <name>")
	}
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	regionID, err := sess.API.ResolveRegion(ctx, worldID, args[0])
	if err != nil {
		return err
	}
	region, err := sess.API.ShowRegion(ctx, worldID, regionID)
	if err != nil {
		return err
	}
	adj, err := sess.API.ShowRegionAdjacent(ctx, worldID, regionID)
	if err != nil {
		return err
	}
	printRegion(sess, region, adj)
	return nil
}

// runRuinsList implements `ruins`.
func runRuinsList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	ruins, err := sess.API.ListRuins(ctx, worldID)
	if err != nil {
		return err
	}
	if len(ruins) == 0 {
		shell.Section(sess.Out, "Ruins:", "")
		return nil
	}
	var lines []string
	for _, r := range ruins {
		state := "unclaimed"
		if r.Claimed {
			state = "claimed"
		}
		lines = append(lines, fmt.Sprintf("%-16s  tier=%-8s  %s  garrison=%s",
			r.RegionName, string(r.Tier), state, formatComposition(map[string]int(r.Garrison))))
	}
	shell.Section(sess.Out, "Ruins:", strings.Join(lines, "\n"))
	return nil
}

// runNodesList implements `nodes [--owner ...]`. Filtering is
// client-side because the spec does not declare a server-side filter
// for ListNodes.
func runNodesList(ctx context.Context, sess *shell.Session, _ []string, flags map[string]string) error {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}
	owner, hasOwner := flags["owner"]
	if hasOwner {
		switch owner {
		case "mine", "wild", "captured", "home-hoard":
		default:
			return fmt.Errorf("invalid --owner %q (want mine|wild|captured|home-hoard)", owner)
		}
	}

	nodes, err := sess.API.ListNodes(ctx, worldID)
	if err != nil {
		return err
	}

	// Caller's kingdom ID — only needed if `mine` / `captured` filter
	// is requested. Don't burn an extra HTTP call when the user just
	// types `nodes`.
	var myKingdomID string
	if hasOwner && (owner == "mine" || owner == "captured") {
		myKingdomID, err = sess.API.ResolveKingdom(ctx, worldID)
		if err != nil {
			return err
		}
	}

	filtered := nodes[:0]
	for _, n := range nodes {
		if !matchOwner(n, owner, myKingdomID) {
			continue
		}
		filtered = append(filtered, n)
	}

	if len(filtered) == 0 {
		shell.Section(sess.Out, "Nodes:", "")
		return nil
	}
	var lines []string
	for _, n := range filtered {
		lines = append(lines, formatNodeRow(n))
	}
	shell.Section(sess.Out, "Nodes:", strings.Join(lines, "\n"))
	return nil
}

// runNodeShow implements `node show <id-or-region>`. The arg is tried
// first as a region name (the common case — region names are unique
// and rendered everywhere). If no region matches, it falls back to
// treating the arg as a node ULID.
func runNodeShow(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: node show <id-or-region>")
	}
	arg := args[0]
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return err
	}

	// First try as a region name. If ResolveRegion 404s, fall back to
	// treating the arg as a node ULID.
	regionID, rerr := sess.API.ResolveRegion(ctx, worldID, arg)
	if rerr == nil {
		region, err := sess.API.ShowRegion(ctx, worldID, regionID)
		if err != nil {
			return err
		}
		switch len(region.Nodes) {
		case 0:
			return fmt.Errorf("region %q has no nodes", arg)
		case 1:
			printNode(sess, &region.Nodes[0])
			return nil
		default:
			var ids []string
			for _, n := range region.Nodes {
				ids = append(ids, n.ID)
			}
			return fmt.Errorf("region %q has %d nodes — pick by id: %s",
				arg, len(region.Nodes), strings.Join(ids, ", "))
		}
	}
	if apiErr := api.AsError(rerr); apiErr == nil || apiErr.Code != "not_found" {
		return rerr
	}
	node, err := sess.API.ShowNode(ctx, worldID, arg)
	if err != nil {
		return err
	}
	printNode(sess, node)
	return nil
}

// matchOwner returns true when the node passes the --owner filter
// (empty filter ⇒ keep all).
func matchOwner(n gen.Node, owner, myKingdomID string) bool {
	switch owner {
	case "":
		return true
	case "home-hoard":
		return n.IsHomeHoard
	case "wild":
		_, owned := n.OwnerKingdomID.Get()
		return !n.IsHomeHoard && !owned
	case "mine":
		ok, _ := ownerMatches(n, myKingdomID)
		return ok && !n.IsHomeHoard
	case "captured":
		ok, present := ownerMatches(n, myKingdomID)
		// captured ≡ has a non-caller owner.
		return present && !ok
	}
	return false
}

// ownerMatches reports whether n's owner_kingdom_id equals the given
// kingdomID; present is true iff the field has any non-null value.
func ownerMatches(n gen.Node, kingdomID string) (matches, present bool) {
	v, ok := n.OwnerKingdomID.Get()
	if !ok {
		return false, false
	}
	return v == kingdomID, true
}

// formatComposition pretty-prints a unit-count map (used for ruin
// garrisons and node garrisons) as `levy=12, archer=4` with stable
// key order.
func formatComposition(m map[string]int) string {
	if len(m) == 0 {
		return "(empty)"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func formatNodeRow(n gen.Node) string {
	owner := "wild"
	if v, ok := n.OwnerKingdomID.Get(); ok {
		owner = v
	}
	if n.IsHomeHoard {
		owner = "home-hoard"
	}
	region := "(unknown region)"
	if v, ok := n.RegionName.Get(); ok {
		region = v
	}
	var garrison string
	if g, ok := n.Garrison.Get(); ok && len(g) > 0 {
		garrison = "  garrison=" + formatComposition(map[string]int(g))
	}
	return fmt.Sprintf("%-16s  %-6s  %-8s  owner=%s%s",
		region, string(n.Resource), string(n.Tier), owner, garrison)
}

// printRegion renders region detail + an `Adjacent:` line from the
// separate showRegionAdjacent call.
func printRegion(sess *shell.Session, r *gen.Region, adj []gen.ShowRegionAdjacentOKRegionsItem) {
	shell.Strong(sess.Out, fmt.Sprintf("%s  (%s)", r.Name, terrainGlyph(string(r.Terrain))+" "+string(r.Terrain)))
	_, _ = fmt.Fprintf(sess.Out, "  position:  x=%.2f y=%.2f\n", r.Position.X, r.Position.Y)
	if owner, ok := r.OwnerKingdomID.Get(); ok {
		_, _ = fmt.Fprintln(sess.Out, "  owner:     "+owner)
	} else {
		_, _ = fmt.Fprintln(sess.Out, "  owner:     (wild)")
	}
	if hub, ok := r.IsHub.Get(); ok && hub {
		_, _ = fmt.Fprintln(sess.Out, "  hub:       yes")
	}
	if len(r.Nodes) > 0 {
		var lines []string
		for _, n := range r.Nodes {
			lines = append(lines, "    "+formatNodeRow(n))
		}
		_, _ = fmt.Fprintln(sess.Out, "  nodes:")
		for _, l := range lines {
			_, _ = fmt.Fprintln(sess.Out, l)
		}
	}
	if ruin, ok := r.Ruin.Get(); ok {
		state := "unclaimed"
		if ruin.Claimed {
			state = "claimed"
		}
		_, _ = fmt.Fprintf(sess.Out, "  ruin:      tier=%s  %s\n", string(ruin.Tier), state)
	}
	if len(adj) > 0 {
		names := make([]string, 0, len(adj))
		for _, a := range adj {
			names = append(names, a.Name)
		}
		_, _ = fmt.Fprintln(sess.Out, "  adjacent:  "+strings.Join(names, ", "))
	}
}

// printNode renders one node to scrollback.
func printNode(sess *shell.Session, n *gen.Node) {
	shell.Strong(sess.Out, fmt.Sprintf("Node %s  (%s, %s)", n.ID, string(n.Resource), string(n.Tier)))
	if v, ok := n.RegionName.Get(); ok {
		_, _ = fmt.Fprintln(sess.Out, "  region:    "+v)
	}
	owner := "(wild)"
	if v, ok := n.OwnerKingdomID.Get(); ok {
		owner = v
	}
	if n.IsHomeHoard {
		owner = "(home-hoard)"
	}
	_, _ = fmt.Fprintln(sess.Out, "  owner:     "+owner)
	if br, ok := n.BaseRate.Get(); ok {
		_, _ = fmt.Fprintf(sess.Out, "  base rate: %d/hr\n", br)
	}
	if g, ok := n.Garrison.Get(); ok && len(g) > 0 {
		_, _ = fmt.Fprintln(sess.Out, "  garrison:  "+formatComposition(map[string]int(g)))
	}
}

// suggestRegionName is the dynamic Suggester for region-name args
// (`region show <Tab>`, `node show <Tab>`).
func suggestRegionName(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	worldID, err := shared.RequireWorldID(ctx, sess)
	if err != nil {
		return nil, nil
	}
	regions, err := sess.API.ShowWorldMap(ctx, worldID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(regions))
	for _, r := range regions {
		out = append(out, r.Name)
	}
	return out, nil
}
