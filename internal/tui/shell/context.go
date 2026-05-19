package shell

import "sync"

// Context is the in-memory, mutable scope tuple every verb reads to
// decide which entity to act on when the user omits an explicit
// reference. Verbs MUST go through (*Context).Set to mutate fields —
// the embedded mutex keeps concurrent readline-thread / tea-program
// access safe, and Set is the single funnel that triggers a
// persistence write (via the StateStore on Session).
type Context struct {
	mu            sync.RWMutex
	serverSlug    string
	worldSlug     string
	kingdomHandle string
}

// NewContext returns a zero-value Context.
func NewContext() *Context { return &Context{} }

// ServerSlug returns the current server scope, or "" if unset.
func (c *Context) ServerSlug() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverSlug
}

// WorldSlug returns the current world scope, or "" if unset.
func (c *Context) WorldSlug() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.worldSlug
}

// KingdomHandle returns the caller's handle in the current world
// scope, or "" if unset.
func (c *Context) KingdomHandle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kingdomHandle
}

// SetServer updates the server scope. Setting a new server (or
// clearing it) drops the dependent world + kingdom fields, since
// those are scoped to the server.
func (c *Context) SetServer(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.serverSlug != slug {
		c.worldSlug = ""
		c.kingdomHandle = ""
	}
	c.serverSlug = slug
}

// SetWorld updates the world scope. Setting a new world drops the
// dependent kingdom handle (each world has its own).
func (c *Context) SetWorld(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.worldSlug != slug {
		c.kingdomHandle = ""
	}
	c.worldSlug = slug
}

// SetKingdomHandle updates the caller's handle in the current world.
func (c *Context) SetKingdomHandle(handle string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kingdomHandle = handle
}

// Snapshot returns a copy safe to pass into state.Save.
func (c *Context) Snapshot() ContextSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return ContextSnapshot{
		ServerSlug:    c.serverSlug,
		WorldSlug:     c.worldSlug,
		KingdomHandle: c.kingdomHandle,
	}
}

// ContextSnapshot is the value-type form of Context used for
// persistence. The fields mirror Context but are exported so the
// state package can marshal them.
type ContextSnapshot struct {
	ServerSlug    string
	WorldSlug     string
	KingdomHandle string
}

// Apply overwrites Context from a snapshot. Used by shell.Run on
// startup after loading ~/.dun/state.json.
func (c *Context) Apply(s ContextSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverSlug = s.ServerSlug
	c.worldSlug = s.WorldSlug
	c.kingdomHandle = s.KingdomHandle
}
