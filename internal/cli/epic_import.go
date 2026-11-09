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

// epicImportResult is tp epic import's payload: the file it wrote, the number
// of epics it holds, and the lint findings the imported graph carries (D4).
type epicImportResult struct {
	File     string                   `json:"file"`
	Epics    int                      `json:"epics"`
	Findings []engine.EpicLintFinding `json:"findings"`
	Revision string                   `json:"revision"`
	// AncestryChanged lists the milestones of the replaced graph whose
	// ancestry the import changed, as tp epic set milestones= does.
	AncestryChanged []engine.AncestryChange `json:"ancestry_changed"`
}

func newEpicImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Replace .tp/epics.json with a graph file; one that holds epics needs --force",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicImport,
	}
	cmd.Flags().Bool("force", false, "replace a graph file that already holds epics")
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicImport implements tp epic import (spec/1.3.0.md §3, D4): the
// payload is written whatever lint errors it holds, and they are reported.
func runEpicImport(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic import")
	force, _ := cmd.Flags().GetBool("force")
	g := readEpicImportPayload(args[0])
	out, findings, err := engine.ImportEpicGraph(engine.ProjectConfigDir("."), g, force, epicIfRevision(cmd))
	var refused *engine.EpicImportRefusedError
	var stale *engine.EpicStaleRevisionError
	var mce *engine.MalformedConfigError
	var lockErr *engine.LockTimeoutError
	switch {
	case errors.As(err, &lockErr):
		output.Error(ExitState, lockErr.Error(), lockErr.Hint())
		os.Exit(ExitState)
	case errors.As(err, &stale):
		output.Error(ExitState, stale.Error(), epicStaleRevisionHint)
		os.Exit(ExitState)
	case errors.As(err, &refused):
		output.Error(ExitValidation, refused.Error(), epicImportRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &mce):
		output.Error(ExitFile, mce.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
	case err != nil:
		output.Error(ExitFile, err.Error(), epicGraphWriteHint)
		os.Exit(ExitFile)
	}
	result := epicImportResult{File: out.Path, Epics: len(g.Epics), Findings: findings, Revision: out.Revision, AncestryChanged: out.Ancestry}
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
	if err == nil {
		err = engine.CheckOpensAfterLists(g.OpensAfter)
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
