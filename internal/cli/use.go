package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

const tpActiveFile = ".tp-active"

var useClear bool

func newUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use [file]",
		Short: "Set active task file for this project directory",
		Long: `Set, show, or clear the active task file.
Writes the pointer to .tp/local.json (git-ignored); discovery reads it after
--file and TP_FILE, before auto-detect.`,
		Example: `  tp use spec.tasks.json    # set the active task file
  tp use --clear            # clear the active pointer
  tp use                    # show the current active file`,
		Args: cobra.MaximumNArgs(1),
		RunE: runUse,
	}
	cmd.Flags().BoolVar(&useClear, "clear", false, "clear the active pointer")
	return cmd
}

func runUse(_ *cobra.Command, args []string) error {
	surfaceConfigWarnings()
	// --clear with positional arg is a usage error
	if useClear && len(args) > 0 {
		output.Error(ExitUsage, "--clear and file argument are mutually exclusive")
		os.Exit(ExitUsage)
		return nil
	}

	if useClear {
		return clearActiveFile()
	}

	if len(args) == 0 {
		return showActiveFile()
	}

	return setActiveFile(args[0])
}

// resolvedActiveSource walks the discovery chain and returns the resolved
// active task file and the rank that supplied it (cli/env/local/autodetect),
// or ("", "") when nothing resolves. The legacy .tp-active marker was removed
// in v0.25.0 and is no longer part of the chain.
func resolvedActiveSource() (path, source string) {
	if flagFile != "" {
		return flagFile, "cli"
	}
	if env := os.Getenv("TP_FILE"); env != "" {
		return env, "env"
	}
	if active := engine.ResolveLocalActive("."); active != "" {
		if _, err := os.Stat(active); err == nil {
			return active, "local"
		}
	}
	if p, err := engine.DiscoverTaskFile(".", ""); err == nil {
		return p, "autodetect"
	}
	return "", ""
}

// showActiveFile prints the resolved active task file and the discovery-chain
// rank that supplied it, and reports a dangling .tp/local.json active pointer
// (one that is set but points at a missing file).
func showActiveFile() error {
	out := map[string]any{}
	if active := engine.ResolveLocalActive("."); active != "" {
		if _, err := os.Stat(active); err != nil {
			out["dangling_active"] = active
		}
	}
	path, source := resolvedActiveSource()
	if path == "" {
		out["active_file"] = nil
		out["source"] = nil
	} else {
		out["active_file"] = path
		out["source"] = source
	}
	return output.JSON(out)
}

// clearActiveFile removes the active pointer from .tp/local.json and any
// leftover legacy .tp-active in the working directory. It is a no-op with
// exit 0 when no active pointer is set.
func clearActiveFile() error {
	if err := os.Remove(tpActiveFile); err != nil && !os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("cannot remove %s: %v", tpActiveFile, err))
		os.Exit(ExitFile)
		return nil
	}
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		return output.JSON(map[string]any{"cleared": true})
	}
	return engine.WithFileLock(filepath.Join(tpDir, "local.json"), func() error {
		lc, _, err := engine.LoadLocalConfig(tpDir)
		if err != nil {
			var mce *engine.MalformedConfigError
			if errors.As(err, &mce) {
				output.Error(ExitFile, mce.Error(), mce.Hint())
			} else {
				output.Error(ExitFile, err.Error())
			}
			os.Exit(ExitFile)
			return nil
		}
		if lc.Active == nil {
			return output.JSON(map[string]any{"cleared": true}) // no-op
		}
		lc.Active = nil
		if err := engine.WriteLocalConfig(tpDir, lc); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		return output.JSON(map[string]any{"cleared": true})
	})
}

// setActiveFile stores file (resolved project-root-relative) in
// .tp/local.json active, rejecting a target outside the project root with
// exit 2 and no longer writing a .tp-active marker.
func setActiveFile(file string) error {
	abs, err := filepath.Abs(file)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	if _, statErr := os.Stat(abs); os.IsNotExist(statErr) {
		output.Error(ExitFile, fmt.Sprintf("file not found: %s", file))
		os.Exit(ExitFile)
		return nil
	}
	root := engine.ProjectRoot(".")
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		output.Error(ExitUsage, fmt.Sprintf("%s is outside the project root %s", file, root))
		os.Exit(ExitUsage)
		return nil
	}
	tpDir := engine.ProjectConfigDir(".")
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	return engine.WithFileLock(filepath.Join(tpDir, "local.json"), func() error {
		lc, _, loadErr := engine.LoadLocalConfig(tpDir)
		if loadErr != nil {
			var mce *engine.MalformedConfigError
			if errors.As(loadErr, &mce) {
				output.Error(ExitFile, mce.Error(), mce.Hint())
			} else {
				output.Error(ExitFile, loadErr.Error())
			}
			os.Exit(ExitFile)
			return nil
		}
		lc.Active = &rel
		if err := engine.WriteLocalConfig(tpDir, lc); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		fmt.Fprintf(os.Stderr, "active task file set to %s\n", rel)
		return output.JSON(map[string]any{"active_file": rel})
	})
}
