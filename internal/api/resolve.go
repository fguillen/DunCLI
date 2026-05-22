package api

import (
	"context"
	"fmt"
	"sync"
)

// resolverCache memoizes name/slug/handle → ULID lookups for one shell
// session. The shell creates a single *Client and reuses it across
// commands; resolvers consult the cache first, fall back to the
// appropriate list*/show* endpoint on miss, and write the result back.
//
// Mutating operations are responsible for calling the matching
// Invalidate* method on success so a stale ULID isn't served for an
// entity the user just renamed/merged/left/joined.
//
// Concurrency: the shell is single-threaded today, but later phases may
// run a connectivity probe in parallel with the prompt. Guarding the
// maps with a single sync.RWMutex is the cheapest correct choice — no
// resolver is on a hot path.
type resolverCache struct {
	mu       sync.RWMutex
	servers  map[string]string            // slug → ULID
	worlds   map[string]map[string]string // serverID → slug → ULID
	players  map[string]map[string]string // serverID → handle → ULID (uses handle as the ID-like key)
	regions  map[string]map[string]string // worldID → name → ULID
	armies   map[string]map[string]string // kingdomID → name → ULID
	kingdoms map[string]string            // worldID → caller's kingdom ULID

	// Admin-scope caches. Kept separate from the player ones above
	// because the admin surface is a different listing (servers the
	// caller administers, not servers the caller can join) — the two
	// namespaces must never alias even for the same slug.
	adminServers map[string]string            // slug → ULID
	adminWorlds  map[string]map[string]string // serverID → slug → ULID
}

func newResolverCache() *resolverCache {
	return &resolverCache{
		servers:      map[string]string{},
		worlds:       map[string]map[string]string{},
		players:      map[string]map[string]string{},
		regions:      map[string]map[string]string{},
		armies:       map[string]map[string]string{},
		kingdoms:     map[string]string{},
		adminServers: map[string]string{},
		adminWorlds:  map[string]map[string]string{},
	}
}

// notFound is the error returned when an entity name/slug/handle wasn't
// present in the upstream list/show response. It surfaces as an
// api.Error with code="not_found" so UI code can treat it identically
// to a backend 404.
func notFound(kind, key string) error {
	return &Error{
		Code:    "not_found",
		Message: fmt.Sprintf("%s %q not found", kind, key),
	}
}

// ResolveServer maps a server slug to its ULID. On miss it calls
// listPlayerServers and walks the result.
func (c *Client) ResolveServer(ctx context.Context, slug string) (string, error) {
	c.cache.mu.RLock()
	if id, ok := c.cache.servers[slug]; ok {
		c.cache.mu.RUnlock()
		return id, nil
	}
	c.cache.mu.RUnlock()

	servers, err := c.ListPlayerServers(ctx)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	for _, s := range servers {
		c.cache.servers[s.Slug] = s.ID
	}
	if id, ok := c.cache.servers[slug]; ok {
		return id, nil
	}
	return "", notFound("server", slug)
}

// ResolveWorld maps a world slug (within a known server) to its ULID.
func (c *Client) ResolveWorld(ctx context.Context, serverID, slug string) (string, error) {
	c.cache.mu.RLock()
	if m, ok := c.cache.worlds[serverID]; ok {
		if id, ok := m[slug]; ok {
			c.cache.mu.RUnlock()
			return id, nil
		}
	}
	c.cache.mu.RUnlock()

	worlds, err := c.ListServerWorlds(ctx, serverID)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	m := c.cache.worlds[serverID]
	if m == nil {
		m = map[string]string{}
		c.cache.worlds[serverID] = m
	}
	for _, w := range worlds {
		m[w.Slug] = w.ID
	}
	if id, ok := m[slug]; ok {
		return id, nil
	}
	return "", notFound("world", slug)
}

// ResolvePlayer maps a player handle (on a server) to a stable key the
// rest of the system can use. The player surface keys profiles by
// handle, not ULID — the resolver still exists so the shell can verify
// the handle exists before issuing a downstream call, and so future
// backend changes that surface a ULID slot can be added here in one
// place. The return value today is the canonical handle string echoed
// by the backend (which may differ from the user's input in casing).
func (c *Client) ResolvePlayer(ctx context.Context, serverID, handle string) (string, error) {
	c.cache.mu.RLock()
	if m, ok := c.cache.players[serverID]; ok {
		if id, ok := m[handle]; ok {
			c.cache.mu.RUnlock()
			return id, nil
		}
	}
	c.cache.mu.RUnlock()

	profile, err := c.ShowPlayerProfile(ctx, serverID, handle)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	m := c.cache.players[serverID]
	if m == nil {
		m = map[string]string{}
		c.cache.players[serverID] = m
	}
	m[profile.Handle] = profile.Handle
	return profile.Handle, nil
}

