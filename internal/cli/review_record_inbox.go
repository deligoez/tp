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
// write fails the emission (exit 3): without the list the round would read as
// emitted before v1.20.0, and --record <file> would record it with no proof
// any role ran.
func keepOwedReviewRoles(specPath string, round int, owed []string) {
	if err := engine.WriteReviewOwedRoles(specPath, round, owed); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot keep review round %d's owed roles: %v", round, err), runtimeFailureHint)
		os.Exit(ExitFile)
	}
}

// refuseHandRecordOverEmission exits 3 when --record <file> would record a
// round whose emission stored the roles it owes (v1.20.0+) and nothing on
// disk shows every one of them ran. By hand that round is recorded from its
// inbox with --record-inbox: a hand file cannot tell a role that found
// nothing from one that never ran, and a row deleted from it leaves without a
// disposition. Under tp run the record unit merges the role files the driver
// collected in $TP_ROUND_DIR, so the same rule is met there by each owed
// role's file being present, not by the environment variable alone. A round
// nobody emitted, one emitted before the list was stored, and a re-record of
// a round already recorded (TP_ROUND naming it) keep --record <file>.
func refuseHandRecordOverEmission(specPath string) {
	inbox, err := engine.ReviewInboxPanel(specPath)
	if err != nil || !inbox.Emitted || !inbox.Stored {
		return
	}
	// Only a TP_ROUND naming a recorded round re-records; any other value
	// records the next round, the one this rule protects.
	if _, rewrite := engine.RecordTargetRound(os.Getenv(engine.EnvRound), inbox.Round-1); rewrite {
		return
	}
	if dir := os.Getenv(engine.EnvRoundDir); dir != "" && everyRoleFilePresent(dir, inbox.Roles) {
		return
	}
	output.Error(ExitFile, fmt.Sprintf("review round %d of %s was emitted with the roles it owes, so it is recorded from its inbox, not from a hand file", inbox.Round, specPath),
		fmt.Sprintf("have each role write its prompt's output_path (empty when it finds nothing), then tp review %s --record-inbox", specPath))
	os.Exit(ExitFile)
}

// recordRoundCommand is the command that records an emitted round in flight:
// a review round from its inbox (v1.21.0), an audit round from its file.
func recordRoundCommand(phase, specPath string) string {
	if phase == "review" {
		return "tp review " + specPath + " --record-inbox"
	}
	return "tp " + phase + " " + specPath + " --record <file>"
}

// everyRoleFilePresent reports whether the driver's round directory holds a
// findings file for every owed role.
func everyRoleFilePresent(roundDir string, roles []string) bool {
	for _, role := range roles {
		if _, err := os.Stat(engine.RoleFindingsPath(roundDir, role)); err != nil {
			return false
		}
	}
	return true
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
		output.Error(ExitFile, fmt.Sprintf("review round %d of %s was emitted without the list of roles it owes (before v1.20.0)", inbox.Round, specPath),
			fmt.Sprintf("emit the round again with %s, which stores the list, or record a merged file with --record <file>", emit))
		os.Exit(ExitFile)
	}
	files := reviewRoleFiles(specPath, inbox)
	var missing []string
	for _, role := range inbox.Roles {
		if files[role] == "" {
			missing = append(missing, role)
		}
	}
	if len(missing) > 0 {
		roles := strings.Join(missing, ", ")
		read := engine.RoleInboxDirAt(namedInbox(), specPath, engine.PhaseReview)
		if abs, absErr := filepath.Abs(read); absErr == nil {
			read = abs
		}
		output.Error(ExitFile, fmt.Sprintf("review round %d cannot be recorded from the inbox %s: no findings file from roles %s", inbox.Round, read, roles),
			fmt.Sprintf("run the prompts of roles %s (%s --role <role>), each writing its file, empty when it finds nothing, then %s --record-inbox; a round emitted with --inbox <dir> is recorded with the same --inbox <dir>", roles, emit, emit))
		os.Exit(ExitFile)
	}
	data, problems := reviewInboxRows(inbox, files)
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

// reviewRoleFiles finds each owed role's findings file: in the round's inbox,
// or, for a round a tp run collected and stopped before recording, in the
// run's round directory ($TP_ROUND_DIR/role-<role>.ndjson), which the driver
// keeps per task file and round. A role with neither maps to "".
func reviewRoleFiles(specPath string, inbox engine.ReviewInboxRound) map[string]string {
	runDir := ""
	if _, taskFile := engine.ResolveWorkflow(specPath, flagFile); taskFile != "" {
		root := engine.CanonicalProjectRoot(filepath.Dir(taskFile))
		runDir = engine.RoundDir(root, taskFile, engine.PhaseReview, inbox.Round)
	}
	files := make(map[string]string, len(inbox.Roles))
	for _, role := range inbox.Roles {
		candidates := []string{engine.RoleInboxPathAt(namedInbox(), specPath, engine.PhaseReview, inbox.Round, role)}
		if runDir != "" {
			candidates = append(candidates, engine.RoleFindingsPath(runDir, role))
		}
		for _, path := range candidates {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				files[role] = path
				break
			}
		}
	}
	return files
}

// reviewInboxRows reads each role's file through the reader --merge applies,
// stamping every row with its file's role, and names each skipped line and each
// row that names another role.
func reviewInboxRows(inbox engine.ReviewInboxRound, files map[string]string) (data []byte, problems []string) {
	var buf bytes.Buffer
	for _, role := range inbox.Roles {
		path := files[role]
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
