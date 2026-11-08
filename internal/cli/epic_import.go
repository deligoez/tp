package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicImportRefusedHint answers an import over a file that holds epics.
const epicImportRefusedHint = "pass --force to replace the file; tp epic export --md shows what it holds"

// epicImportResult is tp epic import's payload: the file it wrote, the epic
// ids it holds, and the lint findings the imported graph carries (D4).
type epicImportResult struct {
	File     string                   `json:"file"`
	Epics    []string                 `json:"epics"`
	Findings []engine.EpicLintFinding `json:"findings"`
}

func newEpicImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Replace .tp/epics.json with a graph file; one that holds epics needs --force",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicImport,
	}
	cmd.Flags().Bool("force", false, "replace a graph file that already holds epics")
	return cmd
}

// runEpicImport implements tp epic import (spec/1.3.0.md §3, D4): the
// payload is written whatever lint errors it holds, and they are reported.
func runEpicImport(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic import")
	force, _ := cmd.Flags().GetBool("force")
	g := readEpicImportPayload(args[0])
	path, findings, err := engine.ImportEpicGraph(engine.ProjectConfigDir("."), g, force)
	var refused *engine.EpicImportRefusedError
	var mce *engine.MalformedConfigError
	switch {
	case errors.As(err, &refused):
		output.Error(ExitValidation, refused.Error(), epicImportRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &mce):
		output.Error(ExitFile, mce.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
	case err != nil:
		output.Error(ExitFile, err.Error(), stateWriteHint)
		os.Exit(ExitFile)
	}
	result := epicImportResult{File: path, Epics: make([]string, 0, len(g.Epics)), Findings: findings}
	for i := range g.Epics {
		result.Epics = append(result.Epics, g.Epics[i].ID)
	}
	if result.Findings == nil {
		result.Findings = []engine.EpicLintFinding{}
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}

// readEpicImportPayload parses the graph file to import, refusing keys
// outside the schema and anything after the one JSON value as tp epic add
// does; a missing file exits 3.
func readEpicImportPayload(path string) *model.EpicGraph {
	data, err := os.ReadFile(path)
	if err != nil {
		output.Error(ExitFile, err.Error(), "pass a readable graph file in .tp/epics.json's shape")
		os.Exit(ExitFile)
	}
	var g model.EpicGraph
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	err = dec.Decode(&g)
	if _, tokErr := dec.Token(); err == nil && !errors.Is(tokErr, io.EOF) {
		err = errors.New("trailing data after the graph: pass exactly one JSON object")
	}
	if err != nil {
		output.Error(ExitUsage, "invalid epic graph JSON", err.Error())
		os.Exit(ExitUsage)
	}
	if g.Milestones == nil {
		g.Milestones = [][]string{}
	}
	if g.Epics == nil {
		g.Epics = []model.Epic{}
	}
	return &g
}
