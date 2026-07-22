package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	keepRemove bool
	keepList   bool
)

func newKeepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keep <path> <reason>",
		Short: "Record a deliberately-uncommitted file in the keep-list",
		Long: `Manage the keep-list — the durable, git-ignored memory (.tp/local.json) of
files a project deliberately leaves uncommitted, so tp resume classifies them as
known-intentional rather than unexplained changes.

  tp keep <path> "<reason>"   add or update an entry (a repeated path overwrites)
  tp keep --remove <path>     drop an entry (an absent path is a no-op, exit 0)
  tp keep --list              print the keep-list as JSON ([] when empty)`,
		Example: `  tp keep .env.local "developer secrets, never committed"
  tp keep "build/*.log" "build logs"
  tp keep --list`,
		Args: cobra.ArbitraryArgs,
		RunE: runKeep,
	}
	cmd.Flags().BoolVar(&keepRemove, "remove", false, "remove the keep-list entry for <path>")
	cmd.Flags().BoolVar(&keepList, "list", false, "print the keep-list as JSON")
	return cmd
}

func runKeep(_ *cobra.Command, args []string) error {
	switch {
	case keepList:
		return outputKeepList()
	case keepRemove:
		return runKeepRemove(args)
	default:
		return runKeepAdd(args)
	}
}

// runKeepAdd adds or updates a keep-list entry (§7.2). A missing reason or a
// malformed glob is a usage error (exit 2); the path is stored repo-root-relative.
func runKeepAdd(args []string) error {
	if len(args) < 2 {
		output.Error(ExitUsage, "tp keep requires a <path> and a <reason>", `usage: tp keep <path> "<reason>"`)
		os.Exit(ExitUsage)
		return nil
	}
	path, reason := args[0], args[1]
	if _, err := filepath.Match(path, ""); errors.Is(err, filepath.ErrBadPattern) {
		output.Error(ExitUsage, fmt.Sprintf("malformed keep pattern %q: %v", path, err))
		os.Exit(ExitUsage)
		return nil
	}
	norm, err := engine.NormalizeKeepPath(".", path)
	if err != nil {
		output.Error(ExitUsage, fmt.Sprintf("cannot normalize %q: %v", path, err))
		os.Exit(ExitUsage)
		return nil
	}
	if err := engine.UpdateKeepList(".", func(cur []model.KeepEntry) []model.KeepEntry {
		out := make([]model.KeepEntry, 0, len(cur)+1)
		replaced := false
		for _, e := range cur {
			if e.Path == norm {
				out = append(out, model.KeepEntry{Path: norm, Reason: reason})
				replaced = true
			} else {
				out = append(out, e)
			}
		}
		if !replaced {
			out = append(out, model.KeepEntry{Path: norm, Reason: reason})
		}
		return out
	}); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	output.Success(fmt.Sprintf("keeping %s", norm))
	return outputKeepList()
}

// runKeepRemove drops the keep-list entry whose stored path equals the
// normalized <path>; an absent path is a no-op at exit 0 (§7.2).
func runKeepRemove(args []string) error {
	if len(args) != 1 {
		output.Error(ExitUsage, "tp keep --remove requires exactly one <path>")
		os.Exit(ExitUsage)
		return nil
	}
	norm, err := engine.NormalizeKeepPath(".", args[0])
	if err != nil {
		output.Error(ExitUsage, fmt.Sprintf("cannot normalize %q: %v", args[0], err))
		os.Exit(ExitUsage)
		return nil
	}
	if err := engine.UpdateKeepList(".", func(cur []model.KeepEntry) []model.KeepEntry {
		out := make([]model.KeepEntry, 0, len(cur))
		for _, e := range cur {
			if e.Path != norm {
				out = append(out, e)
			}
		}
		return out
	}); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	output.Success(fmt.Sprintf("removed %s", norm))
	return outputKeepList()
}

// outputKeepList prints the current keep-list as JSON, emitting [] (never null)
// when empty.
func outputKeepList() error {
	entries := engine.LoadKeepList(".")
	if entries == nil {
		entries = []model.KeepEntry{}
	}
	return output.JSON(entries)
}
