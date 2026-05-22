package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
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

// debugLogTransport logs full request and response payloads at
// slog.LevelDebug. When the active slog handler is not at debug, it is a
// zero-cost passthrough — no body buffering, no header copying.
//
// The Authorization header value is redacted in logged headers; body
// contents (including any sensitive JSON fields) are emitted verbatim,
// since enabling debug is an explicit opt-in.
type debugLogTransport struct {
	base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *debugLogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	logger := slog.Default()
	if !logger.Enabled(req.Context(), slog.LevelDebug) {
		return base.RoundTrip(req)
	}

	reqBody, err := captureRequestBody(req)
	if err != nil {
		// Don't fail the request because we couldn't capture the body
		// for logging — surface the read error in the log instead.
		logger.LogAttrs(req.Context(), slog.LevelDebug, "http request body capture failed",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.String("error", err.Error()),
		)
	}

	logger.LogAttrs(req.Context(), slog.LevelDebug, "http request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Any("headers", redactHeaders(req.Header)),
		slog.String("body", string(reqBody)),
	)

	start := time.Now()
	resp, err := base.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		logger.LogAttrs(req.Context(), slog.LevelDebug, "http response error",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Int64("duration_ms", elapsed.Milliseconds()),
			slog.String("error", err.Error()),
		)
		return resp, err
	}

	respBody, bodyErr := captureResponseBody(resp)
	if bodyErr != nil {
		logger.LogAttrs(req.Context(), slog.LevelDebug, "http response body capture failed",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.String("error", bodyErr.Error()),
		)
	}

	logger.LogAttrs(req.Context(), slog.LevelDebug, "http response",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Int("status", resp.StatusCode),
		slog.String("request_id", resp.Header.Get("X-Request-Id")),
		slog.Any("headers", redactHeaders(resp.Header)),
		slog.String("body", string(respBody)),
		slog.Int64("duration_ms", elapsed.Milliseconds()),
	)

	return resp, nil
}

// captureRequestBody drains req.Body, restores it with a re-readable
// reader, and also sets req.GetBody so the inner transport can replay
// the body on redirects or retries.
func captureRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	buf, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(nil))
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(buf))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	return buf, nil
}

// captureResponseBody drains resp.Body and restores it with a
// re-readable reader so downstream decoders see the bytes untouched.
func captureResponseBody(resp *http.Response) ([]byte, error) {
	if resp.Body == nil {
		return nil, nil
	}
	buf, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(buf))
	return buf, nil
}

// redactHeaders returns a shallow copy of h with the Authorization
// header value replaced by "[REDACTED]". It uses http.Header's
// canonical-case lookup so it catches "authorization", "AUTHORIZATION",
// etc.
func redactHeaders(h http.Header) http.Header {
	if len(h) == 0 {
		return nil
	}
	out := make(http.Header, len(h))
	for k, v := range h {
		out[k] = v
	}
	if out.Get("Authorization") != "" {
		out.Set("Authorization", "[REDACTED]")
	}
	return out
}
