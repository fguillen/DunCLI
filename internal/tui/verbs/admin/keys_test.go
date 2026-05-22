package admin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// The admin verbs register into the shell's admin registry via init(),
// so a blank import of this package from cmd/dun is enough to wire
// them. They must NOT be visible in the player registry.
func TestKeysVerbRegisteredInAdminRegistry(t *testing.T) {
	v, ok := shell.ResolveAdmin("keys")
	require.True(t, ok)

	list, ok := v.Sub["list"]
	require.True(t, ok)
	require.NotNil(t, list.Run)

	revoke, ok := v.Sub["revoke"]
	require.True(t, ok)
	require.NotNil(t, revoke.Run)
	require.NotNil(t, revoke.Complete, "keys revoke <id> must offer tab completion")

	// The admin `keys` verb must never leak into the player shell.
	_, ok = shell.Resolve("keys")
	require.False(t, ok)
}
