package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

func newEpicExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Print the epic graph as one Markdown table per milestone (--md); writes nothing",
		Args:  cobra.NoArgs,
		RunE:  runEpicExport,
	}
	cmd.Flags().Bool("md", false, "Print one Markdown table per milestone, in track order")
	return cmd
}

// runEpicExport reads .tp/epics.json and prints its Markdown tables; it is
// read-only (spec/1.3.0.md §6). --md is the only format: there is no JSON
// export (§8 Non-goal 3).
func runEpicExport(cmd *cobra.Command, _ []string) error {
	if md, _ := cmd.Flags().GetBool("md"); !md {
		output.Error(ExitUsage, "tp epic export needs --md: Markdown is its only format", "run tp epic export --md")
		os.Exit(ExitUsage)
		return nil
	}
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		output.Error(ExitFile, "epic graph not found: no .tp/ directory up to the git boundary", epicGraphFileHint)
		os.Exit(ExitFile)
		return nil
	}
	g, _, err := engine.LoadEpicGraph(tpDir)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
		return nil
	}
	fmt.Print(engine.EpicGraphMarkdown(g))
	return nil
}
