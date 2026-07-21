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
Writes a .tp-active marker in CWD. Discovery reads it after TP_FILE, before dir scan.`,
		Example: `  tp use spec.tasks.json    # set active task file
  tp use --clear            # remove .tp-active
  tp use                    # show current active file`,
		Args: cobra.MaximumNArgs(1),
		RunE: runUse,
	}
	cmd.Flags().BoolVar(&useClear, "clear", false, "remove .tp-active marker")
	return cmd
}

func runUse(_ *cobra.Command, args []string) error {
	// --clear with positional arg is a usage error
	if useClear && len(args) > 0 {
		output.Error(ExitUsage, "--clear and file argument are mutually exclusive")
		os.Exit(ExitUsage)
		return nil
	}

	if useClear {
		// Remove .tp-active (idempotent)
		if err := os.Remove(tpActiveFile); err != nil && !os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("cannot remove %s: %v", tpActiveFile, err))
			os.Exit(ExitFile)
			return nil
		}
		return nil
	}

	if len(args) == 0 {
		// Show current active file
		data, err := os.ReadFile(tpActiveFile)
		if err != nil {
			if os.IsNotExist(err) {
				return output.JSON(map[string]any{"active_file": nil})
			}
			output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", tpActiveFile, err))
			os.Exit(ExitFile)
			return nil
		}
		path := strings.TrimSpace(string(data))
		if path == "" {
			return output.JSON(map[string]any{"active_file": nil})
		}
		return output.JSON(map[string]any{"active_file": path})
	}

	return setActiveFile(args[0])
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
