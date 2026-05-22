// Package admin hosts the verbs of the `dun-admin>` shell. They
// register into the shell's separate admin verb registry via
// shell.RegisterAdmin, so a blank import of this package from
// cmd/dun is enough to wire them — exactly as the player verb
// packages register via shell.Register.
//
// Phase 14 ships only `keys` (admin ApiKey management); Phases 15–18
// add the admin server / world / team verbs.
package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/auth"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.RegisterAdmin(&shell.Verb{
		Name:    "keys",
		Summary: "Manage your admin ApiKeys",
		Usage:   "keys <list|revoke> ...",
		Sub: map[string]*shell.Verb{
			"list": {
				Name:    "list",
				Summary: "List the admin ApiKeys issued for your account",
				Usage:   "keys list",
				Run:     runKeysList,
			},
			"revoke": {
				Name:     "revoke",
				Summary:  "Revoke an admin ApiKey by id",
				Usage:    "keys revoke <id>",
				Run:      runKeysRevoke,
				Complete: shell.SuggestFunc(suggestKeyIDs),
			},
		},
	})
}

// ── keys list ────────────────────────────────────────────────────────

func runKeysList(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	keys, err := sess.API.ListAdminAPIKeys(ctx)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(sess.Out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tLAST USED\tEXPIRES\tSTATUS")
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			k.ID,
			nameOrDash(k.Name),
			timeOrDash(k.LastUsedAt),
			k.ExpiresAt.Format("2006-01-02"),
			keyStatus(k),
		)
	}
	return w.Flush()
}

// ── keys revoke ──────────────────────────────────────────────────────

func runKeysRevoke(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return errors.New("usage: keys revoke <id>")
	}
	id := strings.TrimSpace(args[0])

	// List first so we know whether the user is revoking *this shell's*
	// current key — if so we also clear the local admin credential after
	// the server-side revoke succeeds.
	keys, err := sess.API.ListAdminAPIKeys(ctx)
	if err != nil {
		return err
	}
	var revokingCurrent bool
	for _, k := range keys {
		if k.ID == id && k.Current {
			revokingCurrent = true
			break
		}
	}

	if err := sess.API.RevokeAdminAPIKey(ctx, id); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(sess.Out, "Revoked admin key %s.\n", id)

	if revokingCurrent {
		if err := clearLocalAdminCredential(sess); err != nil {
			return fmt.Errorf("revoked, but could not clear local admin credential: %w", err)
		}
		_, _ = fmt.Fprintln(sess.Out,
			"that was this session's key — local admin credential cleared; run `dun admin login` again")
	}
	return nil
}

// clearLocalAdminCredential removes the current admin entry from
// ~/.dun/credentials. The shell Session does not carry the auth.Store,
// so we reload it from disk, drop the entry, and persist — the
// server-side key is already revoked, so a stale in-memory copy on the
// api.Client is harmless (subsequent calls just 401).
func clearLocalAdminCredential(sess *shell.Session) error {
	store, err := auth.LoadStore()
	if err != nil {
		return err
	}
	store.Delete(sess.Cfg.BaseURL, sess.Creds.Email, auth.ScopeAdmin)
	return store.Save()
}

// suggestKeyIDs feeds tab completion for `keys revoke <Tab>`. Honors
// ctx — the completion engine times us out at 800 ms.
func suggestKeyIDs(ctx context.Context, sess *shell.Session, _ string) ([]string, error) {
	keys, err := sess.API.ListAdminAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if _, revoked := k.RevokedAt.Get(); revoked {
			continue
		}
		out = append(out, k.ID)
	}
	return out, nil
}

// ── rendering helpers ────────────────────────────────────────────────

func nameOrDash(n gen.NilString) string {
	v, ok := n.Get()
	if !ok || v == "" {
		return "-"
	}
	return v
}

func timeOrDash(t gen.NilDateTime) string {
	v, ok := t.Get()
	if !ok {
		return "-"
	}
	return v.Format(time.RFC3339)
}

func keyStatus(k gen.ApiKeyEntry) string {
	if _, ok := k.RevokedAt.Get(); ok {
		return "revoked"
	}
	if k.Current {
		return "current"
	}
	return "active"
}
