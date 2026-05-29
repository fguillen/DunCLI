package battles

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// resourceOrder is the canonical ordering for loot / composition keys
// that match a resource kind. Other keys (unit kinds) fall through to
// alphabetical.
var resourceOrder = []string{"gold", "wood", "stone", "iron"}

// lootString renders a Battle.Loot map in canonical resource order,
// skipping zero/missing keys. Empty maps come out as "(none)".
func lootString(loot gen.BattleLoot) string {
	parts := make([]string, 0, len(loot))
	for _, k := range resourceOrder {
		if v, ok := loot[k]; ok && v > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", k, v))
		}
	}
	// Surface any non-resource keys the backend might add (forward-
	// compat: unknown keys land here alphabetically).
	extra := make([]string, 0)
	for k, v := range loot {
		if isResourceKey(k) {
			continue
		}
		if v > 0 {
			extra = append(extra, fmt.Sprintf("%s=%d", k, v))
		}
	}
	sort.Strings(extra)
	parts = append(parts, extra...)
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " ")
}

func isResourceKey(k string) bool {
	for _, r := range resourceOrder {
		if r == k {
			return true
		}
	}
	return false
}

// compositionString renders a Composition map alphabetically with
// zero-valued keys stripped. Empty maps come out as "(none)".
func compositionString(c gen.Composition) string {
	keys := make([]string, 0, len(c))
	for k, v := range c {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "(none)"
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, c[k]))
	}
	return strings.Join(parts, ", ")
}

// shortKingdom shortens a kingdom_id for the table column. For real
// ULIDs (26 chars) it returns first4…last4; short test IDs are kept
// verbatim. Empty input means "wilderness".
func shortKingdom(id string) string {
	if id == "" {
		return "(wilderness)"
	}
	if len(id) <= 8 {
		return id
	}
	return id[:4] + "…" + id[len(id)-4:]
}

// regionLabel resolves a region_id to a name via the in-scope world
// map; falls back to the ID on miss.
func regionLabel(regionID string, regionNameByID map[string]string) string {
	if name := regionNameByID[regionID]; name != "" {
		return name
	}
	return regionID
}

// opponentLabel renders the "other side" of a battle relative to the
// caller. Wilderness battles (null/absent defender_kingdom_id) collapse
// to the wilderness marker. When the caller defended, the opponent is
// the attacker; otherwise it's the defender.
func opponentLabel(b gen.Battle, callerKingdomID string) string {
	defID, ok := b.DefenderKingdomID.Get()
	if !ok || defID == "" {
		return "(wilderness)"
	}
	if defID == callerKingdomID {
		return shortKingdom(b.AttackerKingdomID)
	}
	return shortKingdom(defID)
}

// printBattleList renders the `battles` table.
func printBattleList(sess *shell.Session, list []gen.Battle, total, limit, offset int, callerKingdomID string, regionNameByID map[string]string) {
	if len(list) == 0 {
		shell.Section(sess.Out, "Battles:", "")
		return
	}
	header := fmt.Sprintf("Battles (showing %d-%d of %d):",
		offset+1, offset+len(list), total)
	lines := make([]string, 0, len(list))
	for _, b := range list {
		lines = append(lines, fmt.Sprintf("%-12s %s  %-14s  vs %-14s  %-18s  loot:%s",
			b.ID,
			b.EndedAt.UTC().Format("2006-01-02 15:04 UTC"),
			regionLabel(b.RegionID, regionNameByID),
			opponentLabel(b, callerKingdomID),
			string(b.Outcome),
			lootString(b.Loot)))
	}
	shell.Section(sess.Out, header, strings.Join(lines, "\n"))
	// Hint when there are more pages beyond the window.
	if shown := offset + len(list); shown < total {
		shell.Info(sess.Out, fmt.Sprintf("more: %d remaining — `battles --limit %d --offset %d`",
			total-shown, limit, shown))
	}
}

