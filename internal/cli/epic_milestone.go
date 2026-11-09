package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicMilestonePlaceHint answers a milestone add with no position or two.
const epicMilestonePlaceHint = "pass exactly one of --after <m>, --before <m> or --new-track"

// epicMilestoneNotFoundHint answers an insert anchored on no milestone.
const epicMilestoneNotFoundHint = "tp epic export --md shows the tracks and their milestones in order"

// epicMilestoneExistsHint answers an insert of a name a track holds.
const epicMilestoneExistsHint = "a milestone name appears once across all tracks; pick another name, or move it with tp epic set milestones=<json>"

func newEpicMilestoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Milestone commands over .tp/epics.json",
	}
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Insert a milestone after or before another in its track, or in a new track",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicMilestoneAdd,
	}
	add.Flags().String("after", "", "insert right after this milestone, in its track")
	add.Flags().String("before", "", "insert right before this milestone, in its track")
	add.Flags().Bool("new-track", false, "open a new track holding only this milestone")
	epicIfRevisionFlag(add)
	cmd.AddCommand(add)
	return cmd
}

// runEpicMilestoneAdd implements tp epic milestone add (spec/1.4.0.md §7).
func runEpicMilestoneAdd(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic milestone add")
	var place engine.EpicMilestonePlace
	place.After, _ = cmd.Flags().GetString("after")
	place.Before, _ = cmd.Flags().GetString("before")
	place.NewTrack, _ = cmd.Flags().GetBool("new-track")
	given := 0
	for _, on := range []bool{cmd.Flags().Changed("after"), cmd.Flags().Changed("before"), place.NewTrack} {
		if on {
			given++
		}
	}
	if given != 1 {
		output.Error(ExitUsage, "tp epic milestone add needs exactly one position", epicMilestonePlaceHint)
		os.Exit(ExitUsage)
	}
	writeEpicGraph(cmd, engine.InsertMilestone(args[0], place))
	return nil
}
