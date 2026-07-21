package cli

import (
	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
)

// applyFlagDefaults applies .tp/local.json flag defaults for compact, quiet,
// and no_color when the corresponding CLI flag was not given. --json is
// intentionally excluded (output mode is auto-selected by piping). An explicit
// CLI flag always wins.
func applyFlagDefaults(c *cobra.Command) {
	defaults := engine.LocalFlagDefaults(".")
	if len(defaults) == 0 {
		return
	}
	apply := func(flagName, key string, target *bool) {
		if c.Flags().Changed(flagName) {
			return // explicit CLI flag wins
		}
		if v, ok := defaults[key]; ok {
			*target = v
		}
	}
	apply("compact", "compact", &flagCompact)
	apply("quiet", "quiet", &flagQuiet)
	apply("no-color", "no_color", &flagNoColor)
}