// printBattleDetail renders `battle show <id>`.
func printBattleDetail(sess *shell.Session, b *gen.Battle, parts []gen.BattleParticipant, callerKingdomID string, regionNameByID map[string]string) {
	shell.Strong(sess.Out, fmt.Sprintf("Battle %s  (%s)", b.ID, regionLabel(b.RegionID, regionNameByID)))
	_, _ = fmt.Fprintf(sess.Out, "  when:       %s → %s\n",
		b.StartedAt.UTC().Format("2006-01-02 15:04 UTC"),
		b.EndedAt.UTC().Format("2006-01-02 15:04 UTC"))
	_, _ = fmt.Fprintf(sess.Out, "  outcome:    %s\n", string(b.Outcome))

	attackerTitle := b.AttackerTitle.Or("—")
	if attackerTitle == "" {
		attackerTitle = "—"
	}
	defenderTitle := b.DefenderTitle.Or("—")
	if defenderTitle == "" {
		defenderTitle = "—"
	}
	_, _ = fmt.Fprintf(sess.Out, "  titles:     attacker=%s  defender=%s\n", attackerTitle, defenderTitle)

	if v, ok := b.MarchOrderID.Get(); ok && v != "" {
		_, _ = fmt.Fprintf(sess.Out, "  march:      %s\n", v)
	} else {
		_, _ = fmt.Fprintln(sess.Out, "  march:      —")
	}
	_, _ = fmt.Fprintf(sess.Out, "  loot:       %s\n", lootString(b.Loot))

	// Participants
	participantLines := make([]string, 0, len(parts)*4)
	ordered := orderParticipants(parts)
	for _, p := range ordered {
		kid, _ := p.KingdomID.Get()
		who := shortKingdom(kid)
		if kid == callerKingdomID && callerKingdomID != "" {
			who = who + " (you)"
		}
		armyID := "—"
		if v, ok := p.ArmyID.Get(); ok && v != "" {
			armyID = v
		}
		participantLines = append(participantLines,
			fmt.Sprintf("%-9s %-22s  army=%s", string(p.Side), who, armyID))
		participantLines = append(participantLines, "  starting:   "+compositionString(p.StartingComposition))
		participantLines = append(participantLines, "  ending:     "+compositionString(p.EndingComposition))
		participantLines = append(participantLines, "  casualties: "+compositionString(p.Casualties))
	}
	shell.Section(sess.Out, "Participants:", strings.Join(participantLines, "\n"))

	// Rounds
	if len(b.Log) == 0 {
		shell.Section(sess.Out, "Rounds:", "")
		return
	}
	roundLines := make([]string, 0, len(b.Log)*6)
	for i, r := range b.Log {
		if i > 0 {
			roundLines = append(roundLines, "")
		}
		roundLines = append(roundLines, fmt.Sprintf("Round %d", r.Round))
		roundLines = append(roundLines,
			fmt.Sprintf("  atk:        atk=%s  def=%s",
				formatOptFloat(r.AttackerAtk), formatOptFloat(r.AttackerDef)))
		roundLines = append(roundLines,
			fmt.Sprintf("  def:        atk=%s  def=%s",
				formatOptFloat(r.DefenderAtk), formatOptFloat(r.DefenderDef)))
		roundLines = append(roundLines,
			fmt.Sprintf("  damage:     attacker→defender=%s  defender→attacker=%s",
				formatFloat(r.AttackerDamageDealt), formatFloat(r.DefenderDamageDealt)))
		roundLines = append(roundLines,
			fmt.Sprintf("  casualties: attacker (%s)  defender (%s)",
				compositionString(r.AttackerCasualties), compositionString(r.DefenderCasualties)))
		if damage, hasDamage := r.WallsDamage.Get(); hasDamage && damage > 0 {
			levelAfter, hasLevel := r.WallsLevelAfter.Get()
			if hasLevel {
				roundLines = append(roundLines, fmt.Sprintf("  walls:      damage=%d  walls_level_after=%d", damage, levelAfter))
			} else {
				roundLines = append(roundLines, fmt.Sprintf("  walls:      damage=%d", damage))
			}
		}
	}
	shell.Section(sess.Out, "Rounds:", strings.Join(roundLines, "\n"))
}

// orderParticipants returns participants sorted attacker-first, then
// defender, then by kingdom_id for a stable order when both sides have
// multiple entries.
func orderParticipants(parts []gen.BattleParticipant) []gen.BattleParticipant {
	out := make([]gen.BattleParticipant, len(parts))
	copy(out, parts)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Side != out[j].Side {
			return out[i].Side == gen.BattleParticipantSideAttacker
		}
		ki, _ := out[i].KingdomID.Get()
		kj, _ := out[j].KingdomID.Get()
		return ki < kj
	})
	return out
}

func formatOptFloat(o gen.OptFloat64) string {
	if v, ok := o.Get(); ok {
		return formatFloat(v)
	}
	return "—"
}

// formatFloat prints integral values without a trailing ".0" and other
// values with up to two decimal places trimmed.
func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
