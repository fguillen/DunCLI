package shell

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/shlex"
)

// ParsedCommand is the structured form of one line typed at the
// prompt. Resolve() walks the verb tree to find a leaf; the dispatcher
// in Dispatch() runs that leaf with positional Args + named Flags.
type ParsedCommand struct {
	Verb  *Verb             // resolved leaf verb
	Path  []string          // verb names traversed (e.g. ["server", "join"])
	Args  []string          // positional arguments after the verb path
	Flags map[string]string // --flag → value (boolean flags map to "true")
}

// errUnknownVerb is returned when the first token does not match any
// registered verb. Distinct error so the dispatcher can print a
// helpful "type `help` for available commands" hint.
var errUnknownVerb = errors.New("unknown command")

// Parse splits `line` into a ParsedCommand against the player verb
// registry. An empty / whitespace-only line returns (nil, nil) so the
// prompt loop can ignore it without rendering anything. The dispatcher
// uses parseWith so the admin shell resolves against its own registry.
func Parse(line string) (*ParsedCommand, error) {
	return parseWith(defaultRegistry, line)
}

// parseWith is Parse against an explicit verb registry.
func parseWith(reg *registry, line string) (*ParsedCommand, error) {
	if strings.TrimSpace(line) == "" {
		return nil, nil
	}
	tokens, err := shlex.Split(line)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	// Walk the verb tree.
	root, ok := reg.resolve(tokens[0])
	if !ok {
		return nil, fmt.Errorf("%w: %q (type `help` for a list)", errUnknownVerb, tokens[0])
	}

	// Walk into Sub as long as the next token matches a registered
	// sub-verb. A verb that has both `Run` and `Sub` (e.g. `build`,
	// `train`) is a leaf for the default invocation but still routes
	// to its sub when the next token names one — so `build preview x`
	// resolves to the `preview` sub-verb, while `build x` falls
	// through to the parent's Run with args=["x"].
	path := []string{tokens[0]}
	cur := root
	i := 1
	for i < len(tokens) {
		sub, ok := cur.Sub[tokens[i]]
		if !ok {
			if cur.IsLeaf() {
				break
			}
			return nil, fmt.Errorf("unknown subcommand %q for %q", tokens[i], strings.Join(path, " "))
		}
		path = append(path, tokens[i])
		cur = sub
		i++
	}
	if !cur.IsLeaf() {
		return nil, fmt.Errorf("`%s` requires a subcommand (try `help %s`)",
			strings.Join(path, " "), tokens[0])
	}

	// Remaining tokens = positional args + `--flag [value]` pairs.
	args := make([]string, 0)
	flags := make(map[string]string)
	for j := i; j < len(tokens); j++ {
		t := tokens[j]
		if !strings.HasPrefix(t, "--") {
			args = append(args, t)
			continue
		}
		// Strip leading dashes; accept `--name=value` or `--name value`.
		raw := strings.TrimPrefix(t, "--")
		eq := strings.IndexByte(raw, '=')
		var name, val string
		var inline bool
		if eq >= 0 {
			name = raw[:eq]
			val = raw[eq+1:]
			inline = true
		} else {
			name = raw
		}
		spec, ok := cur.FlagByName(name)
		if !ok {
			return nil, fmt.Errorf("unknown flag --%s for `%s`", name, strings.Join(path, " "))
		}
		if !spec.HasValue {
			if inline {
				return nil, fmt.Errorf("--%s does not take a value", name)
			}
			flags[name] = "true"
			continue
		}
		if !inline {
			j++
			if j >= len(tokens) {
				return nil, fmt.Errorf("--%s requires a value", name)
			}
			val = tokens[j]
		}
		flags[name] = val
	}

	return &ParsedCommand{
		Verb:  cur,
		Path:  path,
		Args:  args,
		Flags: flags,
	}, nil
}

// Dispatch parses `line` and runs the resolved verb, rendering any
// error via Err(). Returns ErrExit when the verb (e.g. `quit`) asked
// the loop to terminate. Other errors are written to scrollback and
// nil is returned — the prompt should keep going.
func Dispatch(ctx context.Context, sess *Session, line string) error {
	cmd, err := parseWith(sess.registry(), line)
	if err != nil {
		Err(sess.Out, err)
		return nil
	}
	if cmd == nil {
		return nil
	}
	if err := cmd.Verb.Run(ctx, sess, cmd.Args, cmd.Flags); err != nil {
		if errors.Is(err, ErrExit) {
			return ErrExit
		}
		Err(sess.Out, err)
	}
	return nil
}
