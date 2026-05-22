package verbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api"
)

func TestValidateHandle(t *testing.T) {
	cases := map[string]bool{
		"IronFist":                   true,
		"iron-fist":                  true,
		"iron_fist_1":                true,
		"abc":                        true,
		"a":                          false,
		"ab":                         false,
		"":                           false,
		"with space":                 false,
		"bang!":                      false,
		"twentyfivecharacterhandle!": false,
	}
	for in, ok := range cases {
		err := validateHandle(in)
		if ok {
			require.NoError(t, err, "expected %q valid", in)
		} else {
			require.Error(t, err, "expected %q invalid", in)
		}
	}
}

func TestRunProfileSet_handleLocked(t *testing.T) {
	sess, _, _ := newTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-pl")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/servers"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{{"id": "srv-1", "slug": "acme", "name": "Acme", "member": true}},
			})
		case strings.HasSuffix(r.URL.Path, "/servers/srv-1/me"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "handle_locked", "message": "handle cannot be changed"},
			})
		}
	})

	sess.Context.SetServer("acme")
	err := runProfileSet(context.Background(), sess, nil, map[string]string{"handle": "NewHandle"})
	require.Error(t, err)
	apiErr := api.AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "handle_locked", apiErr.Code)
}

func TestRunProfileSet_rejectsBadHandleClientSide(t *testing.T) {
	calls := 0
	sess, _, _ := newTestSession(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})
	sess.Context.SetServer("acme")
	err := runProfileSet(context.Background(), sess, nil, map[string]string{"handle": "bad handle!"})
	require.Error(t, err)
	require.Zero(t, calls, "client-side rejection must short-circuit before HTTP")
}

func TestRunProfileSet_requiresServerScope(t *testing.T) {
	sess, _, _ := newTestSession(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	err := runProfileSet(context.Background(), sess, nil, map[string]string{"handle": "IronFist"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "server scope")
}
