package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// runReviewRecordInbox records the collected review round from its inbox
// (spec/backlog/role-presence.md §2.4): one file per role of the round, the
// whole panel the emission resolves before any --role filter. It refuses,
// recording nothing, a round that was not emitted and a role whose file is
// missing (exit 3: a role that found nothing writes an empty file), and a line
// the reader skips or a row naming another role than its file's (exit 1).
// The rows, each stamped with its file's role, are then recorded by --record.
func runReviewRecordInbox(specPath, harnessNote string) error {
	inbox, err := engine.ReviewInboxPanel(specPath)
	if err != nil {
		exitStateError(err)
		return nil
	}
	if !inbox.Emitted {
		output.Error(ExitFile, fmt.Sprintf("review round %d of %s was not emitted: no inbox to record", inbox.Round, specPath),
			"emit the round first with tp review "+specPath)
		os.Exit(ExitFile)
	}
	var missing []string
	for _, role := range inbox.Roles {
		if _, statErr := os.Stat(engine.RoleInboxPath(specPath, engine.PhaseReview, inbox.Round, role)); statErr != nil {
			missing = append(missing, role)
		}
	}
	if len(missing) > 0 {
		roles := strings.Join(missing, ", ")
		output.Error(ExitFile, fmt.Sprintf("review round %d cannot be recorded from the inbox: no findings file from roles %s", inbox.Round, roles),
			fmt.Sprintf("run the prompts of roles %s (tp review %s --role <role>), each writing its file, empty when it finds nothing, then tp review %s --record-inbox", roles, specPath, specPath))
		os.Exit(ExitFile)
	}
	data, problems := reviewInboxRows(specPath, inbox)
	if len(problems) > 0 {
		output.Error(ExitValidation, "review round "+fmt.Sprint(inbox.Round)+"'s inbox cannot be recorded: "+strings.Join(problems, "; "),
			"fix the named lines in the role files, then tp review "+specPath+" --record-inbox")
		os.Exit(ExitValidation)
	}
	tmp, err := os.CreateTemp("", "tp-review-inbox-*.ndjson")
	if err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	_, writeErr := tmp.Write(data)
	_ = tmp.Close()
	if writeErr != nil {
		_ = os.Remove(tmp.Name())
		output.Error(ExitFile, writeErr.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	err = runReviewRecord(specPath, tmp.Name(), harnessNote)
	_ = os.Remove(tmp.Name())
	return err
}

// reviewInboxRows reads each role's file through the reader --merge applies,
// stamping every row with its file's role, and names each skipped line and each
// row that names another role.
func reviewInboxRows(specPath string, inbox engine.ReviewInboxRound) (data []byte, problems []string) {
	var buf bytes.Buffer
	for _, role := range inbox.Roles {
		path := engine.RoleInboxPath(specPath, engine.PhaseReview, inbox.Round, role)
		rows, counts := loadMergeRows([]string{path}, reviewMergeRule)
		for _, c := range counts {
			for _, s := range c.skips {
				problems = append(problems, fmt.Sprintf("%s (role %s) %s: %s", filepath.Base(path), role, lineList(s.lines), s.reason))
			}
		}
		for i, row := range rows {
			if named, _ := row["role"].(string); named != "" && named != role {
				problems = append(problems, fmt.Sprintf("%s row %d names role %q, but the file is role %s's", filepath.Base(path), i, named, role))
				continue
			}
			row["role"] = role
			line, err := json.Marshal(row)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s row %d: %v", filepath.Base(path), i, err))
				continue
			}
			buf.Write(append(line, '\n'))
		}
	}
	return buf.Bytes(), problems
}
