package wonders

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// TestWonderSlugsMatchSpec pins the local slug list against the ogen
// enum. Backend co-evolution that adds or renames a wonder will fail
// loudly here.
func TestWonderSlugsMatchSpec(t *testing.T) {
	specSlugs := make(map[string]bool, len(wonderSlugs))
	for _, v := range gen.WonderStartRequestName("").AllValues() {
		specSlugs[string(v)] = true
	}
	for _, s := range wonderSlugs {
		require.True(t, specSlugs[s], "%q in wonderSlugs is not in the spec enum", s)
	}
	require.Equal(t, len(specSlugs), len(wonderSlugs),
		"wonderSlugs length must match spec enum size (got %d local vs %d spec)",
		len(wonderSlugs), len(specSlugs))

	// Every slug must have a title-case display name registered.
	for _, s := range wonderSlugs {
		require.NotEqual(t, s, wonderTitles[s], "no title for slug %q", s)
	}
}

func TestValidWonderSlug(t *testing.T) {
	require.True(t, validWonderSlug("sky_tower"))
	require.True(t, validWonderSlug("black_spire"))
	require.False(t, validWonderSlug("Sky Tower"))
	require.False(t, validWonderSlug("not_a_wonder"))
	require.False(t, validWonderSlug(""))
}

func TestTitleName(t *testing.T) {
	require.Equal(t, "Sky Tower", titleName("sky_tower"))
	require.Equal(t, "Library of Worlds", titleName("library_of_worlds"))
	// Unknown slugs pass through verbatim.
	require.Equal(t, "future_wonder", titleName("future_wonder"))
}

func TestCurrentPhaseKey(t *testing.T) {
	require.Equal(t, "foundation", currentPhaseKey(gen.WonderStatusFoundation))
	require.Equal(t, "construction", currentPhaseKey(gen.WonderStatusConstruction))
	require.Equal(t, "consecration", currentPhaseKey(gen.WonderStatusConsecration))
	require.Equal(t, "", currentPhaseKey(gen.WonderStatusCompleted))
	require.Equal(t, "", currentPhaseKey(gen.WonderStatusDestroyed))
}
