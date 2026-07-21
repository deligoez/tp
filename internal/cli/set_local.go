package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// localDefaultFlags are the boolean global flags settable in .tp/local.json.
var localDefaultFlags = map[string]bool{"compact": true, "quiet": true, "no_color": true}

// runSetLocal implements `tp set --local defaults.<flag>=<bool>`, writing a
// global flag default to .tp/local.json. <flag> must be compact, quiet, or
// no_color and <bool> must be true or false; anything else is rejected with
// exit 1. Writes acquire the standard flock.
func runSetLocal(args []string) error {
	if len(args) == 0 {
		output.Error(ExitUsage, "tp set --local requires at least one defaults.<flag>=<bool> pair")
		os.Exit(ExitUsage)
		return nil
	}
	surfaceConfigWarnings()

	defaults := make(map[string]bool)
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			output.Error(ExitUsage, fmt.Sprintf("expected defaults.<flag>=<bool>, got %q", arg))
			os.Exit(ExitUsage)
			return nil
		}
		flag, ok := strings.CutPrefix(parts[0], "defaults.")
		if !ok {
			output.Error(ExitValidation, "tp set --local only accepts defaults.<flag>=<bool>")
			os.Exit(ExitValidation)
			return nil
		}
		if !localDefaultFlags[flag] {
			output.Error(ExitValidation, fmt.Sprintf("unknown flag default: %s (want compact, quiet, or no_color)", flag))
			os.Exit(ExitValidation)
			return nil
		}
		switch parts[1] {
		case "true":
			defaults[flag] = true
		case "false":
			defaults[flag] = false
		default:
			output.Error(ExitValidation, fmt.Sprintf("defaults.%s must be true or false", flag))
			os.Exit(ExitValidation)
			return nil
		}
	}

	tpDir := engine.ProjectConfigDir(".")
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
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
		if lc.Defaults == nil {
			lc.Defaults = make(map[string]bool)
		}
		for k, v := range defaults {
			lc.Defaults[k] = v
		}
		if err := engine.WriteLocalConfig(tpDir, lc); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		return output.JSON(map[string]any{"updated": defaults})
	})
}
