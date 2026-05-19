package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/tui/theme"
)

// Options tweak Run's startup behavior. The zero value is fine for
// the bare `dun` invocation; `dun login` flips PostLoginPicker on
// when the player has more than one server membership.
type Options struct {
	// Version is the build version string printed by the `version`
	// built-in verb.
	Version string

	// PostLoginPicker, when true, asks the configured PickServer hook
	// (set via SetPostLoginPicker) to run before the prompt loop
	// begins. The hook is registered by the verbs package so the
	// shell stays free of game-specific imports.
	PostLoginPicker bool

	// SingleMemberSlug, if non-empty, is auto-applied to the session
	// Context.ServerSlug — the "exactly one server" branch of the
	// post-login flow that does not need the interactive picker.
	SingleMemberSlug string
}

// PostLoginPicker is the hook the verbs package registers so the
// shell can ask the user to choose a server right after login
// without the shell importing the verbs package. nil means "no
// picker registered yet" and the shell silently skips this step.
var postLoginPicker func(ctx context.Context, sess *Session) error

// SetPostLoginPicker installs the hook. Call from the verbs package
// init() so it is available before Run is invoked.
func SetPostLoginPicker(f func(ctx context.Context, sess *Session) error) {
	postLoginPicker = f
}

// Run drives the REPL. It is the bare `dun` entrypoint and the post-
// login handoff target. The returned error is non-nil only for a
// fatal startup condition (e.g. the connectivity probe failed); a
// clean Ctrl-D returns nil.
func Run(ctx context.Context, cfgBaseURL, credEmail string, client *api.Client, opts Options) error {
	// Build the session.
	scope := NewContext()
	state := NewFileStore(cfgBaseURL, credEmail)
	if snap, err := state.Load(); err == nil {
		scope.Apply(snap)
	}
	if opts.SingleMemberSlug != "" {
		scope.SetServer(opts.SingleMemberSlug)
		_ = state.Save(scope.Snapshot())
	}

	sess := &Session{
		API:     client,
		Cfg:     ConfigSnapshot{BaseURL: cfgBaseURL},
		Creds:   CredentialSnapshot{Email: credEmail},
		Context: scope,
		Out:     os.Stdout,
		State:   state,
	}

	// Register built-ins (idempotent across re-entries within the
	// same process; in practice Run is called once per invocation).
	registerBuiltinsOnce(opts.Version)

	// Connectivity probe — fail fast and loud if the backend is
	// unreachable. The error renderer pulls the typed code +
	// request_id out of *api.Error.
	if err := client.Health(ctx); err != nil {
		Err(sess.Out, fmt.Errorf("backend unreachable: %s: %w", cfgBaseURL, err))
		return err
	}

	// Greeting.
	Info(sess.Out, fmt.Sprintf("Connected to %s as %s", cfgBaseURL, credEmail))

	// Post-login server picker hook.
	if opts.PostLoginPicker && postLoginPicker != nil {
		if err := postLoginPicker(ctx, sess); err != nil {
			// Soft-fail — the user can still type `server join …`
			// later. Render the error to scrollback and move on.
			Err(sess.Out, err)
		}
	}

	rl, err := newReadline(sess)
	if err != nil {
		return fmt.Errorf("shell: init readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	st := theme.Active()
	rl.SetPrompt(st.Prompt.Render("dun> "))

	for {
		line, err := rl.Readline()
		switch {
		case errors.Is(err, readline.ErrInterrupt):
			// Ctrl-C on an empty / partial line: discard and loop.
			continue
		case errors.Is(err, io.EOF):
			// Ctrl-D: persist and bail.
			_ = sess.State.Save(scope.Snapshot())
			return nil
		case err != nil:
			return fmt.Errorf("shell: readline: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := Dispatch(ctx, sess, line); errors.Is(err, ErrExit) {
			_ = sess.State.Save(scope.Snapshot())
			return nil
		}
		// Persist after every successful dispatch so a crash mid-
		// session doesn't lose the most recent scope change.
		_ = sess.State.Save(scope.Snapshot())
	}
}

// newReadline builds the *readline.Instance with the history file
// and tab completer wired in. ~/.dun/ is created lazily at 0700.
func newReadline(sess *Session) (*readline.Instance, error) {
	historyPath, err := historyPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir history dir: %w", err)
	}
	return readline.NewEx(&readline.Config{
		Prompt:            "dun> ",
		HistoryFile:       historyPath,
		AutoComplete:      NewCompleter(sess),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
}

func historyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".dun", "history"), nil
}

// registerBuiltinsOnce guards built-in registration so a test that
// invokes Run multiple times in-process doesn't trip the duplicate
// panic in Register.
var builtinsRegistered bool

func registerBuiltinsOnce(version string) {
	if builtinsRegistered {
		return
	}
	registerBuiltins(version)
	builtinsRegistered = true
}
