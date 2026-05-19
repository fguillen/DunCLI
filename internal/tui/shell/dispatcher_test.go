package shell

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// installTestVerbs wipes the registry and adds a small, well-known
// verb tree the parser/dispatcher tests can rely on. Tests call this
// from t.Cleanup-style setup so cases don't leak verbs into each
// other.
func installTestVerbs() {
	reset()
	Register(&Verb{
		Name:    "leaf",
		Summary: "leaf verb",
		Run:     func(context.Context, *Session, []string, map[string]string) error { return nil },
		Flags: []FlagSpec{
			{Name: "name", HasValue: true},
			{Name: "force", HasValue: false},
		},
	})
	Register(&Verb{
		Name:    "branch",
		Summary: "branching verb",
		Sub: map[string]*Verb{
			"sub": {
				Name:    "sub",
				Summary: "leaf under branch",
				Run:     func(context.Context, *Session, []string, map[string]string) error { return nil },
				Flags:   []FlagSpec{{Name: "flag", HasValue: true}},
			},
		},
	})
}

func TestParse_emptyLine(t *testing.T) {
	installTestVerbs()
	cmd, err := Parse("   ")
	require.NoError(t, err)
	require.Nil(t, cmd)
}

func TestParse_unknownVerb(t *testing.T) {
	installTestVerbs()
	_, err := Parse("nope arg")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown command")
}

func TestParse_subverbResolved(t *testing.T) {
	installTestVerbs()
	cmd, err := Parse("branch sub one two")
	require.NoError(t, err)
	require.Equal(t, []string{"branch", "sub"}, cmd.Path)
	require.Equal(t, []string{"one", "two"}, cmd.Args)
}

func TestParse_branchWithoutSub(t *testing.T) {
	installTestVerbs()
	_, err := Parse("branch")
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires a subcommand")
}

func TestParse_unknownSubverb(t *testing.T) {
	installTestVerbs()
	_, err := Parse("branch bogus")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}

func TestParse_flagWithValue(t *testing.T) {
	installTestVerbs()
	cmd, err := Parse(`leaf --name "Iron Fist"`)
	require.NoError(t, err)
	require.Equal(t, "Iron Fist", cmd.Flags["name"])
}

func TestParse_inlineEquals(t *testing.T) {
	installTestVerbs()
	cmd, err := Parse("leaf --name=Iron")
	require.NoError(t, err)
	require.Equal(t, "Iron", cmd.Flags["name"])
}

func TestParse_booleanFlag(t *testing.T) {
	installTestVerbs()
	cmd, err := Parse("leaf --force")
	require.NoError(t, err)
	require.Equal(t, "true", cmd.Flags["force"])
}

func TestParse_booleanFlagWithValueRejected(t *testing.T) {
	installTestVerbs()
	_, err := Parse("leaf --force=yes")
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not take a value")
}

func TestParse_unknownFlag(t *testing.T) {
	installTestVerbs()
	_, err := Parse("leaf --bogus")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag")
}

func TestParse_missingFlagValue(t *testing.T) {
	installTestVerbs()
	_, err := Parse("leaf --name")
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires a value")
}

func TestDispatch_runsVerbAndExits(t *testing.T) {
	reset()
	var ran bool
	Register(&Verb{
		Name: "ping",
		Run: func(_ context.Context, _ *Session, _ []string, _ map[string]string) error {
			ran = true
			return nil
		},
	})
	Register(&Verb{
		Name: "stop",
		Run:  func(_ context.Context, _ *Session, _ []string, _ map[string]string) error { return ErrExit },
	})

	var buf strings.Builder
	sess := &Session{Out: &buf}

	require.NoError(t, Dispatch(context.Background(), sess, "ping"))
	require.True(t, ran)

	err := Dispatch(context.Background(), sess, "stop")
	require.ErrorIs(t, err, ErrExit)
}
