package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// Client is the hand-rolled wrapper around gen.Client. It owns auth,
// error envelope decoding, request-id propagation, and the per-session
// resolver cache. UI code should only ever see this type (or values
// pulled directly from gen schemas like gen.ServerSummary); the raw
// gen.Client and the per-operation `*Res` union types stay private.
type Client struct {
	gen    *gen.Client
	logger *slog.Logger
	cache  *resolverCache
}

// New constructs a Client. baseURL is the API root including the version
// prefix (e.g. "http://localhost:3000/v1"). tp supplies the bearer token
// lazily on each call; pass nil for unauthenticated use (the health
// check and tests). hc is the underlying http.Client; pass nil to use a
// fresh http.Client with default settings. The constructor wraps hc's
// Transport with a requestIDTransport so every response's X-Request-Id
// is captured into the per-operation respMeta.
func New(baseURL string, tp TokenProvider, hc *http.Client) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("api: baseURL is required")
	}

	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	} else {
		// Don't mutate the caller's http.Client — clone it before
		// wrapping the transport.
		clone := *hc
		hc = &clone
	}
	hc.Transport = &requestIDTransport{base: hc.Transport}

	gc, err := gen.NewClient(baseURL, bearerSource{tp: tp}, gen.WithClient(hc))
	if err != nil {
		return nil, fmt.Errorf("api: build gen client: %w", err)
	}

	return &Client{
		gen:    gc,
		logger: slog.Default(),
		cache:  newResolverCache(),
	}, nil
}

// call wraps a single ogen invocation: it installs a respMeta in the
// context, runs op, logs a structured summary, and converts the
// returned Res-union value into either a typed Error or the decoded
// success payload (which the caller's `decode` closure pulls out).
//
// callers pass:
//   - op:    the ogen operation name (for logs)
//   - run:   the closure that issues the request and returns the union
//   - decode: the type-switch that turns the union into either (success, nil),
//     (nil, *Error), or (nil, error) for unexpected types.
//
// We pass closures rather than generics because the response interfaces
// are unrelated types (one per operation), and the per-status error
// types are distinct named aliases of ErrorEnvelope — explicit
// switching reads cleanly and lints clean.
func (c *Client) call(
	ctx context.Context,
	op string,
	run func(ctx context.Context) (any, error),
	decode func(res any, requestID string) error,
) error {
	ctx, meta := withMeta(ctx)
	start := time.Now()

	res, err := run(ctx)

	// 429s are detected in the transport because the OpenAPI spec does
	// not declare a TooManyRequests response for any operation — ogen
	// returns an UnexpectedStatusCode error for the same response that
	// the transport has already turned into a typed *RateLimitError.
	// Prefer the typed form regardless of whether ogen also surfaced an
	// error.
	if meta.rateLimit != nil {
		c.logAPIErr(ctx, op, meta, meta.rateLimit, start)
		return meta.rateLimit
	}

	if err != nil {
		c.logTransportErr(ctx, op, meta, err, start)
		return fmt.Errorf("api %s: %w", op, err)
	}

	if derr := decode(res, meta.requestID); derr != nil {
		c.logAPIErr(ctx, op, meta, derr, start)
		return derr
	}

	c.logger.LogAttrs(ctx, slog.LevelDebug, "api ok",
		slog.String("op", op),
		slog.String("request_id", meta.requestID),
		slog.Int("http_status", meta.status),
		slog.Duration("elapsed", time.Since(start)),
	)
	return nil
}

func (c *Client) logTransportErr(ctx context.Context, op string, m *respMeta, err error, start time.Time) {
	c.logger.LogAttrs(ctx, slog.LevelWarn, "api transport error",
		slog.String("op", op),
		slog.String("request_id", m.requestID),
		slog.Int("http_status", m.status),
		slog.Duration("elapsed", time.Since(start)),
		slog.String("err", err.Error()),
	)
}

func (c *Client) logAPIErr(ctx context.Context, op string, m *respMeta, err error, start time.Time) {
	attrs := []slog.Attr{
		slog.String("op", op),
		slog.String("request_id", m.requestID),
		slog.Int("http_status", m.status),
		slog.Duration("elapsed", time.Since(start)),
		slog.String("err", err.Error()),
	}
	if apiErr := AsError(err); apiErr != nil {
		attrs = append(attrs, slog.String("code", apiErr.Code))
	}
	c.logger.LogAttrs(ctx, slog.LevelWarn, "api error", attrs...)
}

// unexpectedRes is the error returned when ogen hands back a union
// variant that we did not include in the operation's decode switch.
// Hitting this in practice means the OpenAPI spec gained a new response
// status and the spec → generated code → wrapper chain is now out of
// sync.
func unexpectedRes(op string, res any) error {
	return fmt.Errorf("api %s: unexpected response type %T", op, res)
}

