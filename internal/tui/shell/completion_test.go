package shell

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompleter_topLevelVerbs(t *testing.T) {
	reset()
	Register(&Verb{Name: "alpha", Run: noopRun})
	Register(&Verb{Name: "beta", Run: noopRun})

	c := &completer{sess: &Session{}}
	got := c.suggest(nil, "a", false)
	require.ElementsMatch(t, []string{"alpha", "beta"}, got)
}

func TestCompleter_subverbs(t *testing.T) {
	reset()
	Register(&Verb{
		Name: "branch",
		Sub: map[string]*Verb{
			"foo": {Name: "foo", Run: noopRun},
			"bar": {Name: "bar", Run: noopRun},
		},
	})

	c := &completer{sess: &Session{}}
	got := c.suggest([]string{"branch"}, "", true)
	sort.Strings(got)
	require.Equal(t, []string{"bar", "foo"}, got)
}

func TestCompleter_flagNames(t *testing.T) {
	reset()
	Register(&Verb{
		Name:  "leaf",
		Run:   noopRun,
		Flags: []FlagSpec{{Name: "alpha", HasValue: true}, {Name: "beta", HasValue: false}},
	})

	c := &completer{sess: &Session{}}
	got := c.suggest([]string{"leaf"}, "--", false)
	sort.Strings(got)
	require.Equal(t, []string{"--alpha", "--beta"}, got)
}

func TestCompleter_dynamicPositional(t *testing.T) {
	reset()
	Register(&Verb{
		Name: "leaf",
		Run:  noopRun,
		Complete: SuggestFunc(func(_ context.Context, _ *Session, _ string) ([]string, error) {
			return []string{"acme", "beta"}, nil
		}),
	})

	c := &completer{sess: &Session{}}
	got := c.suggest([]string{"leaf"}, "", true)
	require.ElementsMatch(t, []string{"acme", "beta"}, got)
}

func TestCompleter_dynamicFlagValue(t *testing.T) {
	reset()
	Register(&Verb{
		Name: "leaf",
		Run:  noopRun,
		Flags: []FlagSpec{
			{
				Name: "kind", HasValue: true,
				Suggest: SuggestFunc(func(_ context.Context, _ *Session, _ string) ([]string, error) {
					return []string{"farm", "barracks"}, nil
				}),
			},
		},
	})

	c := &completer{sess: &Session{}}
	got := c.suggest([]string{"leaf", "--kind"}, "", true)
	require.ElementsMatch(t, []string{"farm", "barracks"}, got)
}

func noopRun(context.Context, *Session, []string, map[string]string) error { return nil }
