package verbs

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/tui/selector"
	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "profile",
		Summary: "Read or update your per-server profile",
		Usage:   "profile <show|set> [flags]",
		Sub: map[string]*shell.Verb{
			"show": {
				Name:    "show",
				Summary: "Show your profile on the in-scope server",
				Usage:   "profile show",
				Run:     runProfileShow,
			},
			"set": {
				Name:    "set",
				Summary: "Update your handle and/or real name on the in-scope server",
				Usage:   "profile set [--handle X] [--real-name Y]",
				Run:     runProfileSet,
				Flags: []shell.FlagSpec{
					{Name: "handle", HasValue: true, Help: "new player handle (alphanumeric / underscore / hyphen, 3-24 chars)"},
					{Name: "real-name", HasValue: true, Help: "new display real name"},
				},
			},
		},
	})
}

// handleSyntax is the client-side cheap check from §17.1. The
// backend stays the source of truth; this only catches obviously
// wrong input before we burn a request.
var handleSyntax = regexp.MustCompile(`^[A-Za-z0-9_-]{3,24}$`)

// runProfileShow implements `profile show` — uses the existing
// ShowPlayerProfile endpoint scoped to the in-context server, with
// the caller's own handle pulled from the current context (when
// known) or rejected with a clear error if not.
func runProfileShow(ctx context.Context, sess *shell.Session, _ []string, _ map[string]string) error {
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}
	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}
	handle := sess.Context.KingdomHandle()
	if handle == "" {
		// We don't know our own handle on this server yet. For Phase
		// 4 the only way to learn it is via a profile fetch — which
		// requires the handle. Surface as a backend co-evolution
		// note. The user can still set a fresh handle via
		// `profile set --handle ...`.
		return errors.New("don't know your handle on this server yet — set one with `profile set --handle <name>`")
	}
	prof, err := sess.API.ShowPlayerProfile(ctx, serverID, handle)
	if err != nil {
		return err
	}
	printProfileRead(sess, prof)
	return nil
}

// runProfileSet implements `profile set` with or without flags.
func runProfileSet(ctx context.Context, sess *shell.Session, _ []string, flags map[string]string) error {
	serverSlug := sess.Context.ServerSlug()
	if serverSlug == "" {
		return errors.New("not in a server scope — try `server join <slug>` first")
	}

	handle, hasHandle := flags["handle"]
	realName, hasReal := flags["real-name"]

	if !hasHandle && !hasReal {
		// No flags → open a huh form.
		res, err := selector.Form(ctx, "Update profile", []selector.Field{
			{Key: "handle", Label: "Handle (3-24 chars, A-Z 0-9 _ -)", Initial: sess.Context.KingdomHandle(), Validate: validateHandle},
			{Key: "real_name", Label: "Real name (optional)"},
		})
		if err != nil {
			if errors.Is(err, selector.ErrCancelled) {
				return nil
			}
			return err
		}
		handle = res["handle"]
		realName = res["real_name"]
		hasHandle = handle != ""
		hasReal = realName != ""
	} else if hasHandle {
		if err := validateHandle(handle); err != nil {
			return err
		}
	}

	if !hasHandle && !hasReal {
		return errors.New("nothing to update")
	}

	serverID, err := sess.API.ResolveServer(ctx, serverSlug)
	if err != nil {
		return err
	}

	update := api.ProfileUpdate{}
	if hasHandle {
		update.Handle = &handle
	}
	if hasReal {
		update.RealName = &realName
	}

	prof, err := sess.API.UpdateOwnProfile(ctx, serverID, update)
	if err != nil {
		return err
	}

	if h, ok := prof.Handle.Get(); ok {
		sess.Context.SetKingdomHandle(h)
		if perr := sess.State.Save(sess.Context.Snapshot()); perr != nil {
			shell.Err(sess.Out, fmt.Errorf("persist state: %w", perr))
		}
	}
	printProfileWrite(sess, prof)
	return nil
}

// validateHandle enforces the §17.1 syntax rule locally so a clearly-
// bad handle does not consume an HTTP round-trip.
func validateHandle(in string) error {
	if !handleSyntax.MatchString(in) {
		return errors.New("handle must be 3-24 chars: letters, digits, underscore, hyphen")
	}
	return nil
}
