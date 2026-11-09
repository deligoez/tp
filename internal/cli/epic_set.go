package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicNotFoundHint answers a write naming an epic the graph does not hold.
const epicNotFoundHint = "tp epic lint lists the file's epics; tp epic add adds one"

// epicSetUsageHint answers a malformed tp epic set call.
const epicSetUsageHint = "tp epic set <id> <field>=<value>, tp epic set <milestones|opens_after|gates|tag_checks>=<json>, or tp epic set gates.<milestone>.<threshold|deferrable|members>=<value>"

func newEpicSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [id] <field>=<value>",
		Short: "Set an epic field, replace milestones, opens_after, gates or tag_checks whole, or edit one gate's threshold, deferrable or members; a write that adds a lint error exits 1 and writes nothing",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runEpicSet,
	}
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicSet implements tp epic set (spec/1.3.0.md §3).
func runEpicSet(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic set")
	key, value, ok := strings.Cut(args[len(args)-1], "=")
	if !ok {
		epicSetUsageExit("expected <field>=<value>, got " + args[len(args)-1])
	}
	var mutate func(*model.EpicGraph) error
	var err error
	if len(args) == 2 {
		mutate, err = engine.ParseEpicFieldSet(args[0], key, value)
	} else {
		mutate, err = engine.ParseEpicGraphSet(key, value)
	}
	if err != nil {
		epicSetUsageExit(err.Error())
	}
	opts := epicWriteOpts{ifRevision: epicIfRevision(cmd)}
	if len(args) == 1 && (key == "opens_after" || key == "milestones") {
		mutate = withAncestryReport(mutate, &opts)
	}
	writeEpicGraphWith(opts, mutate)
	return nil
}

// withAncestryReport wraps a set of opens_after or milestones so the response
// lists the milestones whose ancestry the write changed (spec/1.6.0.md §1).
func withAncestryReport(mutate func(*model.EpicGraph) error, opts *epicWriteOpts) func(*model.EpicGraph) error {
	changes := []engine.AncestryChange{}
	opts.ancestry = &changes
	return func(g *model.EpicGraph) error {
		before := &model.EpicGraph{Milestones: g.Milestones, OpensAfter: g.OpensAfter}
		if err := mutate(g); err != nil {
			return err
		}
		changes = engine.EpicAncestryChanges(before, g)
		return nil
	}
}

// epicSetUsageExit refuses a malformed set before any lock is taken.
func epicSetUsageExit(msg string) {
	output.Error(ExitUsage, msg, epicSetUsageHint)
	os.Exit(ExitUsage)
}
