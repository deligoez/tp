package cli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicHistoryResult is tp epic review <slice> --history's payload
// (spec/1.8.0.md §11): the slice's state directory and its history.
type epicHistoryResult struct {
	StateDir string `json:"state_dir"`
	engine.EpicHistory
}

// epicOfEpicResult is tp epic review --history --of-epic <id>'s payload
// (spec/1.8.0.md §11): the epic, its edges per recorded round of each slice
// holding it, and a message when no recorded slice holds it.
type epicOfEpicResult struct {
	Epic    string                  `json:"epic"`
	Slices  []engine.EpicSliceEdges `json:"slices"`
	Message string                  `json:"message,omitempty"`
}

// epicOfEpicUsageHint answers an --of-epic call that is not its own mode.
const epicOfEpicUsageHint = "run tp epic review --history --of-epic <id> alone: it reads every slice, so it takes no slice flag and no --from or --to"

// runEpicReviewOfEpicIfSet prints one epic's graph history when --of-epic is
// set, and reports whether it did. It needs --history and takes no other flag
// of the command.
func runEpicReviewOfEpicIfSet(cmd *cobra.Command) bool {
	if !cmd.Flags().Changed("of-epic") {
		return false
	}
	if history, _ := cmd.Flags().GetBool("history"); !history {
		output.Error(ExitUsage, "--of-epic narrows --history and needs it", epicOfEpicUsageHint)
		os.Exit(ExitUsage)
	}
	own := cmd.LocalNonPersistentFlags()
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if own.Lookup(f.Name) != nil && f.Name != "history" && f.Name != "of-epic" {
			output.Error(ExitUsage, "--of-epic takes no --"+f.Name, epicOfEpicUsageHint)
			os.Exit(ExitUsage)
		}
	})
	id, _ := cmd.Flags().GetString("of-epic")
	if id == "" {
		output.Error(ExitUsage, "--of-epic needs an epic id", epicOfEpicUsageHint)
		os.Exit(ExitUsage)
	}
	runEpicReviewOfEpic(id)
	return true
}

// runEpicReviewOfEpic prints epic id's edges and whys per recorded round of
// every slice whose snapshot holds it, writing nothing.
func runEpicReviewOfEpic(id string) {
	eg := loadEpicReviewGraph()
	dirs, err := engine.EpicReviewSliceDirs(eg.tpDir, eg.g)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	slices, err := engine.EpicEdgeHistory(dirs, id)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	res := epicOfEpicResult{Epic: id, Slices: slices}
	for i := range res.Slices {
		if rel, err := filepath.Rel(filepath.Dir(eg.tpDir), res.Slices[i].StateDir); err == nil {
			res.Slices[i].StateDir = rel
		}
	}
	if len(slices) == 0 {
		res.Message = "epic " + id + " is in no recorded slice: no snapshot of a recorded round holds it, so it has no graph history"
	}
	printResultOrExit(res)
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
