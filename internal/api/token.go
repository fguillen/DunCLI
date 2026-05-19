// Package api wraps the ogen-generated client in internal/api/gen, adding
// the cross-cutting concerns every later phase consumes: typed error
// envelope decoding, X-Request-Id propagation, lazy bearer-token loading,
// and a per-session entity-name → ULID resolver cache.
package api

import "context"

// TokenProvider supplies the per-request player bearer token.
//
// Implementations are free to read from disk, cache in memory, or refresh
// on demand. The api package calls Token once per outgoing operation, so
// providers should be cheap — back the call with an in-memory cache if a
// disk read is expensive. Returning ("", nil) is valid and signals "no
// credentials available"; the backend will respond with 401, which the
// caller can handle as an unauthenticated state.
//
// The concrete TokenProvider that reads ~/.dun/credentials is implemented
// in Phase 2 (internal/auth). Tests in this package use small inline
// stubs.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken is a TokenProvider that always returns the same token. It
// exists for tests and for one-shot scripts; production code should use
// the Phase 2 credentials-file-backed provider.
type StaticToken string

// Token implements TokenProvider.
func (s StaticToken) Token(context.Context) (string, error) {
	return string(s), nil
}
