package verbs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fguillen/dun-cli/internal/api/gen"
	"github.com/fguillen/dun-cli/internal/tui/shell"
	"github.com/fguillen/dun-cli/internal/tui/verbs/shared"
)

// eventsDefaultLimit / eventsMaxLimit mirror the spec's default (10) and
// maximum (100) on listKingdomEvents.
const (
	eventsDefaultLimit = 10
	eventsMaxLimit     = 100
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "events",
		Summary: "Show your kingdom's recent event timeline (oldest first)",
		Usage:   "events [<num>]",
		Run:     runEvents,
	})
}

func runEvents(ctx context.Context, sess *shell.Session, args []string, _ map[string]string) error {
	if len(args) > 1 {
		return errors.New("usage: events [<num>]")
	}
	num := eventsDefaultLimit
	if len(args) == 1 {
		n, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil || n < 1 || n > eventsMaxLimit {
			return fmt.Errorf("events: <num> must be between 1 and %d", eventsMaxLimit)
		}
		num = n
	}

	kingdomID, err := shared.RequireKingdomID(ctx, sess)
	if err != nil {
		return err
	}
	events, err := sess.API.ListKingdomEvents(ctx, kingdomID, num)
	if err != nil {
		return err
	}
	printEventsList(sess, events)
	return nil
}

// printEventsList renders the event timeline in backend order (oldest
// first, newest last). Empty feeds come out as "(none)".
func printEventsList(sess *shell.Session, events []gen.Event) {
	if len(events) == 0 {
		shell.Section(sess.Out, "Recent events:", "")
		return
	}
	lines := make([]string, 0, len(events))
	for _, e := range events {
		lines = append(lines, fmt.Sprintf("%s  %-8s  %s",
			e.OccurredAt.UTC().Format("2006-01-02 15:04 UTC"),
			string(e.Type),
			e.Description))
	}
	shell.Section(sess.Out, "Recent events:", strings.Join(lines, "\n"))
}
