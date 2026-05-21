package wonders

import (
	"context"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func init() {
	shell.Register(&shell.Verb{
		Name:    "wonder",
		Summary: "Inspect or mutate your kingdom's wonder",
		Usage:   "wonder [show|start|cancel|repair|milestone] ...",
		Run:     runWonderShow,
		Sub: map[string]*shell.Verb{
			"show": {
				Name:    "show",
				Summary: "Show your kingdom's wonder (HP, pending milestone, paused state)",
				Usage:   "wonder show",
				Run:     runWonderShow,
			},
			"start": {
				Name:     "start",
				Summary:  "Begin construction of a wonder (deducts 25% foundation payment)",
				Usage:    "wonder start [<name>]",
				Run:      runWonderStart,
				Complete: shell.SuggestFunc(suggestWonderNames),
			},
			"cancel": {
				Name:    "cancel",
				Summary: "Abandon construction (paid resources lost — typed-name confirm)",
				Usage:   "wonder cancel",
				Run:     runWonderCancel,
			},
			"repair": {
				Name:    "repair",
				Summary: "Spend Stone to restore wonder HP (1 HP per 8 Stone, 2000 HP/phase cap)",
				Usage:   "wonder repair [<hp>]",
				Run:     runWonderRepair,
			},
			"milestone": {
				Name:     "milestone",
				Summary:  "Pay the pending 25/50/75% milestone to resume construction",
				Usage:    "wonder milestone [25|50|75]",
				Run:      runWonderMilestone,
				Complete: shell.SuggestFunc(suggestMilestonePercents),
			},
		},
	})

	shell.Register(&shell.Verb{
		Name:    "wonders",
		Summary: "List every wonder under construction (or completed) in the in-scope world",
		Usage:   "wonders",
		Run:     runWondersList,
	})
}

// suggestWonderNames is the static completer for `wonder start <Tab>`.
// Returns the six §14 slugs in display order.
func suggestWonderNames(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
	out := make([]string, len(wonderSlugs))
	copy(out, wonderSlugs)
	return out, nil
}

// suggestMilestonePercents is the static completer for
// `wonder milestone <Tab>`. The valid set is fixed by spec
// (`WonderMilestoneRequestPercent`); the verb still checks the
// pending percent against the wonder's current state before sending.
func suggestMilestonePercents(_ context.Context, _ *shell.Session, _ string) ([]string, error) {
	return []string{"25", "50", "75"}, nil
}
