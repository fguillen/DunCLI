package shared

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelTime(t *testing.T) {
	require.Equal(t, "ready", RelTime(time.Now().Add(-time.Hour)))

	// The helper measures `time.Until(...)` against its own internal
	// clock read, so leave a small slack window to absorb scheduler
	// jitter on slow CI.
	require.Contains(t, []string{"29s", "30s"}, RelTime(time.Now().Add(30*time.Second)))
	require.Contains(t, []string{"4m", "5m"}, RelTime(time.Now().Add(5*time.Minute)))
	require.Contains(t, []string{"2h 29m", "2h 30m"}, RelTime(time.Now().Add(2*time.Hour+30*time.Minute)))
}
