package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestEveryModeHasASide is spec/backlog/project-layer-from-task-file.md's
// placement rule, by mode: the side of a mode — work in the working tree, the
// task file only, or no task file — is not written from memory. Every command
// in the tree has its default mode placed in commandSides, every mode flag
// commandModeFlags names exists on its command and is placed as
// "<path> --<flag>", and every entry names a mode that exists, so a new
// command or mode fails here until someone decides its side.
func TestEveryModeHasASide(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			path := sub.CommandPath()
			seen[path] = true
			assertPlaced(t, path)
			for _, flag := range commandModeFlags[path] {
				assert.NotNil(t, sub.Flags().Lookup(flag), "%s has no --%s flag to select a mode", path, flag)
				mode := path + " --" + flag
				seen[mode] = true
				assertPlaced(t, mode)
			}
			walk(sub)
		}
	}
	walk(newRootCmdForTest())

	stale := make([]string, 0)
	for mode := range commandSides {
		if !seen[mode] {
			stale = append(stale, mode)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale, "commandSides names modes the tree does not have")

	for path := range projectLayerReaders {
		assert.Equal(t, sideFile, commandSides[path], "%s reads the project layer, so it takes a task file", path)
	}
}

func assertPlaced(t *testing.T, mode string) {
	t.Helper()
	side, ok := commandSides[mode]
	if assert.True(t, ok, "%s has no side in commandSides: place it as sideTree, sideFile or sideNone", mode) {
		assert.Contains(t, []string{sideTree, sideFile, sideNone}, side, "%s", mode)
	}
	assert.False(t, strings.HasSuffix(mode, " "), "%s", mode)
}
