package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roleFlagOn returns the --role flag as the named command registers it, or nil
// when the command does not register it at all.
func roleFlagOn(t *testing.T, name string) *pflag.Flag {
	t.Helper()
	root := NewRootCmd()
	for _, c := range root.Commands() {
		if c.Name() == name {
			return c.Flags().Lookup("role")
		}
	}
	t.Fatalf("command %q not found on the root command", name)
	return nil
}

// TestRoleFlagIsRegisteredOnBothCommands pins v0.36.0 §4.2's flag surface.
//
// The shape matters as much as the presence. --role selects one entry from a
// panel, so it takes exactly one value: a repeatable flag would silently keep
// the last occurrence and give two spellings of the same request different
// meanings, and a boolean could not name a role at all.
func TestRoleFlagIsRegisteredOnBothCommands(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"review", "audit"} {
		t.Run(cmd, func(t *testing.T) {
			f := roleFlagOn(t, cmd)
			require.NotNil(t, f, "tp %s must register --role", cmd)
			assert.Equal(t, "string", f.Value.Type(),
				"--role takes one name, so it is a string rather than a repeatable slice or a bool")
			assert.Empty(t, f.DefValue,
				"--role defaults to empty, which is how an absent flag is told from a named role")
			assert.NotEmpty(t, f.Usage, "--role needs usage text: it appears in tp %s --help", cmd)
		})
	}
}
