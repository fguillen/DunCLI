// Package shared holds tiny helpers that more than one verbs package
// needs: the scope guards (RequireWorldID, RequireKingdomID) and the
// human-friendly duration formatter (RelTime). Kept narrow on purpose
// — sibling packages should import this and not each other.
package shared

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

// RequireWorldID resolves the in-scope (server, world) tuple to a
// world ULID. The two user-facing errors map onto the two missing
// scope cases.
func RequireWorldID(ctx context.Context, sess *shell.Session) (string, error) {
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return "", errors.New("not in a server scope — try `server join <slug>` first")
	}
	worldSlug := sess.Context.WorldSlug()
	if worldSlug == "" {
		return "", errors.New("not in a world scope — try `world join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return "", err
	}
	return sess.API.ResolveWorld(ctx, serverID, worldSlug)
}

// RequireKingdomID resolves the caller's kingdom ULID in the in-scope
// world. Propagates the world-scope errors from RequireWorldID and
// remaps a backend not_found into the "no kingdom yet" advisory.
func RequireKingdomID(ctx context.Context, sess *shell.Session) (string, error) {
	worldID, err := RequireWorldID(ctx, sess)
	if err != nil {
		return "", err
	}
	id, err := sess.API.ResolveKingdom(ctx, worldID)
	if err != nil {
		if apiErr := api.AsError(err); apiErr != nil && apiErr.Code == "not_found" {
			return "", errors.New("you have no kingdom in this world — try `world join <slug>` first")
		}
		return "", err
	}
	return id, nil
}

// RelTime renders a "time until t" as a compact human string.
// Negative durations collapse to "ready".
func RelTime(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return "ready"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}
