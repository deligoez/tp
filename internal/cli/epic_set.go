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
const epicSetUsageHint = "tp epic set <id> <field>=<value> or tp epic set <milestones|gates|tag_checks>=<json>"

func newEpicSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [id] <field>=<value>",
		Short: "Set an epic field, or replace milestones, gates or tag_checks whole; a write that adds a lint error exits 1 and writes nothing",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runEpicSet,
	}
}

// runEpicSet implements tp epic set (spec/1.3.0.md §3).
func runEpicSet(_ *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic set")
	key, value, ok := strings.Cut(args[len(args)-1], "=")
	if !ok {
		epicSetUsageExit("expected <field>=<value>, got " + args[len(args)-1])
	}
	ids := []string{}
	var mutate func(*model.EpicGraph) error
	var err error
	if len(args) == 2 {
		ids = append(ids, args[0])
		mutate, err = engine.ParseEpicFieldSet(args[0], key, value)
	} else {
		mutate, err = engine.ParseEpicGraphSet(key, value)
	}
	if err != nil {
		epicSetUsageExit(err.Error())
	}
	writeEpicGraph(ids, mutate)
	return nil
}

// epicSetUsageExit refuses a malformed set before any lock is taken.
func epicSetUsageExit(msg string) {
	output.Error(ExitUsage, msg, epicSetUsageHint)
	os.Exit(ExitUsage)
}
