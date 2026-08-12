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

// unknownCommandUsage reports an unrecognized command name as a usage error
// (§9.1: exit 2), replacing cobra's two defaults — a plain error at the top
// level, which Execute would otherwise classify as a validation failure (exit
// 1), and a silent help dump (exit 0) for an unknown subcommand of a command
// that only dispatches, such as 'tp completion'. It returns nil when args name
// a known command, leaving cobra's dispatch untouched.
func unknownCommandUsage(root *cobra.Command, args []string) error {
	switch firstPositional(root, args) {
	case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		// The shell-completion helper commands are registered by cobra's own
		// Execute through an unexported hook, so they cannot be resolved here.
		return nil
	}
	// help and completion are registered lazily inside Execute too, but their
	// initializers are exported and idempotent: without them this check would
	// call 'tp help' an unknown command.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd(args...)
	// Find resolves the deepest matching command; its only failure mode is
	// cobra's own unknown-command check at the root, whose message carries the
	// did-you-mean suggestions, so it is kept verbatim.
	found, rest, err := root.Find(args)
	if err != nil {
		return flagUsageError{cmd: root, err: err}
	}
	// A command that dispatches to subcommands but cannot run takes no
	// positionals of its own, so a leftover token is an unknown subcommand.
	if !found.HasSubCommands() || found.Runnable() {
		return nil
	}
	name := firstPositional(found, rest)
	if name == "" {
		return nil
	}
	return flagUsageError{cmd: found, err: fmt.Errorf("unknown command %q for %q", name, found.CommandPath())}
}

// firstPositional returns the first token of args that is neither a flag of cmd
// nor a value consumed by one, or "" when args carries no positional.
func firstPositional(cmd *cobra.Command, args []string) string {
	flags := cmd.Flags()
	skipValue := false
	for _, a := range args {
		switch {
		case skipValue:
			skipValue = false
		case a == "--":
			continue // the next token is positional by definition
		case len(a) > 1 && a[0] == '-':
			flagName := strings.TrimLeft(a, "-")
			if strings.IndexByte(flagName, '=') >= 0 {
				continue // --flag=value consumes no further token
			}
			// An unknown flag is left to cobra's parser, which raises its own
			// usage error; a known non-bool flag eats the following token.
			if f := lookupFlag(flags, flagName); f != nil && f.Value.Type() != "bool" {
				skipValue = true
			}
		default:
			return a
		}
	}
	return ""
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
