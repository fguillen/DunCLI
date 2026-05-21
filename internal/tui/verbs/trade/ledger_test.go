package trade

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/tui/shell"
)

func TestParseLedgerFlags_rejectsBadFlags(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string]string
		want  string
	}{
		{"limit non-numeric", map[string]string{"limit": "abc"}, "--limit must be a positive integer"},
		{"limit zero", map[string]string{"limit": "0"}, "--limit must be a positive integer"},
		{"limit too large", map[string]string{"limit": "999"}, "--limit must be ≤ 100"},
		{"page zero", map[string]string{"page": "0"}, "--page must be a positive integer"},
		{"page non-numeric", map[string]string{"page": "abc"}, "--page must be a positive integer"},
		{"since bad shape", map[string]string{"since": "yesterday"}, `--since must look like`},
		{"since bare number", map[string]string{"since": "5"}, `--since must look like`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, err := parseLedgerFlags(tc.flags)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseLedgerFlags_acceptsValidShapes(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string]string
	}{
		{"empty", nil},
		{"since 24h", map[string]string{"since": "24h"}},
		{"since 7d", map[string]string{"since": "7d"}},
		{"since 30m", map[string]string{"since": "30m"}},
		{"since 1h30m", map[string]string{"since": "1h30m"}},
		{"all flags", map[string]string{"player": "Alice", "since": "24h", "limit": "10", "page": "2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, err := parseLedgerFlags(tc.flags)
			require.NoError(t, err)
		})
	}
}

func TestRunTradeLedger_requiresWorldScope(t *testing.T) {
	sess, _, _ := newTestSession(t, tradeHandler(t, nil))
	err := runTradeLedger(context.Background(), sess, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in a server scope")
}

func TestRunTradeLedger_rendersEntriesAndPagination(t *testing.T) {
	var seen url.Values
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/trade-ledger": func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Query()
			_, _ = w.Write([]byte(`{
				"entries": [
					{
						"id": "tle-1", "caravan_id": "car-1",
						"sender_handle": "IronFist", "receiver_handle": "ShadowWolf",
						"resource": "gold", "amount": 5000,
						"status": "delivered",
						"recorded_at": "2026-05-20T21:30:00Z"
					},
					{
						"id": "tle-2", "caravan_id": "car-2",
						"sender_handle": "IronFist", "receiver_handle": "ShadowWolf",
						"attacker_handle": "RedTalon",
						"resource": "iron", "amount": 400,
						"status": "intercepted",
						"recorded_at": "2026-05-19T14:02:00Z"
					}
				],
				"pagy": {"count": 87, "page": 1, "limit": 25, "pages": 4}
			}`))
		},
	}
	sess, out := setupTradeSession(t, extra)
	err := runTradeLedger(context.Background(), sess, nil, map[string]string{
		"player": "IronFist",
		"since":  "24h",
	})
	require.NoError(t, err)
	require.Equal(t, "IronFist", seen.Get("player"))
	require.Equal(t, "24h", seen.Get("since"))
	got := out.String()
	require.Contains(t, got, "Trade ledger (page 1 of 4, showing 1-2 of 87)")
	require.Contains(t, got, "[player=IronFist since=24h]")
	require.Contains(t, got, "IronFist → ShadowWolf")
	require.Contains(t, got, "delivered")
	require.Contains(t, got, "intercepted")
	require.Contains(t, got, "attacker=RedTalon")
	require.Contains(t, got, "gold")
	require.Contains(t, got, "5000")
	require.Contains(t, got, "more: 85 remaining — `trade ledger --page 2`")
}

func TestRunTradeLedger_emptyState(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/trade-ledger": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"entries": [], "pagy": {"count": 0, "page": 1, "limit": 25, "pages": 0}}`))
		},
	}
	sess, out := setupTradeSession(t, extra)
	require.NoError(t, runTradeLedger(context.Background(), sess, nil, nil))
	require.Contains(t, out.String(), "Trade ledger:")
	require.Contains(t, out.String(), "(none)")
}

func TestRunTradeLedger_lastPageHasNoMoreHint(t *testing.T) {
	extra := map[string]http.HandlerFunc{
		"/v1/worlds/wld-1/trade-ledger": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"entries": [{
					"id": "tle-1", "caravan_id": "car-1",
					"sender_handle": "A", "receiver_handle": "B",
					"resource": "gold", "amount": 10,
					"status": "delivered",
					"recorded_at": "2026-05-20T21:30:00Z"
				}],
				"pagy": {"count": 1, "page": 1, "limit": 25, "pages": 1}
			}`))
		},
	}
	sess, out := setupTradeSession(t, extra)
	require.NoError(t, runTradeLedger(context.Background(), sess, nil, nil))
	require.NotContains(t, out.String(), "more:")
}

func TestSuggestStaticValues(t *testing.T) {
	got, err := suggestSinceValues(context.Background(), nil, "")
	require.NoError(t, err)
	require.Contains(t, got, "24h")
	require.Contains(t, got, "7d")

	got, err = suggestLimitValues(context.Background(), nil, "")
	require.NoError(t, err)
	require.Contains(t, got, "25")
	require.Contains(t, got, "100")
}

func TestVerbsRegistered_trade(t *testing.T) {
	parent, ok := shell.Resolve("trade")
	require.True(t, ok)
	ledger, ok := parent.Sub["ledger"]
	require.True(t, ok)
	require.NotNil(t, ledger.Run)
	require.True(t, hasFlag(ledger, "player"))
	require.True(t, hasFlag(ledger, "since"))
	require.True(t, hasFlag(ledger, "limit"))
	require.True(t, hasFlag(ledger, "page"))
}

func hasFlag(v *shell.Verb, name string) bool {
	for _, f := range v.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}
