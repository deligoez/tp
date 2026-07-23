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

// EnsureConfigured sets output mode from the environment when the normal startup
// Configure (run via cobra PersistentPreRun) never executed — chiefly when a
// flag-parse error aborted cobra before PersistentPreRun ran. JSON mode is
// enabled whenever stdout is not a terminal or --json appears in argv, so the
// error is emitted as the standard {error, code, hint} object an agent expects
// rather than colored TTY text. (spec §13.1: flag-parse failures surface here.)
func EnsureConfigured(argv []string) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		jsonMode = true
	}
	for _, a := range argv {
		switch a {
		case "--json":
			jsonMode = true
		case "--no-color":
			color.NoColor = true
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}

// defaultHint returns a code-appropriate "next command to run" hint for a call
// site that supplied none. This is the §13.2 guarantee: every error object tp
// emits on a non-zero exit carries a hint naming a command to run, not only the
// rule that was broken. Call sites that pass a tailored hint always win.
func defaultHint(code int) string {
	switch code {
	case 2: // usage
		return "see 'tp --help' or the subcommand's '--help' for usage"
	case 3: // file
		return "run 'tp use <file>' to set the task file, or 'tp init <spec>' to create one"
	case 4: // state
		return "run 'tp status' and 'tp list' to inspect task states"
	default: // 1 (validation) and anything else
		return "run 'tp validate' to audit the task file"
	}
}

func resolveHint(code int, hint ...string) string {
	if len(hint) > 0 && hint[0] != "" {
		return hint[0]
	}
	return defaultHint(code)
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

// Error writes a structured error to stderr. Optional hint provides recovery
// action; when omitted, a code-keyed default hint names the next command to run
// (spec §13.2).
func Error(code int, msg string, hint ...string) {
	h := resolveHint(code, hint...)
	if jsonMode {
		e := struct {
			Error string `json:"error"`
			Code  int    `json:"code"`
			Hint  string `json:"hint,omitempty"`
		}{Error: msg, Code: code, Hint: h}
		data, _ := json.Marshal(e)
		fmt.Fprintln(os.Stderr, string(data))
	} else {
		red := color.New(color.FgRed, color.Bold)
		red.Fprintf(os.Stderr, "error: ")
		fmt.Fprintln(os.Stderr, msg)
		if h != "" {
			dim := color.New(color.Faint)
			dim.Fprintf(os.Stderr, "  hint: ")
			fmt.Fprintln(os.Stderr, h)
		}
	}
}

// ErrorExtras writes a structured error with extra top-level JSON fields
// (e.g. suggested_files) in addition to the standard {error, code, hint}. In
// TTY mode the message, hint, and each extra value are printed to stderr.
func ErrorExtras(code int, msg string, extras map[string]any, hint ...string) {
	h := resolveHint(code, hint...)
	if jsonMode {
		payload := map[string]any{
			"error": msg,
			"code":  code,
		}
		for k, v := range extras {
			payload[k] = v
		}
		if h != "" {
			payload["hint"] = h
		}
		data, _ := json.Marshal(payload)
		fmt.Fprintln(os.Stderr, string(data))
		return
	}
	red := color.New(color.FgRed, color.Bold)
	red.Fprintf(os.Stderr, "error: ")
	fmt.Fprintln(os.Stderr, msg)
	if h != "" {
		dim := color.New(color.Faint)
		dim.Fprintf(os.Stderr, "  hint: ")
		fmt.Fprintln(os.Stderr, h)
	}
	for k, v := range extras {
		if items, ok := v.([]string); ok {
			for _, it := range items {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", k, it)
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  %s: %v\n", k, v)
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
