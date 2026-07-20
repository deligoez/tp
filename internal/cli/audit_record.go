package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// runAuditRecord implements `tp audit <spec> --record <results.ndjson>`.
// Row counting only: a row is a finding when its status field is absent or
// not exactly "PASS". The audit round sequence is independent of review
// rounds; output carries no mechanize_candidates.
func runAuditRecord(specPath, recordPath string) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	if _, err := engine.LoadReviewState(specPath); err != nil {
		exitStateError(err)
		return nil
	}

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read results file: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	findings, parseErr := countAuditFindings(data)
	if parseErr != nil {
		output.Error(ExitValidation, parseErr.Error())
		os.Exit(ExitValidation)
		return nil
	}
	clean := findings == 0

	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	if _, err := engine.EnsureReviewState(specPath); err != nil {
		exitStateError(err)
		return nil
	}

	var st *engine.ReviewState
	var round int
	lockErr := engine.WithReviewStateLock(specPath, func() error {
		var loadErr error
		st, loadErr = engine.LoadReviewState(specPath)
		if loadErr != nil {
			return loadErr
		}
		round = len(st.AuditRounds) + 1
		fileName := fmt.Sprintf("audit-round-%d.ndjson", round)
		if writeErr := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), fileName), data, 0o600); writeErr != nil {
			return writeErr
		}
		st.AuditRounds = append(st.AuditRounds, engine.ReviewRound{
			Round:      round,
			Findings:   findings,
			Clean:      clean,
			RecordedAt: time.Now().UTC().Format(time.RFC3339),
			File:       fileName,
			SpecHash:   specHash,
		})
		return engine.SaveReviewState(specPath, st)
	})
	if lockErr != nil {
		exitStateError(lockErr)
		return nil
	}

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	return output.JSON(map[string]any{
		"round":                 round,
		"findings":              findings,
		"clean":                 clean,
		"consecutive_clean":     engine.ConsecutiveClean(st.AuditRounds),
		"required_clean_rounds": wf.AuditCleanRounds,
		"converged":             engine.Converged(st.AuditRounds, wf.AuditCleanRounds, specHash),
		"stale":                 engine.StateStale(st.AuditRounds, specHash),
	})
}

// countAuditFindings parses rows with the shared line rules and counts rows
// whose status is absent or not exactly "PASS".
func countAuditFindings(data []byte) (findings int, err error) {
	lineNum := 0
	for _, line := range strings.Split(string(data), "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var row map[string]any
		if jsonErr := json.Unmarshal([]byte(trimmed), &row); jsonErr != nil {
			return 0, fmt.Errorf("line %d: invalid JSON: %v", lineNum, jsonErr)
		}
		status, _ := row["status"].(string)
		if status != "PASS" {
			findings++
		}
	}
	return findings, nil
}

// runAuditStatus implements `tp audit <spec> --status [--check]`. The shape
// has no mechanical_checks field — workflow checks guard review rounds.
func runAuditStatus(specPath string, check bool) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	st, err := engine.LoadReviewState(specPath)
	if err != nil {
		exitStateError(err)
		return nil
	}

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	rounds := []engine.ReviewRound{}
	if st != nil {
		rounds = st.AuditRounds
	}
	converged := engine.Converged(rounds, wf.AuditCleanRounds, specHash)

	result := map[string]any{
		"audit_rounds":          rounds,
		"consecutive_clean":     engine.ConsecutiveClean(rounds),
		"required_clean_rounds": wf.AuditCleanRounds,
		"converged":             converged,
		"stale":                 engine.StateStale(rounds, specHash),
	}
	if wf.AuditMaxRounds > 0 {
		result["budget_exhausted"] = len(rounds) >= wf.AuditMaxRounds && !converged
	}

	if jsonErr := output.JSON(result); jsonErr != nil {
		output.Error(ExitFile, jsonErr.Error())
	}

	if check && !converged {
		os.Exit(ExitValidation)
	}
	return nil
}
