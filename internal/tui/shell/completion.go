package shell

import (
	"context"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

// completeTimeout caps how long any single dynamic Suggester is
// allowed to run on a Tab. Tab completion is synchronous in readline,
// so a slow Suggester would freeze the prompt — we'd rather show no
// suggestions than hang the UI.
const completeTimeout = 800 * time.Millisecond

// completer is the readline.AutoCompleter adapter. It walks the
// registered verb tree on every Do() call and asks the resolved
// verb's Complete / FlagSpec.Suggest for candidates as appropriate.
type completer struct {
	sess *Session
}

// NewCompleter returns the readline auto-completer for the shell.
func NewCompleter(sess *Session) readline.AutoCompleter {
	return &completer{sess: sess}
}

// Do is readline.AutoCompleter. line is the entire current input;
// pos is the byte index of the cursor. Returns (candidates, length)
// where length is how much of the current word should be replaced
// (the prefix length).
func (c *completer) Do(line []rune, pos int) ([][]rune, int) {
	prefix := string(line[:pos])
	tokens := tokenizePartial(prefix)
	// The last token is what we are completing; the cursor may sit
	// either on it (extending) or just past it (after a space — a
	// fresh empty token).
	endsWithSpace := strings.HasSuffix(prefix, " ")
	var curWord string
	if !endsWithSpace && len(tokens) > 0 {
		curWord = tokens[len(tokens)-1]
		tokens = tokens[:len(tokens)-1]
	}

	candidates := c.suggest(tokens, curWord, endsWithSpace)
	out := make([][]rune, 0, len(candidates))
	for _, cand := range candidates {
		if !strings.HasPrefix(cand, curWord) {
			continue
		}
		// readline replaces the last len(curWord) runes with the chosen
		// candidate; we return only the unique suffix to append.
		out = append(out, []rune(cand[len(curWord):]))
	}
	return out, len([]rune(curWord))
}

// suggest is the pure version of Do exposed for testing. It returns
// the full candidate strings (not the suffixes readline expects).
//
//   - prior:        every token already committed (i.e. before the
//     one being edited).
//   - curWord:      partial token currently under the cursor; "" when
//     the cursor is just past a space.
//   - endsWithSpace: true when the cursor is after a separator and a
//     fresh slot is open.
func (c *completer) suggest(prior []string, curWord string, endsWithSpace bool) []string {
	// At the very start of a line: complete verbs.
	if len(prior) == 0 && !endsWithSpace {
		return verbNames()
	}
	if len(prior) == 0 && endsWithSpace {
		return verbNames()
	}

	// Resolve verb / subverb path.
	root, ok := Resolve(prior[0])
	if !ok {
		return nil
	}
	cur := root
	i := 1
	for !cur.IsLeaf() && i < len(prior) {
		sub, ok := cur.Sub[prior[i]]
		if !ok {
			return nil
		}
		cur = sub
		i++
	}
	// Two interesting positions remain:
	//   - branch verb, no more prior tokens → suggest subverbs.
	//   - leaf verb → suggest positional / flag candidates.
	if !cur.IsLeaf() {
		return subVerbNames(cur)
	}

	// Leaf: figure out whether the cursor is in a positional slot or
	// after a `--flag`. Tokens after `cur`'s name are positional /
	// flags; if the previous token starts with `--` and that flag
	// HasValue=true, we suggest values for that flag.
	rest := prior[i:]
	if len(rest) > 0 {
		last := rest[len(rest)-1]
		if strings.HasPrefix(last, "--") {
			name := strings.TrimPrefix(last, "--")
			if spec, ok := cur.FlagByName(name); ok && spec.HasValue && spec.Suggest != nil {
				return runSuggester(c.sess, spec.Suggest, curWord)
			}
		}
	}

	// --flag-name completion when the user types `--…`.
	if strings.HasPrefix(curWord, "--") {
		out := make([]string, 0, len(cur.Flags))
		for _, f := range cur.Flags {
			out = append(out, "--"+f.Name)
		}
		return out
	}

	// Positional completion via the verb's own Complete suggester.
	if cur.Complete != nil {
		return runSuggester(c.sess, cur.Complete, curWord)
	}
	return nil
}

// runSuggester invokes s under a short timeout. Any error is swallowed
// — completion is best-effort by design.
func runSuggester(sess *Session, s Suggester, prefix string) []string {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), completeTimeout)
	defer cancel()
	out, err := s.Suggest(ctx, sess, prefix)
	if err != nil {
		return nil
	}
	return out
}

// verbNames returns every top-level verb's name, sorted.
func verbNames() []string {
	verbs := Verbs()
	out := make([]string, 0, len(verbs))
	for _, v := range verbs {
		out = append(out, v.Name)
	}
	return out
}

// subVerbNames returns the sorted names of v's direct sub-verbs.
func subVerbNames(v *Verb) []string {
	out := make([]string, 0, len(v.Sub))
	for name := range v.Sub {
		out = append(out, name)
	}
	// Stable ordering for testability.
	sortStrings(out)
	return out
}

// tokenizePartial is a forgiving tokenizer used only by the
// completion engine. shlex returns an error on an unterminated quote
// while typing — completion must keep working in that state, so we
// fall back to a whitespace split.
func tokenizePartial(line string) []string {
	// Drop a trailing single unmatched quote if any so users typing
	// `profile set --real-name "F` still see completions.
	return strings.Fields(line)
}

// sortStrings is a tiny shim so the file doesn't need to import sort.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
