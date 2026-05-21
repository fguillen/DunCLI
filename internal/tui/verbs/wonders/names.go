// Package wonders hosts the Phase 12 wonder verbs: `wonder show`,
// `wonder start`, `wonder cancel`, `wonder repair`, `wonder milestone`
// (all kingdom-scoped) and the plural top-level `wonders` (world-scoped
// public list). Construction outcomes — milestone auto-pauses, repairs,
// trebuchet damage — are surfaced via the Phase 9 battle stream and
// the regular `wonder show` lazy-applied HP read; this package never
// resolves combat itself.
package wonders

import "github.com/fguillen/dun-cli/internal/api/gen"

// wonderSlugs is the §14 fixed menu, in display order. Pinned to the
// spec enum (gen.WonderStartRequestName.AllValues()); a regression
// test in names_test.go fails loudly if the spec drifts.
var wonderSlugs = []string{
	"sky_tower",
	"eternal_citadel",
	"cathedral_of_ages",
	"library_of_worlds",
	"crown_of_kings",
	"black_spire",
}

// wonderTitles maps each slug to its title-case display form. Used by
// the `wonder start` picker, the `wonder show` header, and the plural
// `wonders` list.
var wonderTitles = map[string]string{
	"sky_tower":         "Sky Tower",
	"eternal_citadel":   "Eternal Citadel",
	"cathedral_of_ages": "Cathedral of Ages",
	"library_of_worlds": "Library of Worlds",
	"crown_of_kings":    "Crown of Kings",
	"black_spire":       "Black Spire",
}

// titleName returns the title-case display name for a wonder slug.
// Unknown slugs are returned verbatim — a forward-compat hedge if the
// backend adds a name before the CLI catches up.
func titleName(slug string) string {
	if t, ok := wonderTitles[slug]; ok {
		return t
	}
	return slug
}

// validWonderSlug reports whether s is one of the six §14 slugs.
func validWonderSlug(s string) bool {
	_, ok := wonderTitles[s]
	return ok
}

// currentPhaseKey maps a Wonder.Status to the matching key in
// `repaired_hp_by_phase`. Returns "" for terminal statuses (completed,
// destroyed) where the per-phase repair counter no longer applies.
func currentPhaseKey(s gen.WonderStatus) string {
	switch s {
	case gen.WonderStatusFoundation:
		return "foundation"
	case gen.WonderStatusConstruction:
		return "construction"
	case gen.WonderStatusConsecration:
		return "consecration"
	}
	return ""
}
