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
