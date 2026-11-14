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

// owedReviewRoles is the roles a review round owes a file from: every prompt
// the emission built, taken before the --role filter so every sibling emission
// writes the same list (spec/backlog/role-presence.md).
func owedReviewRoles(prompts []reviewPrompt) []string {
	owed := make([]string, 0, len(prompts))
	for i := range prompts {
		owed = append(owed, prompts[i].Role)
	}
	return owed
}

// keepOwedReviewRoles stores the owed roles in the round's inbox. A failed
// write costs the round --record-inbox and nothing else, so it is a notice.
func keepOwedReviewRoles(specPath string, round int, owed []string) {
	if err := engine.WriteReviewOwedRoles(specPath, round, owed); err != nil {
		output.Notice(fmt.Sprintf("warning: cannot keep review round %d's roles, so --record-inbox will refuse it: %v", round, err))
	}
}

// runReviewRecordInbox records the collected review round from its inbox
// (spec/backlog/role-presence.md §2.4): one file per role the round's emission
// asked before any --role filter, as the emission stored them. It refuses,
// recording nothing, a round that was not emitted or was emitted before the
// list was stored, and a role whose file is missing (exit 3: a role that found
// nothing writes an empty file), and a line the reader skips or a row naming
// another role than its file's (exit 1). The rows, each stamped with its file's
// role, are written beside the role files and recorded as --record records a
// file, so a refusal there names lines of a file the agent can open.
func runReviewRecordInbox(specPath, harnessNote string) error {
	if os.Getenv(engine.EnvRound) != "" {
		exitUsage("--record-inbox records the next round by hand and does not rewrite a recorded one", "unset "+engine.EnvRound+", or record a file with --record <file>")
	}
	inbox, err := engine.ReviewInboxPanel(specPath)
	if err != nil {
		exitStateError(err)
		return nil
	}
	emit := "tp review " + specPath
	switch {
	case !inbox.Emitted:
		output.Error(ExitFile, fmt.Sprintf("review round %d of %s was not emitted: no inbox to record", inbox.Round, specPath),
			"emit the round first with "+emit)
		os.Exit(ExitFile)
	case !inbox.Stored:
		output.Error(ExitFile, fmt.Sprintf("review round %d of %s was emitted without the list of roles it owes (before v1.20.0, or under --no-state)", inbox.Round, specPath),
			fmt.Sprintf("emit the round again with %s, which stores the list, or record a merged file with --record <file>", emit))
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
			fmt.Sprintf("run the prompts of roles %s (%s --role <role>), each writing its file, empty when it finds nothing, then %s --record-inbox", roles, emit, emit))
		os.Exit(ExitFile)
	}
	data, problems := reviewInboxRows(specPath, inbox)
	if len(problems) > 0 {
		output.Error(ExitValidation, "review round "+fmt.Sprint(inbox.Round)+"'s inbox cannot be recorded: "+strings.Join(problems, "; "),
			"fix the named lines in the role files, then "+emit+" --record-inbox")
		os.Exit(ExitValidation)
	}
	merged := engine.ReviewInboxMergedPath(specPath, inbox.Round)
	if err := os.WriteFile(merged, data, 0o600); err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	return runReviewRecord(specPath, merged, harnessNote)
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
