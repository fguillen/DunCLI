package api

import (
	"context"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// bearerSource adapts a TokenProvider to ogen's gen.SecuritySource.
// ogen calls PlayerBearer (or AdminBearer) once per outgoing operation
// to look up the bearer token; we delegate that to the TokenProvider
// every time so the credentials file can be re-read after a re-login
// without rebuilding the *Client.
//
// Both seams resolve to the same TokenProvider: the active scope is
// decided by which provider the *Client was built with (player vs
// admin — see internal/auth.NewFileProvider / NewAdminFileProvider),
// not by which security seam ogen happens to call. The player `dun>`
// shell and the admin `dun-admin>` shell never share a process, so a
// given *Client only ever issues operations of one scope.
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

// AdminBearer implements gen.SecuritySource for admin-scope endpoints.
func (s bearerSource) AdminBearer(ctx context.Context, _ gen.OperationName) (gen.AdminBearer, error) {
	if s.tp == nil {
		return gen.AdminBearer{}, nil
	}
	tok, err := s.tp.Token(ctx)
	if err != nil {
		return gen.AdminBearer{}, err
	}
	return gen.AdminBearer{Token: tok}, nil
}
