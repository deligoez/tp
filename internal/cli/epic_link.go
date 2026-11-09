package cli

import (
	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
)

// epicEdgeNotFoundHint answers an unlink of an edge the graph does not hold.
const epicEdgeNotFoundHint = "tp epic export --md lists each epic's edges; tp epic link adds one"

// epicLinkRefusedHint answers a link whose edge would add a lint error: an
// empty why, or a target the graph does not hold.
const epicLinkRefusedHint = "give the edge a non-empty why naming what <to> supplies, and a <to> the graph holds (tp epic add adds it)"

func newEpicLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <from> <to> <why>",
		Short: "Add the edge from -> to with its why, or replace that edge's why; a write that adds a lint error exits 1 and writes nothing",
		Args:  cobra.ExactArgs(3),
		RunE:  runEpicLink,
	}
	epicIfRevisionFlag(cmd)
	return cmd
}

func newEpicUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <from> <to>",
		Short: "Remove the edge from -> to; an edge the graph does not hold exits 4",
		Args:  cobra.ExactArgs(2),
		RunE:  runEpicUnlink,
	}
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicLink implements tp epic link (spec/1.4.0.md §1).
func runEpicLink(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic link")
	var action string
	writeEpicGraphWith(epicWriteOpts{ifRevision: epicIfRevision(cmd), action: &action, refusedHint: epicLinkRefusedHint},
		engine.LinkEpic(args[0], args[1], args[2], &action))
	return nil
}

// runEpicUnlink implements tp epic unlink (spec/1.4.0.md §1).
func runEpicUnlink(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic unlink")
	var action string
	writeEpicGraphWith(epicWriteOpts{ifRevision: epicIfRevision(cmd), action: &action},
		engine.UnlinkEpic(args[0], args[1], &action))
	return nil
}
