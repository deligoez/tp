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
	applyDefault("no-color", "color", "no_color", &flagNoColor)

	// Negating flags force the value off, overriding a default.
	if c.Flags().Changed("no-compact") {
		flagCompact = false
	}
	if c.Flags().Changed("no-quiet") {
		flagQuiet = false
	}
	if c.Flags().Changed("color") {
		flagNoColor = false
	}
}
