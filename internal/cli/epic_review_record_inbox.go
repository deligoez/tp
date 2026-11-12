package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

// runEpicReviewRecordInbox records the slice's in-flight round from the
// files its emission named, one per role of the stored panel
// (spec/backlog/epic-role-attribution.md). Each file is read in panel order
// through loadMergeRows, the reader `tp review --merge` applies to the role
// parts a `tp run` record unit merges, and each row is stamped with its
// file's role. It refuses, recording nothing: a round with no emission or one
// whose snapshot predates the panel (exit 3), a role whose file is missing
// (exit 3: the contract asks a role that found nothing for an empty file),
// and a line the reader skips or a row naming another role (exit 1). The
// rest is --record's: the location check and the round write. Under
// TP_UNATTENDED it exits 2 before reading anything, as --record does.
func runEpicReviewRecordInbox(key engine.EpicSliceKey) {
	if engine.Unattended() {
		refuseUnattended("tp epic review --record-inbox writes a graph review round, which", engine.EscalateOther)
	}
	run := loadEpicReviewRun(key)
	refuseEpicReviewAtCap(&run)
	panel := epicInboxPanel(&run)
	var data bytes.Buffer
	var problems []string
	findings := 0
	for _, r := range panel {
		rows, counts := loadMergeRows([]string{r.OutputPath}, reviewMergeRule)
		problems = append(problems, epicInboxSkips(r.ID, counts)...)
		for i, row := range rows {
			if named, _ := row["role"].(string); named != "" && named != r.ID {
				problems = append(problems, fmt.Sprintf("%s row %d names role %q, but the file is role %s's", r.OutputPath, i, named, r.ID))
				continue
			}
			row["role"] = r.ID
			line, err := json.Marshal(row)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s row %d: %v", r.OutputPath, i, err))
				continue
			}
			data.Write(append(line, '\n'))
			findings++
		}
	}
	if len(problems) == 0 {
		_, problems = epicRecordProblems(data.Bytes(), run.all)
	}
	recordEpicRoundData(&run, data.Bytes(), findings, problems)
}

// epicInboxPanel returns the stored panel of run's in-flight round, exiting 3
// when the round was not emitted, its snapshot predates the panel or cannot
// be read, or a role's file is missing, naming every such role.
func epicInboxPanel(run *epicReviewRun) []engine.EpicSnapshotRole {
	label := epicSliceLabel(run.key)
	panel, stored, err := engine.EpicStoredPanel(run.stateDir, run.round)
	switch {
	case err != nil:
		exitEpicRunError(&epicRunError{code: ExitFile, msg: fmt.Sprintf("cannot read round %d's emission snapshot: %v", run.round, err), hint: epicRoundFileHint})
	case !stored:
		exitEpicRunError(&epicRunError{code: ExitFile, msg: fmt.Sprintf("epic review round %d of %s was not emitted: no inbox to record", run.round, label),
			hint: "emit the round first with tp epic review " + label})
	case panel == nil:
		exitEpicRunError(&epicRunError{code: ExitFile, msg: fmt.Sprintf("epic review round %d of %s was emitted before its snapshot kept the panel", run.round, label),
			hint: fmt.Sprintf("emit the round again with tp epic review %s, which stores its panel, or record a file with --record <file>", label)})
	}
	var missing, paths []string
	for _, r := range panel {
		if _, statErr := os.Stat(r.OutputPath); statErr != nil {
			missing = append(missing, r.ID)
			paths = append(paths, r.OutputPath)
		}
	}
	if len(missing) > 0 {
		exitEpicRunError(&epicRunError{code: ExitFile,
			msg: fmt.Sprintf("epic review round %d of %s cannot be recorded from the inbox: role %s wrote no findings file (%s)",
				run.round, label, strings.Join(missing, ", "), strings.Join(paths, ", ")),
			hint: "run each such role's prompt; a role that found nothing writes an empty file"})
	}
	return panel
}

// epicInboxSkips names each line the reader skipped in role's file.
func epicInboxSkips(role string, counts []mergeInputCounts) []string {
	var out []string
	for _, c := range counts {
		for _, s := range c.skips {
			out = append(out, fmt.Sprintf("%s (role %s) %s: %s", c.Path, role, lineList(s.lines), s.reason))
		}
	}
	return out
}
