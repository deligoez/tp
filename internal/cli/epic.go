package cli

import "github.com/spf13/cobra"

// newEpicCmd is the parent of the epic graph commands (spec/1.3.0.md): the
// graph lives in .tp/epics.json beside config.json.
func newEpicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Epic graph commands over .tp/epics.json",
	}
	cmd.AddCommand(newEpicLintCmd(), newEpicExportCmd(), newEpicAddCmd(), newEpicSetCmd(), newEpicRemoveCmd(), newEpicImportCmd(), newEpicReviewCmd(),
		newEpicLinkCmd(), newEpicUnlinkCmd(), newEpicMilestoneCmd())
	return cmd
}
