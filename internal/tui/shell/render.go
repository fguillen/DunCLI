package shell

import (
	"fmt"
	"io"
	"strings"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/tui/theme"
)

// Err formats an error for the scrollback in the standard
// `error: <message> (code=<code>, request_id=<id>)` form. When err
// wraps an *api.Error, the typed fields are used; otherwise the
// message comes from err.Error() and code / request_id are omitted.
func Err(out io.Writer, err error) {
	if err == nil {
		return
	}
	st := theme.Active()
	var line string
	if apiErr := api.AsError(err); apiErr != nil {
		if apiErr.RequestID != "" {
			line = fmt.Sprintf("error: %s (code=%s, request_id=%s)",
				apiErr.Message, apiErr.Code, apiErr.RequestID)
		} else {
			line = fmt.Sprintf("error: %s (code=%s)", apiErr.Message, apiErr.Code)
		}
	} else {
		line = "error: " + err.Error()
	}
	_, _ = fmt.Fprintln(out, st.Error.Render(line))
}

// Info prints a single line styled as a subtle informational message.
func Info(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, theme.Active().Subtle.Render(msg))
}

// Success prints a single line styled with the success color.
func Success(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, theme.Active().Success.Render(msg))
}

// Strong prints a single line styled with the strong / accent color.
func Strong(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, theme.Active().Strong.Render(msg))
}

// Section prints a header followed by `body`. If body is empty the
// "(none)" placeholder is rendered in the subtle style.
func Section(out io.Writer, header, body string) {
	st := theme.Active()
	_, _ = fmt.Fprintln(out, st.Strong.Render(header))
	if strings.TrimSpace(body) == "" {
		_, _ = fmt.Fprintln(out, "  "+st.Subtle.Render("(none)"))
		return
	}
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		_, _ = fmt.Fprintln(out, "  "+line)
	}
}
