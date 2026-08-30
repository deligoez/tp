package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// arityUsageError wraps a cobra Args-validator failure so Execute() routes it
// to the tp error contract at exit 2 (spec §8a.5). Cobra returns a plain error
// from ValidateArgs, which the dispatcher's last-resort branch classified as a
// validation failure (exit 1) — the code that means "tp ran the request and it
// failed". A wrong argument count is the opposite: tp never ran anything, which
// is what exit 2 tells an unattended driver. The failing command is captured so
// the hint can name the argument it wanted.
type arityUsageError struct {
	cmd *cobra.Command
	err error
}

func (e arityUsageError) Error() string { return e.err.Error() }
func (e arityUsageError) Unwrap() error { return e.err }

// wrapArityErrors replaces every command's Args validator with one that tags
// its rejection as a usage error, walking the whole tree from cmd down. A
// command whose Args is nil is left alone: cobra substitutes ArbitraryArgs for
// it (command.go ValidateArgs), which accepts everything and so can never
// produce an arity violation to reclassify.
//
// It must run after cobra's lazily registered built-ins (help, completion and
// its per-shell subcommands, which carry cobra.NoArgs) are on the tree —
// otherwise those keep the old exit 1.
func wrapArityErrors(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		wrapArityErrors(sub)
	}
	inner := cmd.Args
	if inner == nil {
		return
	}
	cmd.Args = func(c *cobra.Command, args []string) error {
		if err := inner(c, args); err != nil {
			return arityUsageError{cmd: c, err: err}
		}
		return nil
	}
}

// arityHint answers §8a.5's requirement that the hint name the missing argument
// rather than claim to name the failing object: at exit 1 an arity violation
// inherited runtimeFailureHint, which points at the task file and tp validate —
// neither of which is at fault when the id was simply never typed. The argument
// names come from the command's own Use line ("show <id>"), so the hint cannot
// drift from the usage text. A command that takes no positionals at all (each
// completion shell) has nothing to name, so it falls back to --help.
func arityHint(cmd *cobra.Command) string {
	if args := useArgs(cmd.Use); args != "" {
		return fmt.Sprintf("usage: %s %s", cmd.CommandPath(), args)
	}
	return fmt.Sprintf("run '%s --help' for usage", cmd.CommandPath())
}

// useArgs returns the positional-argument part of a cobra Use line — everything
// after the command name — or "" when the line names no positionals.
func useArgs(use string) string {
	fields := strings.Fields(use)
	if len(fields) < 2 {
		return ""
	}
	return strings.Join(fields[1:], " ")
}
