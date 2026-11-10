package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicReportUsageHint answers a --report call tp epic review refuses.
const epicReportUsageHint = "run tp epic review --report --all (every slice directory) or --report --milestones <m,m,...>, alone"

// epicReportResult is tp epic review --report's payload (spec/1.7.0.md §3):
// each finding class with the slices, rounds and distinct epics it was raised
// on. It suggests no mechanisation: epic review has no registered checks to
// retire a candidate.
type epicReportResult struct {
	Classes []engine.EpicClassRaised `json:"classes"`
}

// epicReportAlone lists the modes --report refuses beside it.
var epicReportAlone = []string{"status", "check", "role"}

// refuseEpicReportOutsideBatch exits 2 when --report comes without --all or
// --milestones: the report reads slices across the graph, never one.
func refuseEpicReportOutsideBatch(cmd *cobra.Command) {
	if report, _ := cmd.Flags().GetBool("report"); report {
		output.Error(ExitUsage, "--report takes --all or --milestones: it reads classes across slices", epicReportUsageHint)
		os.Exit(ExitUsage)
	}
}

// runEpicReviewReport prints the classes the scope's recorded rounds raised.
// It reads and writes no state but the round files; one tp cannot read exits 3.
func runEpicReviewReport(cmd *cobra.Command, scope epicBatchScope) {
	for _, name := range epicReportAlone {
		if cmd.Flags().Changed(name) {
			output.Error(ExitUsage, "--report takes no --"+name+": it is its own mode", epicReportUsageHint)
			os.Exit(ExitUsage)
		}
	}
	eg := loadEpicReviewGraph()
	classes, err := engine.EpicReviewClasses(epicBatchStatusDirs(&eg, scope))
	if err != nil {
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	if err := output.JSON(epicReportResult{Classes: classes}); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}
