package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCommand_PrintsConstant(t *testing.T) {
	var out bytes.Buffer
	root := newRootCmd(&out)
	root.SetArgs([]string{"version"})

	require.NoError(t, root.Execute())
	require.Equal(t, Version, strings.TrimSpace(out.String()))
}
