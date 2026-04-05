package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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

	// Set active file
	filePath := args[0]
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("file not found: %s", filePath))
		os.Exit(ExitFile)
		return nil
	}

	if err := os.WriteFile(tpActiveFile, []byte(filePath+"\n"), 0o644); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot write %s: %v", tpActiveFile, err))
		os.Exit(ExitFile)
		return nil
	}

	fmt.Fprintf(os.Stderr, "active task file set to %s\n", filePath)
	return output.JSON(map[string]any{"active_file": filePath})
}
