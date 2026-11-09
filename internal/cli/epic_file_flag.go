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

// epicFileFlagUnknownHint answers an --epic naming an id the graph lacks.
const epicFileFlagUnknownHint = "name an epic the graph holds (tp epic status lists them), or pass --file"

// refuseEpicFileFlag exits 4 on --epic's refusal. Several candidates ride as
// a "candidates" array of --file arguments, the error naming them too.
func refuseEpicFileFlag(refused *engine.EpicBoundFileError) {
	if refused.Unknown {
		output.Error(ExitState, refused.Message(nil), epicFileFlagUnknownHint)
		os.Exit(ExitState)
	}
	labels := make([]string, 0, len(refused.Files))
	for _, f := range refused.Files {
		labels = append(labels, taskFileLabel(f))
	}
	if len(labels) == 0 {
		output.Error(ExitState, refused.Message(labels), epicFileFlagHint)
	} else {
		output.ErrorExtras(ExitState, refused.Message(labels), map[string]any{"candidates": labels}, epicFileFlagHint)
	}
	os.Exit(ExitState)
}

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
		refuseEpicFileFlag(refused)
	}
	if err != nil {
		// task-file-hint: an unreadable task file or review state
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	flagFile = path
}