// ── Wrapped operations ────────────────────────────────────────────────

// Health calls GET /v1/health. The Phase 3 shell uses it as a
// connectivity probe on startup.
func (c *Client) Health(ctx context.Context) error {
	return c.call(ctx, gen.GetHealthOperation,
		func(ctx context.Context) (any, error) { return c.gen.GetHealth(ctx) },
		func(res any, _ string) error {
			if _, ok := res.(*gen.GetHealthOK); ok {
				return nil
			}
			return unexpectedRes(gen.GetHealthOperation, res)
		},
	)
}

// ListPlayerServers returns the servers the authenticated player is
// admitted to. Backs ResolveServer.
func (c *Client) ListPlayerServers(ctx context.Context) ([]gen.ServerSummary, error) {
	var out []gen.ServerSummary
	err := c.call(ctx, gen.ListPlayerServersOperation,
		func(ctx context.Context) (any, error) { return c.gen.ListPlayerServers(ctx) },
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListPlayerServersOK:
				out = v.Servers
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.ListPlayerServersOperation, res)
		},
	)
	return out, err
}

// ListServerWorlds returns the worlds visible on the given server.
// Backs ResolveWorld.
func (c *Client) ListServerWorlds(ctx context.Context, serverID string) ([]gen.WorldSummary, error) {
	var out []gen.WorldSummary
	err := c.call(ctx, gen.ListServerWorldsOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListServerWorlds(ctx, gen.ListServerWorldsParams{ID: serverID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListServerWorldsOK:
				out = v.Worlds
				return nil
			case *gen.ListServerWorldsNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListServerWorldsUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListServerWorldsOperation, res)
		},
	)
	return out, err
}

// ShowPlayerProfile fetches another player's per-server profile by
// handle. Backs ResolvePlayer.
func (c *Client) ShowPlayerProfile(ctx context.Context, serverID, handle string) (*gen.PlayerProfileRead, error) {
	var out *gen.PlayerProfileRead
	err := c.call(ctx, gen.ShowPlayerProfileOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowPlayerProfile(ctx, gen.ShowPlayerProfileParams{
				ServerId: serverID,
				Handle:   handle,
			})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.PlayerProfileRead:
				out = v
				return nil
			case *gen.ShowPlayerProfileForbidden:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowPlayerProfileNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowPlayerProfileUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowPlayerProfileOperation, res)
		},
	)
	return out, err
}

// ShowWorld fetches world detail including the caller's kingdom summary
// when present. Backs ResolveKingdom (which reads MyKingdom).
func (c *Client) ShowWorld(ctx context.Context, worldID string) (*gen.World, error) {
	var out *gen.World
	err := c.call(ctx, gen.ShowWorldOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowWorld(ctx, gen.ShowWorldParams{ID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.World:
				out = v
				return nil
			case *gen.ShowWorldNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowWorldUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowWorldOperation, res)
		},
	)
	return out, err
}

// ShowWorldMap returns every region for the given world. Backs
// ResolveRegion.
func (c *Client) ShowWorldMap(ctx context.Context, worldID string) ([]gen.RegionSummary, error) {
	var out []gen.RegionSummary
	err := c.call(ctx, gen.ShowWorldMapOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowWorldMap(ctx, gen.ShowWorldMapParams{ID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ShowWorldMapOK:
				out = v.Regions
				return nil
			case *gen.ShowWorldMapNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowWorldMapUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowWorldMapOperation, res)
		},
	)
	return out, err
}

// ListKingdomArmies returns the armies belonging to the given kingdom.
// Backs ResolveArmy.
func (c *Client) ListKingdomArmies(ctx context.Context, kingdomID string) ([]gen.Army, error) {
	var out []gen.Army
	err := c.call(ctx, gen.ListKingdomArmiesOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListKingdomArmies(ctx, gen.ListKingdomArmiesParams{ID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListKingdomArmiesOK:
				out = v.Armies
				return nil
			case *gen.ListKingdomArmiesNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListKingdomArmiesUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListKingdomArmiesOperation, res)
		},
	)
	return out, err
}

// RequestPlayerMagicLink enqueues a player magic-link email for the given
// address. The response shape is identical whether or not a Player
// already exists (no enumeration leak), so success here only means the
// mailer was enqueued.
func (c *Client) RequestPlayerMagicLink(ctx context.Context, email string) error {
	return c.call(ctx, gen.RequestPlayerMagicLinkOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RequestPlayerMagicLink(ctx, &gen.RequestPlayerMagicLinkReq{Email: email})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.RequestPlayerMagicLinkAccepted:
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.RequestPlayerMagicLinkOperation, res)
		},
	)
}

