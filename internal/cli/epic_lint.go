package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicLintResult is tp epic lint's payload (spec/1.3.0.md §4).
type epicLintResult struct {
	Errors   int                      `json:"errors"`
	Warnings int                      `json:"warnings"`
	Findings []engine.EpicLintFinding `json:"findings"`
}

// epicGraphFileHint answers an absent or unparsable .tp/epics.json.
const epicGraphFileHint = "create .tp/epics.json with tp epic set or tp epic import, or fix its JSON to the graph file's schema"

func newEpicLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Lint the epic graph; exit 1 on an error-severity rule, 3 with no graph file",
		Args:  cobra.NoArgs,
		RunE:  runEpicLint,
	}
}

// runEpicLint reads .tp/epics.json, writes nothing, and exits 0 when no rule
// fires at error severity, 1 when one does, 3 when there is no readable file.
func runEpicLint(_ *cobra.Command, _ []string) error {
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		output.Error(ExitFile, "epic graph not found: no .tp/ directory up to the git boundary", epicGraphFileHint)
		os.Exit(ExitFile)
		return nil
	}
	g, unknown, err := engine.LoadEpicGraph(tpDir)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
		return nil
	}
	result := epicLintResult{Findings: engine.LintEpicGraph(g, unknown)}
	if result.Findings == nil {
		result.Findings = []engine.EpicLintFinding{}
	}
	for _, f := range result.Findings {
		if f.Severity == engine.EpicSeverityError {
			result.Errors++
		} else {
			result.Warnings++
		}
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	if result.Errors > 0 {
		os.Exit(ExitValidation)
	}
	return nil
}
