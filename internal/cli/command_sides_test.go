package cli

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestEveryCommandHasASide is spec/backlog/project-layer-from-task-file.md's
// placement rule: the side of a command — work in the working tree, the task
// file only, or no task file — is not written from memory. Every command in the
// tree is placed in commandSides, and every entry names a command, so a new
// command fails here until someone decides whether it is refused across
// project trees.
func TestEveryCommandHasASide(t *testing.T) {
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
			side, ok := commandSides[path]
			if assert.True(t, ok, "%s has no side in commandSides: place it as sideTree, sideFile or sideNone", path) {
				assert.Contains(t, []string{sideTree, sideFile, sideNone}, side, "%s", path)
			}
			walk(sub)
		}
	}
	walk(NewRootCmd())

	stale := make([]string, 0)
	for path := range commandSides {
		if !seen[path] {
			stale = append(stale, path)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale, "commandSides names commands the tree does not have")

	for path := range projectLayerReaders {
		assert.Equal(t, sideFile, commandSides[path], "%s reads the project layer, so it takes a task file", path)
	}
}
