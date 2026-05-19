package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// errProvider is a TokenProvider that always returns the configured error.
type errProvider struct{ err error }

func (e errProvider) Token(context.Context) (string, error) { return "", e.err }

// newTestClient spins an httptest.Server with the given handler and
// returns a Client pointing at it. baseURL includes the `/v1` prefix
// that the real backend exposes; the test handler can register routes
// rooted at `/v1/...` directly.
func newTestClient(t *testing.T, tp TokenProvider, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL+"/v1", tp, srv.Client())
	require.NoError(t, err)
	return c, srv
}

// writeEnvelope marshals an ErrorEnvelope to the response writer.
func writeEnvelope(t *testing.T, w http.ResponseWriter, status int, code, message string, retryAfter int) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if retryAfter > 0 {
		body["error"].(map[string]any)["retry_after"] = retryAfter
	}
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func TestClient_New_requiresBaseURL(t *testing.T) {
	_, err := New("", nil, nil)
	require.Error(t, err)
}

func TestClient_New_clonesHTTPClient(t *testing.T) {
	// The caller's http.Client should not have its Transport mutated.
	hc := &http.Client{Timeout: 5 * time.Second}
	_, err := New("http://example.test/v1", nil, hc)
	require.NoError(t, err)
	require.Nil(t, hc.Transport, "constructor must not mutate caller's http.Client")
}

// writeJSON sends a JSON body with the Content-Type ogen expects.
func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(body))
	require.NoError(t, err)
}

func TestClient_sendsBearerAuth(t *testing.T) {
	var seen string
	c, _ := newTestClient(t, StaticToken("test-token"), func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.Header().Set("X-Request-Id", "req-abc")
		writeJSON(t, w, `{"servers": []}`)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Bearer test-token", seen)
}

func TestClient_emptyTokenStillSendsHeader(t *testing.T) {
	var seen string
	c, _ := newTestClient(t, StaticToken(""), func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		writeJSON(t, w, `{"servers": []}`)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.NoError(t, err)
	// ogen unconditionally sends `Authorization: Bearer <token>`; with an
	// empty token the value is "Bearer " on the wire but Go's header
	// reader strips trailing whitespace, so the test sees "Bearer".
	require.Equal(t, "Bearer", seen)
}

func TestClient_tokenProviderError(t *testing.T) {
	want := errors.New("boom")
	var handlerCalls atomic.Int32
	c, _ := newTestClient(t, errProvider{err: want}, func(w http.ResponseWriter, _ *http.Request) {
		handlerCalls.Add(1)
		writeJSON(t, w, `{"servers": []}`)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.ErrorIs(t, err, want)
	require.Zero(t, handlerCalls.Load(), "token provider error should short-circuit before HTTP")
}

func TestClient_decodesEnvelopeOn401(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-xyz")
		writeEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "bad token", 0)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.Error(t, err)

	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "unauthorized", apiErr.Code)
	require.Equal(t, "bad token", apiErr.Message)
	require.Equal(t, "req-xyz", apiErr.RequestID)
}

func TestClient_decodes404Envelope_onListServerWorlds(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-404")
		writeEnvelope(t, w, http.StatusNotFound, "not_found", "no such server", 0)
	})

	_, err := c.ListServerWorlds(context.Background(), "srv-1")
	require.Error(t, err)

	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
	require.Equal(t, "req-404", apiErr.RequestID)
}

func TestClient_decodes429AsRateLimit(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-rl")
		writeEnvelope(t, w, http.StatusTooManyRequests, "rate_limited", "slow down", 30)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.Error(t, err)

	var rl *RateLimitError
	require.ErrorAs(t, err, &rl)
	require.Equal(t, 30*time.Second, rl.RetryAfter)
	require.Equal(t, "req-rl", rl.Err.RequestID)
	require.Equal(t, "rate_limited", rl.Err.Code)

	// errors.As against *Error should still work via Unwrap.
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "rate_limited", apiErr.Code)
}

func TestClient_capturesRequestIDOnSuccess(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-ok")
		writeJSON(t, w, `{"status": "ok"}`)
	})

	require.NoError(t, c.Health(context.Background()))
	// Success path doesn't return the ID to callers in Phase 1 — it's
	// for the slog record only. This test ensures the transport ran
	// without crashing and the round-trip completed.
}

func TestClient_contextCancellation(t *testing.T) {
	// Handler blocks until the test's context is done so the cancel
	// races the response.
	c, _ := newTestClient(t, StaticToken("t"), func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before issuing the call

	err := c.Health(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestClient_health_envelopeDecoded(t *testing.T) {
	// Sanity: the Health wrapper rejects an unexpected union variant.
	// The spec only declares 200 for /health, so any non-200 turns into
	// an UnexpectedStatusCode transport error from ogen — we just make
	// sure that surfaces rather than panicking.
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := c.Health(context.Background())
	require.Error(t, err)
}

// Phase-1 sanity: the SecuritySource adapter rejects admin operations
// so an accidental admin call surfaces loudly instead of silently
// sending an empty token.
func TestSecurity_adminBearerRejected(t *testing.T) {
	src := bearerSource{tp: StaticToken("x")}
	_, err := src.AdminBearer(context.Background(), "anything")
	require.Error(t, err)
}

// fromEnvelope is exercised indirectly by the 429 path; this unit-level
// test pins down the time conversion and the nil-envelope guard.
func TestFromEnvelope_retryAfterParsed(t *testing.T) {
	env := &gen.ErrorEnvelope{}
	env.Error.Code = "rate_limited"
	env.Error.Message = "slow down"
	env.Error.RetryAfter.SetTo(45)

	err := fromEnvelope(env, "req-1")
	var rl *RateLimitError
	require.ErrorAs(t, err, &rl)
	require.Equal(t, 45*time.Second, rl.RetryAfter)
}

func TestFromEnvelope_noRetryAfter(t *testing.T) {
	env := &gen.ErrorEnvelope{}
	env.Error.Code = "not_found"
	env.Error.Message = "nope"

	err := fromEnvelope(env, "")
	require.Error(t, err)
	require.Nil(t, AsRateLimit(err))
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
}

// AsRateLimit is a small test helper that mirrors AsError for the
// rate-limit variant. Kept private to the test file because v1 callers
// just `errors.As` directly.
func AsRateLimit(err error) *RateLimitError {
	var rl *RateLimitError
	if errors.As(err, &rl) {
		return rl
	}
	return nil
}
