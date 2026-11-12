package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
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
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Print one task file's read model as JSON: sections, tasks, loops and the tp resume view (read-only)",
		Args:  cobra.NoArgs,
		RunE:  runExport,
	}
	cmd.Flags().Bool("list", false, "Print one row per task file of the repository, bound or not, in phase order")
	return cmd
}

// runExport prints the read model of the task file --file names or discovery
// finds, writing nothing. No task file, or one it cannot read, exits 3.
func runExport(cmd *cobra.Command, _ []string) error {
	if list, _ := cmd.Flags().GetBool("list"); list {
		printResultOrExit(engine.TaskFileListOf("."))
		return nil
	}
	payload, err := exportOf(flagFile, IsCompact())
	if err != nil {
		exitEpicRunError(err)
	}
	printResultOrExit(payload)
	return nil
}

// exportOf derives runExport's payload for the task file explicit names, or
// discovery finds when it is empty, returning the error it would exit with.
func exportOf(explicit string, compact bool) (exportPayload, *epicRunError) {
	path, err := engine.DiscoverTaskFile(".", explicit)
	if err != nil {
		return exportPayload{}, exportDiscoverError(err)
	}
	exp, err := engine.TaskExportOf(".", path)
	return exportResultOf(&exp, err, compact)
}

// exportDiscoverError is the error tp export exits with when discovery
// fails with err.
func exportDiscoverError(err error) *epicRunError {
	hint := exportNoTaskFileHint
	if errors.Is(err, engine.ErrMultipleTaskFiles) {
		hint = "pass --file with one of the task files the message names"
	}
	return &epicRunError{code: ExitFile, msg: err.Error(), hint: hint}
}

// exportResultOf renders exp, or converts err, the export's failure, into
// the error tp export exits with.
func exportResultOf(exp *engine.TaskExport, err error, compact bool) (exportPayload, *epicRunError) {
	if err != nil {
		hint := exportReadHint
		var sce *engine.StateCorruptError
		if errors.As(err, &sce) {
			hint = sce.Hint()
		}
		return exportPayload{}, &epicRunError{code: ExitFile, msg: err.Error(), hint: hint}
	}
	return exportPayloadOf(exp, compact), nil
}

// exportPayloadOf renders exp, dropping under compact each section's body,
// each task's description and each round's snapshot difference and findings.
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
			loop.Rounds[i].FindingRows = nil
		}
	}
	return out
}
