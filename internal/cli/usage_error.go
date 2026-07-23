package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagUsageError wraps a cobra flag-parse failure so Execute() can route it to
// the tp error contract at exit 2 (spec §13.1): a usage error emitted as the
// standard {error, code, hint} object, never as bare cobra text on exit 1. The
// failing command is captured so the hint can be command-specific.
type flagUsageError struct {
	cmd *cobra.Command
	err error
}

func (e flagUsageError) Error() string { return e.err.Error() }
func (e flagUsageError) Unwrap() error { return e.err }

// wrapFlagErrors installs a FlagErrorFunc on cmd. cobra inherits a parent's
// FlagErrorFunc onto every subcommand, so installing it once on the root covers
// the whole command tree.
func wrapFlagErrors(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return flagUsageError{cmd: c, err: err}
	})
}

// reasonCommands take a trailing positional reason/commit value that an agent
// commonly prefixes with '-' — cobra then reads it as a flag. For these, the
// usage hint names the '--' separator (spec §13.1 row 3).
var reasonCommands = map[string]bool{
	"done":   true,
	"close":  true,
	"commit": true,
}

// usageErrorDetail shapes a flag-parse failure into the tp error contract's
// (message, hint). For a reason-taking command, a positional that starts with
// '-' is detected and the hint points at the '--' separator.
func usageErrorDetail(cmd *cobra.Command, err error) (msg, hint string) {
	msg = err.Error()
	hint = fmt.Sprintf("run '%s --help' for usage", cmd.CommandPath())
	if reasonCommands[cmd.Name()] {
		if token := dashLeadingPositional(cmd); token != "" {
			msg = fmt.Sprintf("argument %q starts with '-' and was read as a flag", token)
			hint = fmt.Sprintf("separate positionals from flags with '--': %s <id> -- <reason>", cmd.CommandPath())
		}
	}
	return msg, hint
}

// dashLeadingPositional scans os.Args after cmd's invocation for a positional
// token (one that is not a registered flag of cmd and not a value consumed by
// one) that begins with '-'. Such a token is the likely cause of a flag-parse
// failure on a reason/commit-taking command. Returns the offending token, or ""
// if none is found.
func dashLeadingPositional(cmd *cobra.Command) string {
	flags := cmd.Flags()
	start := -1
	name := cmd.Name()
	for i, a := range os.Args {
		if a == name {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	expectValue := false
	for _, a := range os.Args[start+1:] {
		if expectValue {
			expectValue = false
			continue
		}
		if a == "--" {
			break // explicit separator ends flag scanning
		}
		if len(a) > 1 && a[0] == '-' {
			flagName := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(flagName, '='); eq >= 0 {
				flagName = flagName[:eq]
			}
			if f := lookupFlag(flags, flagName); f != nil {
				// Known flag: a non-bool flag consumes the next token as its value.
				if f.Value.Type() != "bool" {
					expectValue = true
				}
				continue
			}
			// Unknown flag-like token → a dash-leading positional.
			return a
		}
	}
	return ""
}

func lookupFlag(fs *pflag.FlagSet, name string) *pflag.Flag {
	if f := fs.Lookup(name); f != nil {
		return f
	}
	// ShorthandLookup panics on multi-character input, so only treat a single
	// character as a possible shorthand (e.g. -v).
	if len(name) == 1 {
		return fs.ShorthandLookup(name)
	}
	return nil
}
