package cli

import (
	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
)

// epicRemoveRefusedHint answers a remove refused for its dependents or its
// tag_checks entries.
const epicRemoveRefusedHint = "--force drops the edges into the epic and its gate memberships; a tag_checks provider must be repointed with tp epic set tag_checks=<json> first"

func newEpicRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an epic from .tp/epics.json; one another epic depends on needs --force",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicRemove,
	}
	cmd.Flags().Bool("force", false, "also drop the edges into the epic and its gate memberships")
	return cmd
}

// runEpicRemove implements tp epic remove (spec/1.3.0.md §3, D8).
func runEpicRemove(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	writeEpicGraph(args, engine.RemoveEpic(args[0], force))
	return nil
}
