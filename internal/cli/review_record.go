package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// runReviewRecord implements `tp review <spec> --record <findings.ndjson>`:
// parse the rows, copy the file into the state directory as
// review-round-<R>.ndjson (round file first, index entry second, under the
// state flock), and report convergence plus mechanization candidates.
func runReviewRecord(specPath, recordPath string) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	// Corrupt state aborts before any parsing or write
	stPre, err := engine.LoadReviewState(specPath)
	if err != nil {
		exitStateError(err)
		return nil
	}

	// Round-budget refusal comes before line parsing and any state write
	wfPre, _ := engine.ResolveWorkflow(specPath, flagFile)
	preRounds := []engine.ReviewRound{}
	if stPre != nil {
		preRounds = stPre.ReviewRounds
	}
	refuseIfBudgetExhausted("review", specPath, preRounds, wfPre.ReviewMaxRounds, wfPre.ReviewCleanRounds)

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read findings file: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	findings, dirty, parseErr := parseRecordRows(data)
	if parseErr != nil {
		output.Error(ExitValidation, parseErr.Error())
		os.Exit(ExitValidation)
		return nil
	}
	clean := dirty == 0

	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	// --record creates the state directory when absent; it never writes snapshots
	if _, err := engine.EnsureReviewState(specPath); err != nil {
		exitStateError(err)
		return nil
	}

	// Reviewer corpus hash at record time (§9.2), stored on the round entry.
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseReviewers)

	var st *engine.ReviewState
	var round int
	lockErr := engine.WithReviewStateLock(specPath, func() error {
		var loadErr error
		st, loadErr = engine.LoadReviewState(specPath)
		if loadErr != nil {
			return loadErr
		}
		round = len(st.ReviewRounds) + 1
		fileName := fmt.Sprintf("review-round-%d.ndjson", round)
		// Round file first, index entry second
		if writeErr := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), fileName), data, 0o600); writeErr != nil {
			return writeErr
		}
		st.ReviewRounds = append(st.ReviewRounds, engine.ReviewRound{
			Round:      round,
			Findings:   findings,
			Clean:      clean,
			RecordedAt: time.Now().UTC().Format(time.RFC3339),
			File:       fileName,
			SpecHash:   specHash,
			RolesHash:  rolesHash,
		})
		return engine.SaveReviewState(specPath, st)
	})
	if lockErr != nil {
		exitStateError(lockErr)
		return nil
	}

	// Mechanize candidates across all recorded rounds including this one
	roundFindings := make([][]map[string]any, 0, len(st.ReviewRounds))
	for _, r := range st.ReviewRounds {
		rows, found := engine.LoadRoundRows(specPath, &r)
		if !found {
			output.Info(fmt.Sprintf("round %d file %s is missing; skipping its rows", r.Round, r.File))
			continue
		}
		roundFindings = append(roundFindings, rows)
	}
	candidates := computeMechanizeCandidates(roundFindings)

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	result := map[string]any{
		"round":                 round,
		"findings":              findings,
		"clean":                 clean,
		"consecutive_clean":     engine.ConsecutiveClean(st.ReviewRounds),
		"required_clean_rounds": wf.ReviewCleanRounds,
		"converged":             engine.Converged(st.ReviewRounds, wf.ReviewCleanRounds, specHash),
		"stale":                 engine.StateStale(st.ReviewRounds, specHash),
		"mechanize_candidates":  candidates,
	}
	if len(candidates) > 0 {
		result["hint"] = mechanizeRegisterHint
	}
	return output.JSON(result)
}

// parseRecordRows applies the row rules: blank lines skipped, every remaining
// line a JSON object, pre-resolved wontfix needs evidence and does not dirty
// the round, pre-resolved fixed aborts, pre-resolved duplicate dirties.
func parseRecordRows(data []byte) (findings, dirty int, err error) {
	lineNum := 0
	for _, line := range strings.Split(string(data), "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var row map[string]any
		if jsonErr := json.Unmarshal([]byte(trimmed), &row); jsonErr != nil {
			return 0, 0, fmt.Errorf("line %d: invalid JSON: %w", lineNum, jsonErr)
		}
		findings++

		status, evidence := resolvedStatusOf(row)
		switch status {
		case "fixed":
			return 0, 0, fmt.Errorf("line %d: row arrives pre-resolved fixed — a fix means the spec changed; record the round without it and re-review", lineNum)
		case "wontfix":
			if strings.TrimSpace(evidence) == "" {
				return 0, 0, fmt.Errorf("line %d: pre-resolved wontfix row requires non-empty resolved.evidence", lineNum)
			}
			// verified-rejected rows do not dirty the round
		default:
			// unresolved and pre-resolved duplicate rows dirty the round
			dirty++
		}
	}
	return findings, dirty, nil
}

// resolvedStatusOf extracts resolved.status and resolved.evidence from a row.
func resolvedStatusOf(row map[string]any) (status, evidence string) {
	resolved, ok := row["resolved"].(map[string]any)
	if !ok {
		return "", ""
	}
	status, _ = resolved["status"].(string)
	evidence, _ = resolved["evidence"].(string)
	return status, evidence
}

// exitStateError reports state-layer failures: corrupt state exits 3 with the
// repair hint; anything else exits 3 with the raw error.
func exitStateError(err error) {
	var ce *engine.StateCorruptError
	if errors.As(err, &ce) {
		output.Error(ExitFile, ce.Error(), ce.Hint())
		os.Exit(ExitFile)
		return
	}
	output.Error(ExitFile, err.Error())
	os.Exit(ExitFile)
}
