package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicHistoryResult is tp epic review <slice> --history's payload
// (spec/1.8.0.md §11): the slice's state directory and its history.
type epicHistoryResult struct {
	StateDir string `json:"state_dir"`
	engine.EpicHistory
}

// epicHistoryNoStateHint answers a slice no round was ever emitted for.
const epicHistoryNoStateHint = "the slice has no recorded round yet: emit one with tp epic review <slice> and record it with --record <file>"

// runEpicReviewHistoryIfSet prints the slice's history when --history is
// set, and reports whether it did. --from or --to without it, or one without
// the other, is a usage error.
func runEpicReviewHistoryIfSet(cmd *cobra.Command, key engine.EpicSliceKey) bool {
	history, _ := cmd.Flags().GetBool("history")
	from, _ := cmd.Flags().GetInt("from")
	to, _ := cmd.Flags().GetInt("to")
	narrowed := cmd.Flags().Changed("from") || cmd.Flags().Changed("to")
	switch {
	case narrowed && !history:
		output.Error(ExitUsage, "--from and --to narrow --history and need it", epicReviewUsageHint)
		os.Exit(ExitUsage)
	case cmd.Flags().Changed("from") != cmd.Flags().Changed("to"):
		output.Error(ExitUsage, "--from and --to name two rounds and come together", epicReviewUsageHint)
		os.Exit(ExitUsage)
	case !history:
		return false
	}
	runEpicReviewHistory(key, from, to)
	return true
}

// runEpicReviewHistory prints the slice's rounds with their findings by role
// and differences, writing nothing. A slice with no state directory exits 3,
// a round it did not record 4.
func runEpicReviewHistory(key engine.EpicSliceKey, from, to int) {
	run := loadEpicReviewRun(key)
	if info, err := os.Stat(run.stateDir); err != nil || !info.IsDir() {
		output.Error(ExitFile, "epic review state not found for "+epicSliceLabel(key)+": "+epicReviewStateDirOut(&run), epicHistoryNoStateHint)
		os.Exit(ExitFile)
	}
	h, err := engine.EpicReviewHistory(run.stateDir, run.all, from, to)
	if err != nil {
		if errors.As(err, new(*engine.EpicRoundOutOfRangeError)) {
			output.Error(ExitState, err.Error(), "pass --from and --to rounds the slice recorded; tp epic review <slice> --status prints its round")
			os.Exit(ExitState)
		}
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	if IsCompact() {
		h.WithoutContext()
	}
	printResultOrExit(epicHistoryResult{StateDir: epicReviewStateDirOut(&run), EpicHistory: h})
}
