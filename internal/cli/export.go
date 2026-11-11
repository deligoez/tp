package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// exportPayload is tp export's payload (spec/1.8.0.md §16): the task file's
// read model with its sections and its tp resume view rendered as the flags
// ask, the resume view exactly as tp resume prints it.
type exportPayload struct {
	engine.TaskExport
	Sections any            `json:"sections"`
	Resume   map[string]any `json:"resume"`
}

// exportCompactSection is a section under --compact: its heading and anchor,
// without its body or its paragraph anchors.
type exportCompactSection struct {
	Anchor  string `json:"anchor"`
	Heading string `json:"heading"`
	Line    int    `json:"line"`
}

// exportNoTaskFileHint answers a tp export that finds no task file to read.
const exportNoTaskFileHint = "pass --file <path>.tasks.json, or run tp export where discovery finds one task file (tp init <spec> creates one)"

// exportReadHint answers a task file, or its spec's state, tp export cannot read.
const exportReadHint = "tp export reads the task file and its spec's state under .tp-review/: repair the file the message names (tp validate checks a task file)"

func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Print one task file's read model as JSON: sections, tasks, loops and the tp resume view (read-only)",
		Args:  cobra.NoArgs,
		RunE:  runExport,
	}
}

// runExport prints the read model of the task file --file names or discovery
// finds, writing nothing. No task file, or one it cannot read, exits 3.
func runExport(_ *cobra.Command, _ []string) error {
	path, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		hint := exportNoTaskFileHint
		if errors.Is(err, engine.ErrMultipleTaskFiles) {
			hint = "pass --file with one of the task files the message names"
		}
		output.Error(ExitFile, err.Error(), hint)
		os.Exit(ExitFile)
	}
	exp, err := engine.TaskExportOf(".", path)
	if err != nil {
		hint := exportReadHint
		var sce *engine.StateCorruptError
		if errors.As(err, &sce) {
			hint = sce.Hint()
		}
		output.Error(ExitFile, err.Error(), hint)
		os.Exit(ExitFile)
	}
	printResultOrExit(exportPayloadOf(&exp, IsCompact()))
	return nil
}

// exportPayloadOf renders exp, dropping under compact each section's body,
// each task's description and each round's snapshot difference.
func exportPayloadOf(exp *engine.TaskExport, compact bool) exportPayload {
	out := exportPayload{TaskExport: *exp, Sections: exp.Sections, Resume: resumeJSON(&exp.Resume, compact)}
	if !compact {
		return out
	}
	sections := make([]exportCompactSection, 0, len(exp.Sections))
	for _, s := range exp.Sections {
		sections = append(sections, exportCompactSection{Anchor: s.Anchor, Heading: s.Heading, Line: s.Line})
	}
	out.Sections = sections
	out.Tasks = append([]engine.ExportTask{}, exp.Tasks...)
	for i := range out.Tasks {
		out.Tasks[i].Description = ""
	}
	for _, loop := range []*engine.ExportLoop{&out.Loops.Ground, &out.Loops.Review, &out.Loops.Audit} {
		loop.Rounds = append([]engine.ExportRound{}, loop.Rounds...)
		for i := range loop.Rounds {
			loop.Rounds[i].SnapshotDiff = nil
		}
	}
	return out
}
