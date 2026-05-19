package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// respMeta carries per-operation response metadata that the wrapper
// methods need but the generated client never exposes: X-Request-Id, the
// HTTP status code, and a typed *RateLimitError when the response was a
// 429. A pointer to a freshly-allocated respMeta is stashed in the
// operation's context.Context before the call; the transport reads the
// pointer from req.Context() and writes the captured fields into it on
// every response. Wrapper methods read it back after the call.
type respMeta struct {
	requestID string
	status    int
	rateLimit *RateLimitError
}

type metaKey struct{}

// withMeta returns a derived context carrying a fresh respMeta the
// transport can write into. The returned pointer is the same one the
// transport observes through context.Value, so reading from it after the
// operation returns yields the captured metadata.
func withMeta(parent context.Context) (context.Context, *respMeta) {
	m := &respMeta{}
	return context.WithValue(parent, metaKey{}, m), m
}

func metaFromCtx(ctx context.Context) *respMeta {
	m, _ := ctx.Value(metaKey{}).(*respMeta)
	return m
}

// requestIDTransport is an http.RoundTripper that captures X-Request-Id
// and the HTTP status code from every response, threading them back to
// the caller via a respMeta pointer in the request context.
//
// The transport never modifies the request. Authentication is handled
// upstream by ogen's SecuritySource mechanism — keeping the two
// concerns separate means each is testable in isolation and the
// transport stays trivial.
type requestIDTransport struct {
	base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *requestIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if resp != nil {
		if m := metaFromCtx(req.Context()); m != nil {
			m.requestID = resp.Header.Get("X-Request-Id")
			m.status = resp.StatusCode
			if resp.StatusCode == http.StatusTooManyRequests {
				m.rateLimit = parseRateLimit(resp, m.requestID)
			}
		}
	}
	return resp, err
}

// parseRateLimit reads the 429 response body, parses the standard error
// envelope, and returns a typed *RateLimitError. The OpenAPI spec does
// not declare 429 responses for any operation, so ogen treats them as
// unexpected status codes — handling them here in the transport is the
// single place every wrapped operation gets the same behavior.
//
// The body is consumed and replaced with http.NoBody so the downstream
// decoder (which is about to fail with UnexpectedStatusCode anyway)
// doesn't observe a half-read stream.
func parseRateLimit(resp *http.Response, requestID string) *RateLimitError {
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(nil))

	var env gen.ErrorEnvelope
	_ = json.Unmarshal(body, &env)

	base := &Error{
		Code:       string(env.Error.Code),
		Message:    env.Error.Message,
		RequestID:  requestID,
		HTTPStatus: http.StatusTooManyRequests,
	}
	if base.Code == "" {
		base.Code = "rate_limited"
	}
	if base.Message == "" {
		base.Message = "rate limit exceeded"
	}

	retryAfter := 0
	if ra, ok := env.Error.RetryAfter.Get(); ok {
		retryAfter = ra
	} else if h := resp.Header.Get("Retry-After"); h != "" {
		// Best-effort fallback to the HTTP header. Numeric seconds only;
		// HTTP-date form is rare in practice and ignored here.
		_ = json.Unmarshal([]byte(h), &retryAfter)
	}

	return &RateLimitError{
		Err:        base,
		RetryAfter: time.Duration(retryAfter) * time.Second,
	}
}
