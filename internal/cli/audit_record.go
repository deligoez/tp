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
func runAuditRecord(specPath, recordPath, harnessNote string) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	stPre, err := engine.LoadReviewState(specPath)
	if err != nil {
		exitStateError(err)
		return nil
	}

	// Round-budget refusal comes before line parsing and any state write
	wfPre, _ := engine.ResolveWorkflow(specPath, flagFile)
	preRounds := []engine.ReviewRound{}
	if stPre != nil {
		preRounds = stPre.AuditRounds
	}
	refuseIfBudgetExhausted("audit", specPath, preRounds, wfPre.AuditMaxRounds, wfPre.AuditCleanRounds, "")

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read results file: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	findings, parseErr := countAuditFindings(recordPath, data)
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

	st, round, lockErr := recordAuditRoundEntry(specPath, data, findings, clean, specHash, harnessNote)
	if lockErr != nil {
		exitStateError(lockErr)
		return nil
	}

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	result := map[string]any{
		"round":                 round,
		"findings":              findings,
		"clean":                 clean,
		"consecutive_clean":     engine.ConsecutiveClean(st.AuditRounds),
		"required_clean_rounds": wf.AuditCleanRounds,
		"converged":             engine.Converged(st.AuditRounds, wf.AuditCleanRounds, specHash),
		"stale":                 engine.StateStale(st.AuditRounds, specHash),
		"harness_stale":         engine.HarnessStale(st.AuditRounds),
	}
	// harness_note is emitted only when harness_stale is true; --record reports
	// staleness AFTER storing this round, so it is the latest of the two compared.
	if engine.HarnessStale(st.AuditRounds) {
		result["harness_note"] = engine.LatestHarnessNote(st.AuditRounds)
	}
	// §8.1/§8.2: next_action names the single next step by the audit precedence.
	// Advisory/read-only — it changes nothing and never gates the exit code. The
	// just-recorded round is the latest, so its non-PASS rows are exactly !clean.
	converged := engine.Converged(st.AuditRounds, wf.AuditCleanRounds, specHash)
	result["next_action"] = engine.AuditNextAction(specPath, converged, !clean)
	return output.JSON(result)
}

// recordAuditRoundEntry copies the results file into the state directory as
// audit-round-<N>.ndjson and appends the round entry to state.json under the
// state flock (round file first, index entry second).
func recordAuditRoundEntry(specPath string, data []byte, findings int, clean bool, specHash, harnessNote string) (st *engine.ReviewState, round int, err error) {
	// Auditor corpus hash at record time (§9.2), stored on the round entry.
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseAuditors)
	err = engine.WithReviewStateLock(specPath, func() error {
		loaded, loadErr := engine.LoadReviewState(specPath)
		if loadErr != nil {
			return loadErr
		}
		st = loaded
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
			RolesHash:   rolesHash,
			IDScheme:    engine.IDSchemeSlug,
			HarnessNote: harnessNote,
		})
		return engine.SaveReviewState(specPath, st)
	})
	return st, round, err
}

// countAuditFindings parses rows with the shared line rules and counts rows
// whose status is absent or not exactly "PASS".
func countAuditFindings(path string, data []byte) (findings int, err error) {
	lineNum := 0
	for _, line := range strings.Split(string(data), "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var row map[string]any
		if jsonErr := json.Unmarshal([]byte(trimmed), &row); jsonErr != nil {
			return 0, fmt.Errorf("line %d: invalid JSON: %w", lineNum, jsonErr)
		}
		if role, _ := row["role"].(string); role == "" {
			fmt.Fprintf(os.Stderr, "warning: line %d of %s is missing the role field; it will not appear in the per-role overlap report\n", lineNum, path)
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
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseAuditors)

	result := map[string]any{
		"audit_rounds":          rounds,
		"consecutive_clean":     engine.ConsecutiveClean(rounds),
		"required_clean_rounds": wf.AuditCleanRounds,
		"converged":             converged,
		"stale":                 engine.StateStale(rounds, specHash),
		"roles_stale":           engine.RolesStale(rounds, rolesHash),
		"harness_stale":         engine.HarnessStale(rounds),
	}
	// harness_note is emitted only when harness_stale is true; when emitted it is
	// the verbatim stored note of the latest recorded audit round (§6.3).
	if engine.HarnessStale(rounds) {
		result["harness_note"] = engine.LatestHarnessNote(rounds)
	}
	// §9.3 / §8.4: the audit overlap_report over the latest round's non-PASS
	// rows is explanatory and is omitted under --compact.
	if !IsCompact() {
		result["overlap_report"] = latestAuditRoundOverlapReport(specPath, rounds)
	}
	// §10.1: surface the effective cap and remaining budget next to
	// budget_exhausted; null when uncapped. Decision-critical, so these
	// survive --compact (§8.4).
	if wf.AuditMaxRounds > 0 {
		result["max_rounds"] = wf.AuditMaxRounds
		remaining := wf.AuditMaxRounds - len(rounds)
		if remaining < 0 {
			remaining = 0
		}
		result["rounds_remaining"] = remaining
		result["budget_exhausted"] = len(rounds) >= wf.AuditMaxRounds && !converged
	} else {
		result["max_rounds"] = nil
		result["rounds_remaining"] = nil
	}
	// §10.2: surface an interrupted audit round — a snapshot exists for the
	// next round but its audit-round-N.ndjson was never recorded.
	if inFlight := engine.InFlightRound(specPath, engine.PhaseAudit, len(rounds)); inFlight > 0 {
		result["in_flight_round"] = inFlight
	} else {
		result["in_flight_round"] = nil
	}

	// §8.1/§8.2: next_action names the single next step by the audit precedence.
	// Advisory/read-only — it changes nothing and never gates the exit code. The
	// latest recorded round holding open non-PASS rows is exactly !Clean.
	latestHasFindings := len(rounds) > 0 && !rounds[len(rounds)-1].Clean
	result["next_action"] = engine.AuditNextAction(specPath, converged, latestHasFindings)

	if jsonErr := output.JSON(result); jsonErr != nil {
		output.Error(ExitFile, jsonErr.Error())
	}

	if check && !converged {
		os.Exit(ExitValidation)
	}
	return nil
}
