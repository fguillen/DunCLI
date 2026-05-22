package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// httpClient + baseURL + tp are retained so a small number of
	// wrappers can bypass ogen for endpoints whose OpenAPI shape ogen
	// cannot decode cleanly. Today only GetWonder uses these, because
	// its `oneOf: [Wonder, {wonder: null}]` 200 body trips ogen's
	// sum-type discriminator when the "no wonder" branch is hit. The
	// existing requestIDTransport stays on httpClient, so X-Request-Id
	// capture and 429 handling work identically to the ogen path.
	httpClient *http.Client
	baseURL    string
	tp         TokenProvider
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
	hc.Transport = &debugLogTransport{base: &requestIDTransport{base: hc.Transport}}

	gc, err := gen.NewClient(baseURL, bearerSource{tp: tp}, gen.WithClient(hc))
	if err != nil {
		return nil, fmt.Errorf("api: build gen client: %w", err)
	}

	return &Client{
		gen:        gc,
		logger:     slog.Default(),
		cache:      newResolverCache(),
		httpClient: hc,
		baseURL:    baseURL,
		tp:         tp,
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

// ShowOwnProfile fetches the caller's own per-server profile. Unlike
// ShowPlayerProfile it needs no handle — the backend resolves the
// profile from the authenticated player. A handle_not_set / not_found
// envelope means the caller has joined the server but not yet picked a
// handle (or has not joined it at all).
func (c *Client) ShowOwnProfile(ctx context.Context, serverID string) (*gen.PlayerProfileRead, error) {
	var out *gen.PlayerProfileRead
	err := c.call(ctx, gen.ShowOwnProfileOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowOwnProfile(ctx, gen.ShowOwnProfileParams{ID: serverID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.PlayerProfileRead:
				out = v
				return nil
			case *gen.ShowOwnProfileNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowOwnProfileUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowOwnProfileOperation, res)
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

// JoinWorld joins the caller to the given world. The returned Kingdom
// is a stub during `proposed` (no home_region_id yet) and a fully
// spawned kingdom during `grace`. On success the per-server world cache
// is invalidated so the next ListServerWorlds/ResolveWorld reflects the
// new membership.
func (c *Client) JoinWorld(ctx context.Context, worldID, serverID string) (*gen.Kingdom, error) {
	var out *gen.Kingdom
	err := c.call(ctx, gen.JoinWorldOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.JoinWorld(ctx, gen.JoinWorldParams{ID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Kingdom:
				out = v
				return nil
			case *gen.JoinWorldForbidden:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.JoinWorldNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.JoinWorldUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.JoinWorldUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.JoinWorldOperation, res)
		},
	)
	if err == nil {
		c.InvalidateWorlds(serverID)
	}
	return out, err
}

// ShowRegion fetches one region's full detail (nodes, ruin, adjacency).
func (c *Client) ShowRegion(ctx context.Context, worldID, regionID string) (*gen.Region, error) {
	var out *gen.Region
	err := c.call(ctx, gen.ShowRegionOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowRegion(ctx, gen.ShowRegionParams{WorldID: worldID, ID: regionID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Region:
				out = v
				return nil
			case *gen.ShowRegionNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowRegionUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowRegionOperation, res)
		},
	)
	return out, err
}

// ShowRegionAdjacent fetches the lean (id, name, terrain) descriptors
// of every region adjacent to the given region.
func (c *Client) ShowRegionAdjacent(ctx context.Context, worldID, regionID string) ([]gen.ShowRegionAdjacentOKRegionsItem, error) {
	var out []gen.ShowRegionAdjacentOKRegionsItem
	err := c.call(ctx, gen.ShowRegionAdjacentOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowRegionAdjacent(ctx, gen.ShowRegionAdjacentParams{WorldID: worldID, ID: regionID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ShowRegionAdjacentOK:
				out = v.Regions
				return nil
			case *gen.ShowRegionAdjacentNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowRegionAdjacentUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowRegionAdjacentOperation, res)
		},
	)
	return out, err
}

// ListRuins returns every ruin in the given world (claimed and not).
func (c *Client) ListRuins(ctx context.Context, worldID string) ([]gen.Ruin, error) {
	var out []gen.Ruin
	err := c.call(ctx, gen.ListRuinsOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListRuins(ctx, gen.ListRuinsParams{WorldID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListRuinsOK:
				out = v.Ruins
				return nil
			case *gen.ListRuinsNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListRuinsUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListRuinsOperation, res)
		},
	)
	return out, err
}

// ListNodes returns every node in the world (wilderness, captured, and
// home-hoard).
func (c *Client) ListNodes(ctx context.Context, worldID string) ([]gen.Node, error) {
	var out []gen.Node
	err := c.call(ctx, gen.ListNodesOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListNodes(ctx, gen.ListNodesParams{WorldID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListNodesOK:
				out = v.Nodes
				return nil
			case *gen.ListNodesNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListNodesUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListNodesOperation, res)
		},
	)
	return out, err
}

// ShowNode fetches one node's detail by ULID.
func (c *Client) ShowNode(ctx context.Context, worldID, nodeID string) (*gen.Node, error) {
	var out *gen.Node
	err := c.call(ctx, gen.ShowNodeOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowNode(ctx, gen.ShowNodeParams{WorldID: worldID, ID: nodeID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ShowNodeOK:
				out = &v.Node
				return nil
			case *gen.ShowNodeNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowNodeUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowNodeOperation, res)
		},
	)
	return out, err
}

// ShowKingdom returns the caller's kingdom dashboard: materialized
// stockpiles, production rates, buildings, and in-progress orders. The
// backend returns 404 to non-owners, so this is implicitly an "is this
// my kingdom?" check too.
func (c *Client) ShowKingdom(ctx context.Context, kingdomID string) (*gen.KingdomDetail, error) {
	var out *gen.KingdomDetail
	err := c.call(ctx, gen.ShowKingdomOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowKingdom(ctx, gen.ShowKingdomParams{ID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.KingdomDetail:
				out = v
				return nil
			case *gen.ShowKingdomNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowKingdomUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowKingdomOperation, res)
		},
	)
	return out, err
}

// ListKingdomBuildings returns one entry per building kind with the
// same upgrade-preview detail as PreviewBuildUpgrade plus the active
// BuildOrder for that building (if any) and a derived
// `upgrade_possible` flag. When upgradable is non-nil and true, the
// backend trims the response to upgradable rows.
func (c *Client) ListKingdomBuildings(ctx context.Context, kingdomID string, upgradable *bool) (*gen.KingdomBuildingsList, error) {
	params := gen.ListKingdomBuildingsParams{KingdomID: kingdomID}
	if upgradable != nil {
		v := gen.ListKingdomBuildingsUpgradePossibleFalse
		if *upgradable {
			v = gen.ListKingdomBuildingsUpgradePossibleTrue
		}
		params.UpgradePossible = gen.NewOptListKingdomBuildingsUpgradePossible(v)
	}
	var out *gen.KingdomBuildingsList
	err := c.call(ctx, gen.ListKingdomBuildingsOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListKingdomBuildings(ctx, params)
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.KingdomBuildingsList:
				out = v
				return nil
			case *gen.ListKingdomBuildingsNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListKingdomBuildingsUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListKingdomBuildingsOperation, res)
		},
	)
	return out, err
}

// PreviewBuildUpgrade returns cost, duration, tier-gate status, and
// affordability for the next level of the given building. The backend
// reports cost regardless of world status or queue slot — the actual
// QueueBuildOrder still enforces those at commit time.
func (c *Client) PreviewBuildUpgrade(ctx context.Context, kingdomID, building string) (*gen.BuildingUpgradePreview, error) {
	var out *gen.BuildingUpgradePreview
	err := c.call(ctx, gen.PreviewBuildUpgradeOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.PreviewBuildUpgrade(ctx, gen.PreviewBuildUpgradeParams{
				ID:       kingdomID,
				Building: gen.PreviewBuildUpgradeBuilding(building),
			})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.BuildingUpgradePreview:
				out = v
				return nil
			case *gen.PreviewBuildUpgradeNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.PreviewBuildUpgradeUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.PreviewBuildUpgradeUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.PreviewBuildUpgradeOperation, res)
		},
	)
	return out, err
}

