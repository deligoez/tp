package cli

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicDoneUsageHint answers a malformed tp epic done call.
const epicDoneUsageHint = "tp epic done <id> --evidence \"<text or URL>\" or tp epic done <id> --undo"

// epicDoneRefusedHint answers a done record refused on an epic.
const epicDoneRefusedHint = "an epic with a bound file is done when its file reaches release; a condition records done once its acceptance says what it waits for (tp epic set <id> acceptance=<text>)"

func newEpicDoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done <id>",
		Short: "Record an epic with no bound file done on evidence, or remove the record with --undo; refused on a bound epic and on a condition with an empty acceptance",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicDone,
	}
	cmd.Flags().String("evidence", "", "what shows the epic done, a text or a URL")
	cmd.Flags().Bool("undo", false, "remove the epic's done record")
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicDone implements tp epic done (spec/1.5.0.md §5): exactly one of
// --evidence and --undo (else exit 2), then one locked write that exits 1 on
// a refusal and 4 for an absent epic.
func runEpicDone(cmd *cobra.Command, args []string) error {
	fenceEpicOperatorWrite("tp epic done", engine.EscalateEpicDone)
	evidence, _ := cmd.Flags().GetString("evidence")
	undo, _ := cmd.Flags().GetBool("undo")
	if undo == (evidence != "") {
		output.Error(ExitUsage, "exactly one of --evidence and --undo is required", epicDoneUsageHint)
		os.Exit(ExitUsage)
	}
	if undo {
		writeEpicGraph(cmd, engine.UndoEpicDone(args[0]))
		return nil
	}
	writeEpicGraph(cmd, engine.RecordEpicDone(args[0], evidence, ".", time.Now()))
	return nil
}
