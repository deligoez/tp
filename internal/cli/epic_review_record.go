package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicRecordRowHint answers a findings file tp epic review --record refused.
const epicRecordRowHint = "fix every line the message names: each row carries severity, finding, location and evidence, and location is epic:<id> for a unit of the slice or gate:<m> for its gate unit"

// epicRecordFileHint answers an unreadable --record path. The shared
// recordFileMissingHint names the reviewers'/auditors' results file and rules
// out a spec or task file, none of which tp epic review takes.
const epicRecordFileHint = "check the --record path — it takes the epic review findings NDJSON, one row per finding with severity, finding, location (epic:<id> or gate:<m>) and evidence"

// epicRecordResult is tp epic review --record's payload.
type epicRecordResult struct {
	StateDir   string `json:"state_dir"`
	Round      int    `json:"round"`
	Findings   int    `json:"findings"`
	NextAction string `json:"next_action"`
}

// runEpicReviewRecord records a findings file as the slice's next round
// (spec/1.3.0.md §5). Under TP_UNATTENDED it exits 2 before reading anything;
// a file holding any row that lacks a required key or names a location
// outside the slice is refused whole at exit 1, every offending line named,
// and no round is written.
func runEpicReviewRecord(key engine.EpicSliceKey, recordPath string) {
	if engine.Unattended() {
		refuseUnattended("tp epic review --record writes a graph review round, which", engine.EscalateOther)
	}
	run := loadEpicReviewRun(key)
	refuseEpicReviewAtCap(&run)
	data, err := os.ReadFile(recordPath) //nolint:gosec // the caller names the findings file
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read findings file: %s: %v", recordPath, err), epicRecordFileHint)
		os.Exit(ExitFile)
	}
	findings, problems := epicRecordProblems(data, run.all)
	if len(problems) > 0 {
		output.Error(ExitValidation, "findings file refused, no round recorded: "+strings.Join(problems, "; "), epicRecordRowHint)
		os.Exit(ExitValidation)
	}
	round, err := engine.RecordEpicReviewRound(run.stateDir, data)
	var lockErr *engine.LockTimeoutError
	if errors.As(err, &lockErr) {
		output.Error(ExitState, lockErr.Error(), lockErr.Hint())
		os.Exit(ExitState)
	}
	if err != nil {
		output.Error(ExitFile, "cannot write the epic review round: "+err.Error(), epicReviewStateHint)
		os.Exit(ExitFile)
	}
	result := epicRecordResult{
		StateDir: epicReviewStateDirOut(&run),
		Round:    round,
		Findings: findings,
		NextAction: fmt.Sprintf("round %d recorded: emit the next round with tp epic review %s",
			round, epicSliceLabel(key)),
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// epicRecordProblems counts the file's rows and names every line that is not
// a JSON object carrying the four required keys with a location among units.
func epicRecordProblems(data []byte, units []engine.EpicReviewUnit) (findings int, problems []string) {
	legal := make(map[string]bool, len(units))
	for _, u := range units {
		legal[u.Key] = true
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}
		findings++
		if problem := epicRecordRowProblem(trimmed, legal); problem != "" {
			problems = append(problems, fmt.Sprintf("line %d: %s", lineNum, problem))
		}
	}
	if err := scanner.Err(); err != nil {
		problems = append(problems, fmt.Sprintf("line %d: %v", lineNum+1, err))
	}
	return findings, problems
}

// epicRecordRowProblem returns what is wrong with one row, or "".
func epicRecordRowProblem(line string, legal map[string]bool) string {
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		return "invalid JSON: " + err.Error()
	}
	if missing := missingFindingFields(row); len(missing) > 0 {
		return missingFieldsClause(missing)
	}
	if loc, _ := row["location"].(string); !legal[loc] {
		return fmt.Sprintf("location %q names no unit of the slice", loc)
	}
	return ""
}