// QueueBuildOrder enqueues an upgrade for the named building. The
// backend enforces `target_level == current_level + 1` as a defensive
// concurrency check; callers should derive it from PreviewBuildUpgrade
// (or ListKingdomBuildings) rather than guessing. On success the
// kingdom cache is invalidated.
func (c *Client) QueueBuildOrder(ctx context.Context, kingdomID, building string, targetLevel int) (*gen.BuildOrder, error) {
	req := &gen.QueueBuildOrderReq{
		Building:    gen.QueueBuildOrderReqBuilding(building),
		TargetLevel: targetLevel,
	}
	var out *gen.BuildOrder
	err := c.call(ctx, gen.QueueBuildOrderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.QueueBuildOrder(ctx, req, gen.QueueBuildOrderParams{ID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.BuildOrder:
				out = v
				return nil
			case *gen.QueueBuildOrderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.QueueBuildOrderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.QueueBuildOrderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.QueueBuildOrderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// CancelBuildOrder cancels an in-progress build order. The backend
// refunds 75% of the spent resources (elapsed time is lost) and frees
// the build slot.
func (c *Client) CancelBuildOrder(ctx context.Context, kingdomID, orderID string) (*gen.BuildOrder, error) {
	var out *gen.BuildOrder
	err := c.call(ctx, gen.CancelBuildOrderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.CancelBuildOrder(ctx, gen.CancelBuildOrderParams{KingdomID: kingdomID, ID: orderID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.BuildOrder:
				out = v
				return nil
			case *gen.CancelBuildOrderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.CancelBuildOrderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.CancelBuildOrderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.CancelBuildOrderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// PreviewTrainingOrder returns cost, duration, affordability, and the
// max-affordable count for training `count` of `unit` at the given
// `building` in the caller's kingdom. Like the build preview it does
// not commit anything; the queue path still enforces the gates at
// commit time.
func (c *Client) PreviewTrainingOrder(ctx context.Context, kingdomID, building, unit string, count int) (*gen.TrainingPreview, error) {
	var out *gen.TrainingPreview
	err := c.call(ctx, gen.PreviewTrainingOrderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.PreviewTrainingOrder(ctx, gen.PreviewTrainingOrderParams{
				ID:       kingdomID,
				Building: gen.PreviewTrainingOrderBuilding(building),
				Unit:     gen.Unit(unit),
				Count:    count,
			})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.TrainingPreview:
				out = v
				return nil
			case *gen.PreviewTrainingOrderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.PreviewTrainingOrderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.PreviewTrainingOrderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.PreviewTrainingOrderOperation, res)
		},
	)
	return out, err
}

// TrainingCatalog returns, for one military building or for all three,
// the units it can train with per-unit cost, per-unit time, and the
// count the kingdom's current stockpile affords. Pass an empty
// `building` for all three buildings. Read-only — like the build and
// train previews it commits nothing.
func (c *Client) TrainingCatalog(ctx context.Context, kingdomID, building string) (*gen.TrainingCatalog, error) {
	params := gen.TrainingCatalogParams{ID: kingdomID}
	if building != "" {
		params.Building = gen.NewOptTrainingCatalogBuilding(gen.TrainingCatalogBuilding(building))
	}
	var out *gen.TrainingCatalog
	err := c.call(ctx, gen.TrainingCatalogOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.TrainingCatalog(ctx, params)
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.TrainingCatalog:
				out = v
				return nil
			case *gen.TrainingCatalogNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.TrainingCatalogUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.TrainingCatalogUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.TrainingCatalogOperation, res)
		},
	)
	return out, err
}

// QueueTrainingOrder enqueues `count` of `unit` at `building`. The
// backend enforces per-building FIFO. On success the kingdom cache is
// invalidated so the next `kingdom` reflects the deducted stockpile.
func (c *Client) QueueTrainingOrder(ctx context.Context, kingdomID, building, unit string, count int) (*gen.TrainingOrder, error) {
	req := &gen.QueueTrainingOrderReq{
		Building: gen.QueueTrainingOrderReqBuilding(building),
		Unit:     gen.Unit(unit),
		Count:    count,
	}
	var out *gen.TrainingOrder
	err := c.call(ctx, gen.QueueTrainingOrderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.QueueTrainingOrder(ctx, req, gen.QueueTrainingOrderParams{ID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.TrainingOrder:
				out = v
				return nil
			case *gen.QueueTrainingOrderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.QueueTrainingOrderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.QueueTrainingOrderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.QueueTrainingOrderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// CancelTrainingOrder cancels an in-progress training order. The
// backend refunds 75% of the spent resources (elapsed time is lost).
func (c *Client) CancelTrainingOrder(ctx context.Context, kingdomID, orderID string) (*gen.TrainingOrder, error) {
	var out *gen.TrainingOrder
	err := c.call(ctx, gen.CancelTrainingOrderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.CancelTrainingOrder(ctx, gen.CancelTrainingOrderParams{KingdomID: kingdomID, ID: orderID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.TrainingOrder:
				out = v
				return nil
			case *gen.CancelTrainingOrderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.CancelTrainingOrderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.CancelTrainingOrderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.CancelTrainingOrderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// ShowArmy fetches one army by ULID. The backend returns 404 to
// non-owners so this implicitly serves as an "is this my army?" check.
func (c *Client) ShowArmy(ctx context.Context, armyID string) (*gen.Army, error) {
	var out *gen.Army
	err := c.call(ctx, gen.ShowArmyOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowArmy(ctx, gen.ShowArmyParams{ID: armyID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Army:
				out = v
				return nil
			case *gen.ShowArmyNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowArmyUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowArmyOperation, res)
		},
	)
	return out, err
}

// SplitArmy peels `units` off the source army (`armyID`) into a fresh
// army named `name`. The response carries the post-split source (or
// null if it was emptied) plus the new army. On success the caller's
// army cache is invalidated.
func (c *Client) SplitArmy(ctx context.Context, armyID, name string, units map[string]int) (*gen.SplitArmyCreated, error) {
	req := &gen.SplitArmyReq{
		Name:  name,
		Units: gen.SplitArmyReqUnits(units),
	}
	var out *gen.SplitArmyCreated
	err := c.call(ctx, gen.SplitArmyOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.SplitArmy(ctx, req, gen.SplitArmyParams{ID: armyID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.SplitArmyCreated:
				out = v
				return nil
			case *gen.SplitArmyNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.SplitArmyUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.SplitArmyUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.SplitArmyOperation, res)
		},
	)
	if err == nil && out != nil {
		c.InvalidateArmies(out.New.KingdomID)
	}
	return out, err
}

// RenameArmy changes an army's display name. The backend enforces
// per-kingdom uniqueness and a 60-char cap, returning 422 `name_taken`
// for duplicates.
func (c *Client) RenameArmy(ctx context.Context, armyID, name string) (*gen.Army, error) {
	req := &gen.RenameArmyReq{Name: name}
	var out *gen.Army
	err := c.call(ctx, gen.RenameArmyOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RenameArmy(ctx, req, gen.RenameArmyParams{ID: armyID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Army:
				out = v
				return nil
			case *gen.RenameArmyNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RenameArmyUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RenameArmyUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.RenameArmyOperation, res)
		},
	)
	if err == nil && out != nil {
		c.InvalidateArmies(out.KingdomID)
	}
	return out, err
}

// MergeArmy merges `sourceArmyID` into `targetArmyID`. Both armies
// must be in the same kingdom + region and both `home`; the backend
// returns 422 `incompatible_armies` otherwise.
func (c *Client) MergeArmy(ctx context.Context, targetArmyID, sourceArmyID string) (*gen.Army, error) {
	req := &gen.MergeArmyReq{FromID: sourceArmyID}
	var out *gen.Army
	err := c.call(ctx, gen.MergeArmyOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.MergeArmy(ctx, req, gen.MergeArmyParams{ID: targetArmyID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Army:
				out = v
				return nil
			case *gen.MergeArmyNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.MergeArmyUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.MergeArmyUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.MergeArmyOperation, res)
		},
	)
	if err == nil && out != nil {
		c.InvalidateArmies(out.KingdomID)
	}
	return out, err
}

// DispatchMarch sends `armyID` toward `targetRegionID` with the given
// intent (one of attack/reinforce/scout/capture/claim_ruin/caravan).
// The backend computes shortest path + ETA per §16.10.
func (c *Client) DispatchMarch(ctx context.Context, armyID, targetRegionID, intent string) (*gen.MarchOrder, error) {
	req := &gen.DispatchMarchReq{
		TargetRegionID: targetRegionID,
		Intent:         gen.DispatchMarchReqIntent(intent),
	}
	var out *gen.MarchOrder
	err := c.call(ctx, gen.DispatchMarchOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.DispatchMarch(ctx, req, gen.DispatchMarchParams{ID: armyID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.MarchOrder:
				out = v
				return nil
			case *gen.DispatchMarchNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.DispatchMarchUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.DispatchMarchUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.DispatchMarchOperation, res)
		},
	)
	return out, err
}

// RecallMarch turns the army's in-flight march into a return march
// with intent `reinforce` (v1 simplification — elapsed-time return,
// no per-leg position tracking, no unit losses). Returns a typed
// "no active march" error when the backend responds 404.
func (c *Client) RecallMarch(ctx context.Context, armyID string) (*gen.MarchOrder, error) {
	var out *gen.MarchOrder
	err := c.call(ctx, gen.RecallMarchOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RecallMarch(ctx, gen.RecallMarchParams{ID: armyID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.MarchOrder:
				out = v
				return nil
			case *gen.RecallMarchNotFound:
				return &Error{
					Code:       "not_found",
					Message:    "no active march for this army",
					RequestID:  rid,
					HTTPStatus: 404,
				}
			case *gen.RecallMarchUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RecallMarchUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.RecallMarchOperation, res)
		},
	)
	return out, err
}

// ListKingdomBattles returns the battle history for the given kingdom
// (attacker or defender), newest first. `limit` <= 0 falls through to
// the spec default (25); `offset` <= 0 means no offset. Returns the
// page slice and the server's total_count so callers can render
// pagination footers.
func (c *Client) ListKingdomBattles(ctx context.Context, kingdomID string, limit, offset int) ([]gen.Battle, int, error) {
	params := gen.ListKingdomBattlesParams{KingdomID: kingdomID}
	if limit > 0 {
		params.Limit = gen.OptInt{Value: limit, Set: true}
	}
	if offset > 0 {
		params.Offset = gen.OptInt{Value: offset, Set: true}
	}
	var battles []gen.Battle
	var total int
	err := c.call(ctx, gen.ListKingdomBattlesOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListKingdomBattles(ctx, params)
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListKingdomBattlesOK:
				battles = v.Battles
				total = v.TotalCount
				return nil
			case *gen.ListKingdomBattlesNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListKingdomBattlesUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListKingdomBattlesOperation, res)
		},
	)
	return battles, total, err
}

// ShowBattle fetches one battle by ULID, plus its participant snapshots.
// The backend returns 404 to anyone who isn't the attacker or defender
// kingdom owner, so this implicitly serves as an "is this my battle?"
// check.
func (c *Client) ShowBattle(ctx context.Context, battleID string) (*gen.Battle, []gen.BattleParticipant, error) {
	var battle *gen.Battle
	var participants []gen.BattleParticipant
	err := c.call(ctx, gen.ShowBattleOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ShowBattle(ctx, gen.ShowBattleParams{ID: battleID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ShowBattleOK:
				b := v.Battle
				battle = &b
				participants = v.Participants
				return nil
			case *gen.ShowBattleNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ShowBattleUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ShowBattleOperation, res)
		},
	)
	return battle, participants, err
}

// DispatchCaravan splits `escortUnits` off the home army `sourceArmyID`,
// deducts `payload` from the sender's stockpile, and dispatches a march
// with intent `caravan` toward the receiver's home region. On arrival
// the backend either delivers (transferring resources to the receiver's
// stockpile, warehouse-capped) or — if a hostile third-party army is
// camped at the destination — runs the interception combat. Outcomes
// surface in the Phase 9 battle stream and the world's trade ledger.
func (c *Client) DispatchCaravan(ctx context.Context, kingdomID, receiverHandle, sourceArmyID string, payload, escortUnits map[string]int) (*gen.Caravan, error) {
	req := &gen.DispatchCaravanReq{
		ReceiverHandle: receiverHandle,
		SourceArmyID:   sourceArmyID,
		Payload:        gen.DispatchCaravanReqPayload(payload),
		EscortUnits:    gen.DispatchCaravanReqEscortUnits(escortUnits),
	}
	var out *gen.Caravan
	err := c.call(ctx, gen.DispatchCaravanOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.DispatchCaravan(ctx, req, gen.DispatchCaravanParams{KingdomID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Caravan:
				out = v
				return nil
			case *gen.DispatchCaravanNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.DispatchCaravanUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.DispatchCaravanUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.DispatchCaravanOperation, res)
		},
	)
	if err == nil && out != nil {
		// Source army composition changed; invalidate the caller's army cache.
		c.InvalidateArmies(kingdomID)
	}
	return out, err
}

// ListTradeLedger returns the world's trade ledger (one row per
// non-zero resource per caravan), newest first. `limit` <= 0 falls
// through to the spec default (25); `page` <= 0 means page 1. Returns
// the page slice and the pagy meta so callers can render pagination
// footers.
func (c *Client) ListTradeLedger(ctx context.Context, worldID string, player, since string, limit, page int) ([]gen.TradeLedgerEntry, gen.ListTradeLedgerOKPagy, error) {
	params := gen.ListTradeLedgerParams{WorldID: worldID}
	if player != "" {
		params.Player = gen.OptString{Value: player, Set: true}
	}
	if since != "" {
		params.Since = gen.OptString{Value: since, Set: true}
	}
	if limit > 0 {
		params.Limit = gen.OptInt{Value: limit, Set: true}
	}
	if page > 0 {
		params.Page = gen.OptInt{Value: page, Set: true}
	}
	var entries []gen.TradeLedgerEntry
	var pagy gen.ListTradeLedgerOKPagy
	err := c.call(ctx, gen.ListTradeLedgerOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListTradeLedger(ctx, params)
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListTradeLedgerOK:
				entries = v.Entries
				pagy = v.Pagy
				return nil
			case *gen.ListTradeLedgerNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListTradeLedgerUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListTradeLedgerOperation, res)
		},
	)
	return entries, pagy, err
}

// GetWonder fetches the caller's wonder for the given kingdom. The
// backend lazy-applies construction (`Wonders::ApplyConstruction`)
// before serializing, so the returned HP / paused_until reflects the
// current moment. The 200 response is a `oneOf`: either a Wonder
// payload (kingdom has a live wonder) or `{wonder: null}` (no wonder
// under construction). The wrapper collapses the latter to (nil, nil)
// so UI code can render a "no wonder" message.
//
// This call bypasses ogen for one reason: the spec's `oneOf` shape on
// the 200 response has no usable discriminator when the "no wonder"
// branch is hit — ogen's generated sum-type decoder fails with
// "unable to detect sum type variant" on a literal `{"wonder": null}`
// body. Backend co-evolution candidate: flatten the response to a
// single object with a nullable `wonder` field (no `oneOf`) so ogen
// can decode it directly. The raw call still goes through the shared
// requestIDTransport for X-Request-Id capture and 429 handling.
func (c *Client) GetWonder(ctx context.Context, kingdomID string) (*gen.Wonder, error) {
	ctx, meta := withMeta(ctx)
	start := time.Now()

	req, err := c.newRawRequest(ctx, http.MethodGet,
		fmt.Sprintf("/kingdoms/%s/wonder", kingdomID), nil)
	if err != nil {
		return nil, fmt.Errorf("api %s: %w", gen.GetWonderOperation, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logTransportErr(ctx, gen.GetWonderOperation, meta, err, start)
		return nil, fmt.Errorf("api %s: %w", gen.GetWonderOperation, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if meta.rateLimit != nil {
		c.logAPIErr(ctx, gen.GetWonderOperation, meta, meta.rateLimit, start)
		return nil, meta.rateLimit
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logTransportErr(ctx, gen.GetWonderOperation, meta, err, start)
		return nil, fmt.Errorf("api %s: %w", gen.GetWonderOperation, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// Peek the body: a wonder payload has top-level `id`/`name`/`hp`
		// fields; the "no wonder" payload is `{"wonder": ...}` (and the
		// inner value is null today). Distinguish on the presence of the
		// `wonder` envelope key.
		var envelope struct {
			Wonder *json.RawMessage `json:"wonder"`
		}
		_ = json.Unmarshal(body, &envelope)
		// If the parsed envelope has exactly the `wonder` key set (with
		// any value, null or otherwise), there is no live wonder. The
		// spec's example pins value=null; we accept anything to stay
		// forward-compatible.
		if isWonderNullEnvelope(body) {
			c.logger.LogAttrs(ctx, slog.LevelDebug, "api ok",
				slog.String("op", gen.GetWonderOperation),
				slog.String("request_id", meta.requestID),
				slog.Int("http_status", meta.status),
				slog.Duration("elapsed", time.Since(start)),
			)
			return nil, nil
		}
		var w gen.Wonder
		if err := json.Unmarshal(body, &w); err != nil {
			c.logTransportErr(ctx, gen.GetWonderOperation, meta,
				fmt.Errorf("decode wonder: %w", err), start)
			return nil, fmt.Errorf("api %s: decode wonder: %w",
				gen.GetWonderOperation, err)
		}
		c.logger.LogAttrs(ctx, slog.LevelDebug, "api ok",
			slog.String("op", gen.GetWonderOperation),
			slog.String("request_id", meta.requestID),
			slog.Int("http_status", meta.status),
			slog.Duration("elapsed", time.Since(start)),
		)
		return &w, nil

	case http.StatusUnauthorized, http.StatusNotFound:
		var env gen.ErrorEnvelope
		_ = json.Unmarshal(body, &env)
		derr := fromEnvelope(&env, meta.requestID)
		c.logAPIErr(ctx, gen.GetWonderOperation, meta, derr, start)
		return nil, derr
	}

	derr := fmt.Errorf("api %s: unexpected status %d", gen.GetWonderOperation, resp.StatusCode)
	c.logTransportErr(ctx, gen.GetWonderOperation, meta, derr, start)
	return nil, derr
}

// newRawRequest builds a request scoped to the configured base URL and
// installs the bearer token from c.tp (when present). The
// requestIDTransport on c.httpClient handles X-Request-Id + 429
// capture, so callers using this helper get the same observability as
// ogen-routed wrappers.
func (c *Client) newRawRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if c.tp != nil {
		tok, err := c.tp.Token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// isWonderNullEnvelope reports whether body is the spec's "no wonder"
// envelope shape — an object whose only top-level key is `wonder`,
// with a null (or otherwise non-Wonder) value. Distinguishes from a
// real Wonder payload which has many top-level fields including `id`,
// `kingdom_id`, `name`, `status`, `hp`.
func isWonderNullEnvelope(body []byte) bool {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return false
	}
	if _, hasID := obj["id"]; hasID {
		return false
	}
	_, hasWonder := obj["wonder"]
	return hasWonder
}

// StartWonder begins wonder construction in the caller's kingdom. The
// backend validates §14 prerequisites (building gates, ≥3 owned nodes,
// no live wonder), deducts the 25% foundation payment, and locks the
// build queue. On success the kingdom cache is invalidated so the next
// `kingdom` reflects the deducted stockpile.
func (c *Client) StartWonder(ctx context.Context, kingdomID, name string) (*gen.Wonder, error) {
	req := &gen.WonderStartRequest{Name: gen.WonderStartRequestName(name)}
	var out *gen.Wonder
	err := c.call(ctx, gen.StartWonderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.StartWonder(ctx, req, gen.StartWonderParams{KingdomID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Wonder:
				out = v
				return nil
			case *gen.StartWonderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.StartWonderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.StartWonderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.StartWonderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// CancelWonder abandons the caller's wonder. Paid resources are lost
// and the build queue unlocks. The returned Wonder is the now-destroyed
// snapshot.
func (c *Client) CancelWonder(ctx context.Context, kingdomID string) (*gen.Wonder, error) {
	var out *gen.Wonder
	err := c.call(ctx, gen.CancelWonderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.CancelWonder(ctx, gen.CancelWonderParams{KingdomID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Wonder:
				out = v
				return nil
			case *gen.CancelWonderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.CancelWonderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.CancelWonderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// RepairWonder spends Stone to restore wonder HP (1 HP per 8 Stone).
// The backend clamps `hp` to the per-phase 2000 HP cap (foundation /
// construction / consecration each track independently); the returned
// Wonder reflects the actual repair plus any construction pause from
// the 30 min/500 HP rule.
func (c *Client) RepairWonder(ctx context.Context, kingdomID string, hp int) (*gen.Wonder, error) {
	req := &gen.WonderRepairRequest{Hp: hp}
	var out *gen.Wonder
	err := c.call(ctx, gen.RepairWonderOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.RepairWonder(ctx, req, gen.RepairWonderParams{KingdomID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Wonder:
				out = v
				return nil
			case *gen.RepairWonderNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RepairWonderUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.RepairWonderUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.RepairWonderOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// PayWonderMilestone deducts the 10% milestone payment (25%, 50%, or
// 75%) and resumes construction. The backend rejects calls when no
// milestone is pending or when the percent doesn't match the active
// threshold.
func (c *Client) PayWonderMilestone(ctx context.Context, kingdomID string, percent int) (*gen.Wonder, error) {
	req := &gen.WonderMilestoneRequest{Percent: gen.WonderMilestoneRequestPercent(percent)}
	var out *gen.Wonder
	err := c.call(ctx, gen.PayWonderMilestoneOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.PayWonderMilestone(ctx, req, gen.PayWonderMilestoneParams{KingdomID: kingdomID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.Wonder:
				out = v
				return nil
			case *gen.PayWonderMilestoneNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.PayWonderMilestoneUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.PayWonderMilestoneUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.PayWonderMilestoneOperation, res)
		},
	)
	if err == nil {
		c.InvalidateKingdom(kingdomID)
	}
	return out, err
}

// ListWorldWonders returns every wonder in the in-scope world (the
// public, server-wide view used by the plural `wonders` verb). One row
// per wonder, ordered by creation time.
func (c *Client) ListWorldWonders(ctx context.Context, worldID string) ([]gen.WonderListItem, error) {
	var out []gen.WonderListItem
	err := c.call(ctx, gen.ListWorldWondersOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.ListWorldWonders(ctx, gen.ListWorldWondersParams{WorldID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.ListWorldWondersOK:
				out = v.Wonders
				return nil
			case *gen.ListWorldWondersNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.ListWorldWondersUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.ListWorldWondersOperation, res)
		},
	)
	return out, err
}

// ShowWorldArchive fetches the immutable end-of-round snapshot for an
// archived world (§16.6). The backend returns 404 while the world is
// still live or has no archive row; callers receive that as an
// `api.Error{Code: "not_found"}` and decide whether to translate it
// into a friendlier "not archived yet" hint.
func (c *Client) ShowWorldArchive(ctx context.Context, worldID string) (*gen.RoundArchive, error) {
	var out *gen.RoundArchive
	err := c.call(ctx, gen.GetWorldArchiveOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.GetWorldArchive(ctx, gen.GetWorldArchiveParams{WorldID: worldID})
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.RoundArchive:
				out = v
				return nil
			case *gen.GetWorldArchiveNotFound:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.GetWorldArchiveUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.GetWorldArchiveOperation, res)
		},
	)
	return out, err
}

// ShowHallOfFame fetches the four per-server leaderboard snapshots
// (champions / wreckers / warlords / veterans). Passing a non-empty
// `kind` restricts the response to one leaderboard; empty kind returns
// all four. The backend rebuilds these snapshots only at round end
// (§17.4), so the same call returns the same data between rounds.
func (c *Client) ShowHallOfFame(ctx context.Context, serverID, kind string) (*gen.HallOfFame, error) {
	params := gen.GetHallOfFameParams{ID: serverID}
	if kind != "" {
		params.Kind = gen.NewOptGetHallOfFameKind(gen.GetHallOfFameKind(kind))
	}
	var out *gen.HallOfFame
	err := c.call(ctx, gen.GetHallOfFameOperation,
		func(ctx context.Context) (any, error) {
			return c.gen.GetHallOfFame(ctx, params)
		},
		func(res any, rid string) error {
			switch v := res.(type) {
			case *gen.HallOfFame:
				out = v
				return nil
			case *gen.GetHallOfFameUnauthorized:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.GetHallOfFameForbidden:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			case *gen.GetHallOfFameUnprocessableEntity:
				return fromEnvelope((*gen.ErrorEnvelope)(v), rid)
			}
			return unexpectedRes(gen.GetHallOfFameOperation, res)
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
