package shell

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstToken(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"loop", "loop"},
		{"loop 5", "loop"},
		{"  where  ", "where"},
		{"server join acme", "server"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, firstToken(c.line), "firstToken(%q)", c.line)
	}
}

func TestRunLoop_noPreviousCommand(t *testing.T) {
	reset()
	var buf bytes.Buffer
	sess := &Session{Out: &buf}
	err := runLoop(context.Background(), sess, nil, nil)
	require.ErrorContains(t, err, "no previous command")
}

func TestRunLoop_invalidInterval(t *testing.T) {
	reset()
	for _, arg := range []string{"abc", "0", "-1", "1.5"} {
		var buf bytes.Buffer
		sess := &Session{Out: &buf, LastCommand: "where"}
		err := runLoop(context.Background(), sess, []string{arg}, nil)
		require.ErrorContains(t, err, "positive whole number", "arg=%q", arg)
	}
}

func TestRunLoop_repeatsLastCommandUntilCancelled(t *testing.T) {
	reset()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	// The stub cancels the context the first time it runs, so the loop
	// performs exactly one tick and then stops deterministically — no
	// real sleeping.
	Register(&Verb{
		Name: "tick",
		Run: func(context.Context, *Session, []string, map[string]string) error {
			calls++
			cancel()
			return nil
		},
	})

	var buf bytes.Buffer
	sess := &Session{Out: &buf, LastCommand: "tick"}
	err := runLoop(ctx, sess, []string{"1"}, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, calls, 1)
	out := buf.String()
	require.Contains(t, out, "Looping `tick`")
	require.Contains(t, out, "loop stopped")
}
