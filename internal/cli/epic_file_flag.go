package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// flagEpicFile is --epic on tp resume, tp next and tp ready: the epic whose
// bound task file the call works on (spec/1.6.0.md F8).
var flagEpicFile string

// epicFileFlagHint answers an --epic that resolves to no single file.
const epicFileFlagHint = "pass --file with the task file to work on, or bind one to the epic (tp set --epic, tp init --epic)"

// addEpicFileFlag registers --epic on cmd.
func addEpicFileFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flagEpicFile, "epic", "",
		"work on the task file bound to this epic: its only one, or the only one not at release")
}

// applyEpicFileFlag turns --epic into the --file of this call. --epic with
// --file is a usage error (exit 2); an epic binding no file, or several none
// or more than one of which is short of release, exits 4 naming them.
func applyEpicFileFlag() {
	if flagEpicFile == "" {
		return
	}
	if flagFile != "" {
		output.Error(ExitUsage, "--epic and --file are mutually exclusive", "pass one: --epic picks the file --file would name")
		os.Exit(ExitUsage)
	}
	path, err := engine.EpicBoundFile(".", flagEpicFile)
	var refused *engine.EpicBoundFileError
	if errors.As(err, &refused) {
		labels := make([]string, 0, len(refused.Files))
		for _, f := range refused.Files {
			labels = append(labels, taskFileLabel(f))
		}
		output.Error(ExitState, refused.Message(labels), epicFileFlagHint)
		os.Exit(ExitState)
	}
	if err != nil {
		// task-file-hint: an unreadable task file or review state
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	flagFile = path
}