// ResolveKingdom returns the caller's own kingdom ULID for the given
// world. There is no public endpoint that maps (worldID, otherHandle) →
// kingdomID in v1; later phases that need that mapping pull the ID
// directly out of list*/show* payloads.
func (c *Client) ResolveKingdom(ctx context.Context, worldID string) (string, error) {
	c.cache.mu.RLock()
	if id, ok := c.cache.kingdoms[worldID]; ok {
		c.cache.mu.RUnlock()
		return id, nil
	}
	c.cache.mu.RUnlock()

	world, err := c.ShowWorld(ctx, worldID)
	if err != nil {
		return "", err
	}
	mk, ok := world.MyKingdom.Get()
	if !ok {
		return "", notFound("kingdom in world", worldID)
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	c.cache.kingdoms[worldID] = mk.ID
	return mk.ID, nil
}

// ResolveRegion maps a region name (within a world) to its ULID by
// pulling the full world map and scanning. The map is small (one
// world's regions) and rarely changes; the cache makes the first map
// fetch pay for all subsequent name lookups in the session.
func (c *Client) ResolveRegion(ctx context.Context, worldID, name string) (string, error) {
	c.cache.mu.RLock()
	if m, ok := c.cache.regions[worldID]; ok {
		if id, ok := m[name]; ok {
			c.cache.mu.RUnlock()
			return id, nil
		}
	}
	c.cache.mu.RUnlock()

	regions, err := c.ShowWorldMap(ctx, worldID)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	m := c.cache.regions[worldID]
	if m == nil {
		m = map[string]string{}
		c.cache.regions[worldID] = m
	}
	for _, r := range regions {
		m[r.Name] = r.ID
	}
	if id, ok := m[name]; ok {
		return id, nil
	}
	return "", notFound("region", name)
}

// ResolveArmy maps an army name (within a kingdom) to its ULID.
func (c *Client) ResolveArmy(ctx context.Context, kingdomID, name string) (string, error) {
	c.cache.mu.RLock()
	if m, ok := c.cache.armies[kingdomID]; ok {
		if id, ok := m[name]; ok {
			c.cache.mu.RUnlock()
			return id, nil
		}
	}
	c.cache.mu.RUnlock()

	armies, err := c.ListKingdomArmies(ctx, kingdomID)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	m := c.cache.armies[kingdomID]
	if m == nil {
		m = map[string]string{}
		c.cache.armies[kingdomID] = m
	}
	for _, a := range armies {
		m[a.Name] = a.ID
	}
	if id, ok := m[name]; ok {
		return id, nil
	}
	return "", notFound("army", name)
}

// InvalidateServers drops the server cache. Call after joinServer.
func (c *Client) InvalidateServers() {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	c.cache.servers = map[string]string{}
}

// InvalidateWorlds drops the cached worlds for the given server. Call
// after joinWorld.
func (c *Client) InvalidateWorlds(serverID string) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	delete(c.cache.worlds, serverID)
}

// InvalidateArmies drops the cached armies for the given kingdom. Call
// after splitArmy, renameArmy, mergeArmy.
func (c *Client) InvalidateArmies(kingdomID string) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	delete(c.cache.armies, kingdomID)
}

// InvalidateKingdom is the hook mutating Phase 7+ verbs call after a
// successful build/training/cancel. The kingdom ULID itself does not
// change over a session — `ResolveKingdom` is keyed by worldID — so
// this is a no-op today, but the hook is wired so callers don't need
// to remember to add it when later phases extend the kingdom cache.
func (c *Client) InvalidateKingdom(_ string) {}

// ── Admin-scope resolvers ─────────────────────────────────────────────
//
// Mirrors of ResolveServer / ResolveWorld for the admin surface. They
// back the Phase 15+ admin verbs; Phase 14 wires them so later phases
// resolve admin server/world slugs the same way the player shell
// resolves their counterparts.

// ResolveAdminServer maps a server slug to its ULID. On miss it calls
// listAdminServers and walks the result.
func (c *Client) ResolveAdminServer(ctx context.Context, slug string) (string, error) {
	c.cache.mu.RLock()
	if id, ok := c.cache.adminServers[slug]; ok {
		c.cache.mu.RUnlock()
		return id, nil
	}
	c.cache.mu.RUnlock()

	servers, err := c.ListAdminServers(ctx)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	for _, s := range servers {
		c.cache.adminServers[s.Slug] = s.ID
	}
	if id, ok := c.cache.adminServers[slug]; ok {
		return id, nil
	}
	return "", notFound("server", slug)
}

// ResolveAdminWorld maps a world slug (within a known admin server) to
// its ULID. On miss it calls listAdminWorlds and walks the result.
func (c *Client) ResolveAdminWorld(ctx context.Context, serverID, slug string) (string, error) {
	c.cache.mu.RLock()
	if m, ok := c.cache.adminWorlds[serverID]; ok {
		if id, ok := m[slug]; ok {
			c.cache.mu.RUnlock()
			return id, nil
		}
	}
	c.cache.mu.RUnlock()

	worlds, err := c.ListAdminWorlds(ctx, serverID)
	if err != nil {
		return "", err
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	m := c.cache.adminWorlds[serverID]
	if m == nil {
		m = map[string]string{}
		c.cache.adminWorlds[serverID] = m
	}
	for _, w := range worlds {
		m[w.Slug] = w.ID
	}
	if id, ok := m[slug]; ok {
		return id, nil
	}
	return "", notFound("world", slug)
}

// InvalidateAdminServers drops the admin server cache. Call after
// createServer / deleteServer.
func (c *Client) InvalidateAdminServers() {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	c.cache.adminServers = map[string]string{}
}

// InvalidateAdminWorlds drops the cached admin worlds for the given
// server. Call after proposeWorld / cancelWorld.
func (c *Client) InvalidateAdminWorlds(serverID string) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	delete(c.cache.adminWorlds, serverID)
}
