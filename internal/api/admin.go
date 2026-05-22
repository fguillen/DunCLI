package api

import (
	"context"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// This file holds the admin-scope wrapper methods — the mirror of the
// player auth/key operations in client.go plus the two list endpoints
// that back the admin resolvers in resolve.go. Phase 14 wires the
// foundation; Phases 15–18 add the admin game verbs that consume it.
//
// Scope is selected by the bearer token, not by a separate client:
// the admin *Client is built with internal/auth.NewAdminFileProvider,
// so ogen's AdminBearer seam yields the admin ApiKey.

// RequestAdminMagicLink enqueues an admin magic-link email for the
// given address. As with the player flow the response shape does not
// reveal whether an Admin already exists — success only means the
// mailer was enqueued.
func (c *Client) RequestAdminMagicLink(ctx context.Context, email string) error {
	return c.call(ctx, gen.RequestAdminMagicLinkOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RequestAdminMagicLink(ctx, &gen.RequestAdminMagicLinkReq{Email: email})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.RequestAdminMagicLinkAccepted:
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.RequestAdminMagicLinkOperation, res)
		},
	)
}

// ExchangeAdminMagicLink consumes an admin magic-link token and returns
// the freshly issued admin ApiKey. The raw key is shown once; the
// caller persists it (scope="admin") or loses it.
func (c *Client) ExchangeAdminMagicLink(ctx context.Context, token string) (*gen.ExchangeResponse, error) {
	var out *gen.ExchangeResponse
	err := c.call(ctx, gen.ExchangeAdminMagicLinkOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ExchangeAdminMagicLink(ctx, &gen.ExchangeAdminMagicLinkReq{Token: token})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ExchangeResponse:
				out = v
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.ExchangeAdminMagicLinkOperation, res)
		},
	)
	return out, err
}

// ListAdminAPIKeys returns every ApiKey the authenticated admin has
// issued (including revoked ones). `current: true` marks the key used
// to authenticate the current request.
func (c *Client) ListAdminAPIKeys(ctx context.Context) ([]gen.ApiKeyEntry, error) {
	var out []gen.ApiKeyEntry
	err := c.call(ctx, gen.ListAdminApiKeysOperation,
		func(ctx context.Context) (any, error) { return c.gen.ListAdminApiKeys(ctx) },
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListAdminApiKeysOK:
				out = v.Keys
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.ListAdminApiKeysOperation, res)
		},
	)
	return out, err
}

// RevokeAdminAPIKey marks the given admin key as revoked. Revoking the
// current key is allowed; subsequent requests with it return 401.
func (c *Client) RevokeAdminAPIKey(ctx context.Context, id string) error {
	return c.call(ctx, gen.RevokeAdminApiKeyOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RevokeAdminApiKey(ctx, gen.RevokeAdminApiKeyParams{ID: id})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.RevokeAdminApiKeyNoContent:
				return nil
			case *gen.RevokeAdminApiKeyNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RevokeAdminApiKeyUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.RevokeAdminApiKeyOperation, res)
		},
	)
}

// ListAdminServers returns the servers the authenticated admin
// administers. Backs ResolveAdminServer.
func (c *Client) ListAdminServers(ctx context.Context) ([]gen.Server, error) {
	var out []gen.Server
	err := c.call(ctx, gen.ListAdminServersOperation,
		func(ctx context.Context) (any, error) { return c.gen.ListAdminServers(ctx) },
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListAdminServersOK:
				out = v.Servers
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.ListAdminServersOperation, res)
		},
	)
	return out, err
}

// ListAdminWorlds returns the worlds on the given server, across every
// status. Backs ResolveAdminWorld.
func (c *Client) ListAdminWorlds(ctx context.Context, serverID string) ([]gen.AdminWorld, error) {
	var out []gen.AdminWorld
	err := c.call(ctx, gen.ListAdminWorldsOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListAdminWorlds(ctx, gen.ListAdminWorldsParams{ServerId: serverID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListAdminWorldsOK:
				out = v.Worlds
				return nil
			case *gen.ListAdminWorldsNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListAdminWorldsUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListAdminWorldsOperation, res)
		},
	)
	return out, err
}
