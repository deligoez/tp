package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

var (
	jsonMode bool
	quiet    bool
)

// Configure sets output mode flags. Call once at startup.
func Configure(forceJSON, quietMode, noColor bool) {
	jsonMode = forceJSON || !isatty.IsTerminal(os.Stdout.Fd())
	quiet = quietMode
	if noColor {
		color.NoColor = true
	}
}

// IsJSON returns true if output should be JSON.
func IsJSON() bool {
	return jsonMode
}

// JSON writes v as pretty-printed JSON to stdout.
func JSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Error writes a structured error to stderr. Optional hint provides recovery action.
func Error(code int, msg string, hint ...string) {
	if jsonMode {
		e := struct {
			Error string `json:"error"`
			Code  int    `json:"code"`
			Hint  string `json:"hint,omitempty"`
		}{Error: msg, Code: code}
		if len(hint) > 0 {
			e.Hint = hint[0]
		}
		data, _ := json.Marshal(e)
		fmt.Fprintln(os.Stderr, string(data))
	} else {
		red := color.New(color.FgRed, color.Bold)
		red.Fprintf(os.Stderr, "error: ")
		fmt.Fprintln(os.Stderr, msg)
		if len(hint) > 0 {
			dim := color.New(color.Faint)
			dim.Fprintf(os.Stderr, "  hint: ")
			fmt.Fprintln(os.Stderr, hint[0])
		}
	}
}

// Info writes an info message to stderr (suppressed in quiet mode).
func Info(msg string) {
	if quiet {
		return
	}
	if jsonMode {
		return
	}
	dim := color.New(color.Faint)
	dim.Fprintln(os.Stderr, msg)
}

// Success writes a success message to stderr.
func Success(msg string) {
	if quiet {
		return
	}
	if jsonMode {
		return
	}
	green := color.New(color.FgGreen)
	green.Fprintf(os.Stderr, "✓ ")
	fmt.Fprintln(os.Stderr, msg)
}
