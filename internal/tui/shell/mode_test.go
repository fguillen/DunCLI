package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_playerAndAdminAreIsolated(t *testing.T) {
	reset()
	Register(&Verb{Name: "player-only", Run: noopRun})
	RegisterAdmin(&Verb{Name: "admin-only", Run: noopRun})

	// The player registry sees player verbs only.
	_, ok := defaultRegistry.resolve("player-only")
	require.True(t, ok)
	_, ok = defaultRegistry.resolve("admin-only")
	require.False(t, ok, "admin verbs must not leak into the player shell")

	// The admin registry sees admin verbs only.
	_, ok = adminRegistry.resolve("admin-only")
	require.True(t, ok)
	_, ok = adminRegistry.resolve("player-only")
	require.False(t, ok, "player verbs must not leak into the admin shell")
}

func TestDispatch_resolvesAgainstSessionMode(t *testing.T) {
	reset()
	var (
		ranPlayer bool
		ranAdmin  bool
	)
	Register(&Verb{Name: "p", Run: func(context.Context, *Session, []string, map[string]string) error {
		ranPlayer = true
		return nil
	}})
	RegisterAdmin(&Verb{Name: "a", Run: func(context.Context, *Session, []string, map[string]string) error {
		ranAdmin = true
		return nil
	}})

	var buf strings.Builder
	admin := &Session{Out: &buf, Mode: ModeAdmin}
	require.NoError(t, Dispatch(context.Background(), admin, "a"))
	require.True(t, ranAdmin)

	// The admin session cannot reach a player verb.
	buf.Reset()
	require.NoError(t, Dispatch(context.Background(), admin, "p"))
	require.False(t, ranPlayer)
	require.Contains(t, buf.String(), "unknown command")
}

func TestRegisterBuiltinsOnce_perRegistry(t *testing.T) {
	reset()
	// Builtins land in whichever registry is asked, and the per-registry
	// guard makes a repeat call a no-op rather than a duplicate panic.
	registerBuiltinsOnce(adminRegistry, "v-test")
	registerBuiltinsOnce(adminRegistry, "v-test")
	_, ok := adminRegistry.resolve("help")
	require.True(t, ok)
	_, ok = defaultRegistry.resolve("help")
	require.False(t, ok, "registering admin builtins must not touch the player registry")
}

func TestWhere_adminModeOmitsKingdom(t *testing.T) {
	reset()
	registerBuiltins(adminRegistry, "v-test")

	var buf strings.Builder
	sess := &Session{Out: &buf, Mode: ModeAdmin, Context: NewContext()}
	require.NoError(t, Dispatch(context.Background(), sess, "where"))

	out := buf.String()
	require.Contains(t, out, "server")
	require.Contains(t, out, "world")
	require.NotContains(t, out, "kingdom", "admin `where` must not render a kingdom line")
}

func TestAdminFileStore_separateFromPlayerState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	player := NewFileStore("http://localhost:3000/v1", "alice@example.com")
	admin := NewAdminFileStore("http://localhost:3000/v1", "alice@example.com")

	require.NoError(t, player.Save(ContextSnapshot{ServerSlug: "acme", WorldSlug: "spring-2026"}))
	require.NoError(t, admin.Save(ContextSnapshot{ServerSlug: "beta", WorldSlug: "autumn-2026"}))

	// Two distinct files on disk.
	_, err := os.Stat(filepath.Join(dir, ".dun", StateFileName))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, ".dun", AdminStateFileName))
	require.NoError(t, err)

	// Each store loads its own scope, not the other's.
	gotPlayer, err := player.Load()
	require.NoError(t, err)
	require.Equal(t, "acme", gotPlayer.ServerSlug)

	gotAdmin, err := admin.Load()
	require.NoError(t, err)
	require.Equal(t, "beta", gotAdmin.ServerSlug)
}
