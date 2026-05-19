package api

import (
	"context"
	"errors"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// bearerSource adapts a TokenProvider to ogen's gen.SecuritySource.
// ogen calls PlayerBearer (or AdminBearer) once per outgoing operation
// to look up the bearer token; we delegate that to the TokenProvider
// every time so the credentials file can be re-read after a re-login
// without rebuilding the *Client.
type bearerSource struct{ tp TokenProvider }

// PlayerBearer implements gen.SecuritySource for player-scope endpoints.
func (s bearerSource) PlayerBearer(ctx context.Context, _ gen.OperationName) (gen.PlayerBearer, error) {
	if s.tp == nil {
		return gen.PlayerBearer{}, nil
	}
	tok, err := s.tp.Token(ctx)
	if err != nil {
		return gen.PlayerBearer{}, err
	}
	return gen.PlayerBearer{Token: tok}, nil
}

// AdminBearer implements gen.SecuritySource. v1 is player-surface only; if
// the spec ever routes us through an admin operation, fail loudly rather
// than silently sending an empty token.
func (s bearerSource) AdminBearer(context.Context, gen.OperationName) (gen.AdminBearer, error) {
	return gen.AdminBearer{}, errors.New("api: admin scope not supported in v1")
}