// ExchangePlayerMagicLink consumes a magic-link token and returns the
// freshly issued player ApiKey. The raw key is shown once; the caller is
// responsible for persisting it (or losing it).
func (c *Client) ExchangePlayerMagicLink(ctx context.Context, token string) (*gen.ExchangeResponse, error) {
	var out *gen.ExchangeResponse
	err := c.call(ctx, gen.ExchangePlayerMagicLinkOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ExchangePlayerMagicLink(ctx, &gen.ExchangePlayerMagicLinkReq{Token: token})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ExchangeResponse:
				out = v
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.ExchangePlayerMagicLinkOperation, res)
		},
	)
	return out, err
}

// ListPlayerAPIKeys returns every ApiKey the authenticated player has
// issued (including revoked ones). `current: true` marks the key used to
// authenticate the current request — that's how the CLI discovers its
// own key id for `dun logout` and "is this the current one?" checks.
func (c *Client) ListPlayerAPIKeys(ctx context.Context) ([]gen.ApiKeyEntry, error) {
	var out []gen.ApiKeyEntry
	err := c.call(ctx, gen.ListPlayerApiKeysOperation,
		func(ctx context.Context) (any, error) { return c.gen.ListPlayerApiKeys(ctx) },
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListPlayerApiKeysOK:
				out = v.Keys
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.ListPlayerApiKeysOperation, res)
		},
	)
	return out, err
}

// RevokePlayerAPIKey marks the given key as revoked. Revoking the
// current key is allowed; subsequent requests with it return 401.
func (c *Client) RevokePlayerAPIKey(ctx context.Context, id string) error {
	return c.call(ctx, gen.RevokePlayerApiKeyOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RevokePlayerApiKey(ctx, gen.RevokePlayerApiKeyParams{ID: id})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.RevokePlayerApiKeyNoContent:
				return nil
			case *gen.RevokePlayerApiKeyNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RevokePlayerApiKeyUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.RevokePlayerApiKeyOperation, res)
		},
	)
}

// JoinServer joins the caller to the given server. On success the
// server cache is invalidated so the next ResolveServer / membership
// lookup reflects the new membership.
func (c *Client) JoinServer(ctx context.Context, serverID string) (*gen.JoinServerCreated, error) {
	var out *gen.JoinServerCreated
	err := c.call(ctx, gen.JoinServerOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.JoinServer(ctx, gen.JoinServerParams{ID: serverID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.JoinServerCreated:
				out = v
				return nil
			case *gen.JoinServerForbidden:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.JoinServerNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.JoinServerUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.JoinServerOperation, res)
		},
	)
	if err == nil {
		c.InvalidateServers()
	}
	return out, err
}

// ProfileUpdate carries the optional fields PATCH /servers/{id}/me
// accepts. Either field may be nil to leave it untouched; either may
// point at an empty string to clear the server-side value.
type ProfileUpdate struct {
	Handle   *string
	RealName *string
}

// UpdateOwnProfile updates the caller's profile on the given server.
// Surfaces the backend's `handle_locked` (§17.1) verbatim — callers are
// expected to render the error untouched.
func (c *Client) UpdateOwnProfile(ctx context.Context, serverID string, in ProfileUpdate) (*gen.PlayerProfileWrite, error) {
	req := gen.UpdateOwnProfileReq{}
	if in.Handle != nil {
		req.Handle = gen.NewOptString(*in.Handle)
	}
	if in.RealName != nil {
		req.RealName = gen.NewOptString(*in.RealName)
	}

	var out *gen.PlayerProfileWrite
	err := c.call(ctx, gen.UpdateOwnProfileOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.UpdateOwnProfile(ctx, gen.NewOptUpdateOwnProfileReq(req),
				gen.UpdateOwnProfileParams{ID: serverID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.PlayerProfileWrite:
				out = v
				return nil
			case *gen.UpdateOwnProfileNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.UpdateOwnProfileUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.UpdateOwnProfileUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.UpdateOwnProfileOperation, res)
		},
	)
	return out, err
}

// DeleteAccount irreversibly deletes the caller's account. All ApiKeys
// are revoked server-side as part of the same operation, so the caller
// should also wipe its local credentials on success.
func (c *Client) DeleteAccount(ctx context.Context) error {
	return c.call(ctx, gen.DeleteAccountOperation,
		func(ctx context.Context) (any, error) { return c.gen.DeleteAccount(ctx) },
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.DeleteAccountNoContent:
				return nil
			case *gen.ErrorEnvelope:
				return fromEnvelope(v, rid)
			}
			return unexpectedRes(gen.DeleteAccountOperation, res)
		},
	)
}
