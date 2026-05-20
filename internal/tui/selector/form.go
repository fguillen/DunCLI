package selector

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// Field is one row in a Form() call. Label is the prompt text;
// Initial seeds the input; Validate (optional) returns nil for
// accepted values and an error whose message is shown inline. The
// caller reads the final value back via the returned Result map.
type Field struct {
	Key      string
	Label    string
	Initial  string
	Validate func(string) error
}

// Result maps Field.Key → final input string. Only fields present in
// the original input slice appear; the map order is not stable.
type Result map[string]string

// Confirm opens a transient yes/no prompt. The returned bool is the
// user's choice; ErrCancelled is returned if they aborted (Esc /
// Ctrl-C). Useful for irreversible mutations where a full Form would
// be overkill.
func Confirm(ctx context.Context, title, description string) (bool, error) {
	var ans bool
	c := huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&ans)
	if description != "" {
		c = c.Description(description)
	}
	form := huh.NewForm(huh.NewGroup(c)).WithTheme(huh.ThemeBase16())
	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled) {
			return false, ErrCancelled
		}
		return false, fmt.Errorf("selector: confirm: %w", err)
	}
	return ans, nil
}

// Form opens a huh-backed multi-field form with the given fields and
// returns their final values. ErrCancelled is returned on user
// dismissal. The ctx is honored for cancellation.
func Form(ctx context.Context, title string, fields []Field) (Result, error) {
	if len(fields) == 0 {
		return nil, errors.New("selector: no fields in form")
	}

	values := make(map[string]*string, len(fields))
	inputs := make([]huh.Field, 0, len(fields))
	for _, f := range fields {
		v := f.Initial
		values[f.Key] = &v
		in := huh.NewInput().
			Title(f.Label).
			Value(values[f.Key])
		if f.Validate != nil {
			in = in.Validate(f.Validate)
		}
		inputs = append(inputs, in)
	}

	form := huh.NewForm(huh.NewGroup(inputs...)).
		WithTheme(huh.ThemeBase16())
	if title != "" {
		form = form.WithProgramOptions()
	}

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled) {
			return nil, ErrCancelled
		}
		return nil, fmt.Errorf("selector: form: %w", err)
	}

	out := make(Result, len(values))
	for k, v := range values {
		out[k] = *v
	}
	return out, nil
}
