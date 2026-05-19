// Package shell hosts the long-running `dun>` REPL: the prompt
// (readline), the verb dispatcher, the per-session Context tracking
// the active (server, world, kingdom) tuple, and the persistent state
// file at ~/.dun/state.json. Game verbs live in
// internal/tui/verbs/* and register into the package-level Registry
// via init().
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/fguillen/dun-cli/internal/api"
)

// Handler is the function signature every verb runs through. args
// holds positional arguments (after stripping `--flag value` pairs).
// flags holds the parsed `--name value` map. The handler is expected
// to print any user-facing output to sess.Out and return only errors
// that the dispatcher should render via render.Err.
type Handler func(ctx context.Context, sess *Session, args []string, flags map[string]string) error

// FlagSpec declares one `--flag` understood by a verb. Name is
// without the leading dashes; HasValue=false means the flag is a
// boolean toggle (`--upgradable`); Suggest, if non-nil, supplies tab
// completion candidates for the value position.
type FlagSpec struct {
	Name     string
	HasValue bool
	Help     string
	Suggest  Suggester
}

// Suggester returns tab-completion candidates for a verb's positional
// argument (Verb.Complete) or flag value (FlagSpec.Suggest). The
// prefix is the partial token typed so far; implementations may
// ignore it and return the full candidate list.
//
// Suggesters that hit the network MUST honor ctx — readline calls
// completion synchronously on the keystroke, so a wedged Suggester
// freezes the prompt.
type Suggester interface {
	Suggest(ctx context.Context, sess *Session, prefix string) ([]string, error)
}

// SuggestFunc is the convenience adapter for ad-hoc Suggesters.
type SuggestFunc func(ctx context.Context, sess *Session, prefix string) ([]string, error)

// Suggest implements Suggester.
func (f SuggestFunc) Suggest(ctx context.Context, sess *Session, prefix string) ([]string, error) {
	return f(ctx, sess, prefix)
}

// Verb is one entry in the shell's command tree. A Verb either has a
// non-nil Run (leaf) or a non-empty Sub map (branch). Verbs are
// registered with Register / RegisterSub and looked up by Resolve.
type Verb struct {
	Name     string
	Sub      map[string]*Verb
	Summary  string
	Usage    string
	Run      Handler
	Complete Suggester
	Flags    []FlagSpec
}

// IsLeaf reports whether v is a runnable verb (vs. a branching one).
func (v *Verb) IsLeaf() bool { return v.Run != nil }

// FlagByName looks up a FlagSpec by name (without leading dashes).
func (v *Verb) FlagByName(name string) (FlagSpec, bool) {
	for _, f := range v.Flags {
		if f.Name == name {
			return f, true
		}
	}
	return FlagSpec{}, false
}

// registry is the process-wide verb registry. Init order does not
// matter: verbs.* packages call Register from init(), shell.Run
// reads the registry on startup.
type registry struct {
	mu    sync.RWMutex
	verbs map[string]*Verb
}

var defaultRegistry = &registry{verbs: map[string]*Verb{}}

// Register adds a top-level verb. Panics on duplicate name —
// duplicate registration is a programming bug, never a runtime
// condition. Aliases (e.g. quit / exit) are intentionally registered
// as two distinct verbs that point at the same Handler.
func Register(v *Verb) {
	if v == nil || v.Name == "" {
		panic("shell.Register: nil or unnamed verb")
	}
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()
	if _, exists := defaultRegistry.verbs[v.Name]; exists {
		panic("shell.Register: duplicate verb " + v.Name)
	}
	defaultRegistry.verbs[v.Name] = v
}

// Resolve looks up a verb by name. Returns (nil, false) when no such
// verb exists.
func Resolve(name string) (*Verb, bool) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	v, ok := defaultRegistry.verbs[name]
	return v, ok
}

// Verbs returns every registered top-level verb, sorted by name.
// Used by `help` and the completion engine.
func Verbs() []*Verb {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	out := make([]*Verb, 0, len(defaultRegistry.verbs))
	for _, v := range defaultRegistry.verbs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// reset is used by tests to drop registrations between cases.
func reset() {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()
	defaultRegistry.verbs = map[string]*Verb{}
}

// Session is what every verb handler receives. It bundles the API
// client, the persisted config + credentials, the per-shell Context
// (server/world/kingdom scope), and the writer the verb is expected
// to render its output to.
type Session struct {
	API     *api.Client
	Cfg     ConfigSnapshot
	Creds   CredentialSnapshot
	Context *Context
	Out     io.Writer

	// State is the on-disk session-context store, set by shell.Run
	// before the prompt loop starts. Verbs that mutate Context call
	// Save() so the change survives a restart.
	State StateStore
}

// ConfigSnapshot is the read-only view of ~/.dun/config.toml the
// shell hands to verbs. Phase 4 only needs the base URL; later
// phases may extend this struct.
type ConfigSnapshot struct {
	BaseURL string
}

// CredentialSnapshot is the read-only view of the active credential.
type CredentialSnapshot struct {
	Email string
}

// StateStore is the subset of state.go's Store that verbs need —
// they only ever mutate the active context and ask for a persist.
// Defined as an interface here so verb tests can pass a stub.
type StateStore interface {
	Save(ContextSnapshot) error
}

// ErrExit is returned by the `quit` / `exit` built-in to signal the
// main loop to terminate cleanly.
var ErrExit = errors.New("shell: exit requested")

// helpFormat formats one verb summary line for `help`. Exposed so
// the builtin help handler can reuse it.
func helpFormat(verbs []*Verb) string {
	var b strings.Builder
	for _, v := range verbs {
		fmt.Fprintf(&b, "  %-14s %s\n", v.Name, v.Summary)
	}
	return b.String()
}
