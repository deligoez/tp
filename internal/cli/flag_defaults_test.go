package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func flagCmd() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().Bool("compact", false, "")
	c.Flags().Bool("quiet", false, "")
	c.Flags().Bool("no-color", false, "")
	c.Flags().Bool("no-compact", false, "")
	c.Flags().Bool("no-quiet", false, "")
	c.Flags().Bool("color", false, "")
	return c
}

func writeLocalDefaults(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"), []byte(body), 0o600))
	return root
}

func TestApplyFlagDefaults_AppliesWhenFlagAbsent(t *testing.T) {
	root := writeLocalDefaults(t, `{"defaults":{"compact":true,"quiet":true}}`)
	t.Chdir(root)
	flagCompact, flagQuiet = false, false
	defer func() { flagCompact, flagQuiet = false, false }()

	applyFlagDefaults(flagCmd())
	assert.True(t, flagCompact, "compact default applies when the flag is absent")
	assert.True(t, flagQuiet, "quiet default applies")
}

func TestApplyFlagDefaults_ExplicitFlagWins(t *testing.T) {
	root := writeLocalDefaults(t, `{"defaults":{"compact":true}}`)
	t.Chdir(root)
	flagCompact = false
	defer func() { flagCompact = false }()

	c := flagCmd()
	require.NoError(t, c.Flags().Set("compact", "false")) // marks the flag as changed

	applyFlagDefaults(c)
	assert.False(t, flagCompact, "an explicit CLI flag wins over the local default")
}

func TestApplyFlagDefaults_NegatingFlagOverridesDefault(t *testing.T) {
	root := writeLocalDefaults(t, `{"defaults":{"compact":true}}`)
	t.Chdir(root)
	flagCompact = false
	defer func() { flagCompact = false }()

	c := flagCmd()
	require.NoError(t, c.Flags().Set("no-compact", "true"))

	applyFlagDefaults(c)
	assert.False(t, flagCompact, "--no-compact turns off a compact default")
}
