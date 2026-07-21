package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// applyFlagDefaults reconciles the boolean global flags with their
// .tp/local.json defaults. Precedence, highest first: an explicit flag or its
// negation, then the local default, then the built-in false. Passing a flag and
// its negation together is a usage error (exit 2). --json is excluded.
func applyFlagDefaults(c *cobra.Command) {
	conflict := func(pos, neg string) {
		if c.Flags().Changed(pos) && c.Flags().Changed(neg) {
			output.Error(ExitUsage, fmt.Sprintf("--%s and --%s cannot be used together", pos, neg))
			os.Exit(ExitUsage)
		}
	}
	conflict("compact", "no-compact")
	conflict("quiet", "no-quiet")
	conflict("no-color", "color")

	defaults := engine.LocalFlagDefaults(".")
	applyDefault := func(posFlag, negFlag, key string, target *bool) {
		if c.Flags().Changed(posFlag) || c.Flags().Changed(negFlag) {
			return // an explicit flag or its negation already decides the value
		}
		if v, ok := defaults[key]; ok {
			*target = v
		}
	}
	applyDefault("compact", "no-compact", "compact", &flagCompact)
	applyDefault("quiet", "no-quiet", "quiet", &flagQuiet)

	// Negating flags force compact/quiet off, overriding a default.
	if c.Flags().Changed("no-compact") {
		flagCompact = false
	}
	if c.Flags().Changed("no-quiet") {
		flagQuiet = false
	}

	// no_color resolution order: explicit --color/--no-color flag, then the
	// NO_COLOR environment variable, then the local default, then TTY detection
	// (handled downstream by fatih/color when flagNoColor stays false).
	switch {
	case c.Flags().Changed("color"):
		flagNoColor = false
	case c.Flags().Changed("no-color"):
		flagNoColor = true
	case os.Getenv("NO_COLOR") != "":
		flagNoColor = true
	default:
		if v, ok := defaults["no_color"]; ok {
			flagNoColor = v
		}
	}
}
