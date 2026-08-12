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
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
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
		output.Error(ExitFile, fmt.Sprintf("cannot read results file: %s", recordPath), recordFileMissingHint)
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
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %s", specPath), err.Error())
		os.Exit(ExitFile)
		return nil
	}

	if _, err := engine.EnsureReviewState(specPath); err != nil {
		exitStateError(err)
		return nil
	}

	st, round, roundRolesHash, lockErr := recordAuditRoundEntry(specPath, data, findings, clean, specHash, harnessNote)
	if lockErr != nil {
		exitStateError(lockErr)
		return nil
	}

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	// converged and stale are computed once, so the payload and §2.4's
	// conditions 3 and 4 read the same two values and the divergence object can
	// never be emitted beside a payload that contradicts it.
	converged := engine.Converged(st.AuditRounds, wf.AuditCleanRounds, specHash)
	stale := engine.StateStale(st.AuditRounds, specHash)
	result := map[string]any{
		"round":                 round,
		"findings":              findings,
		"clean":                 clean,
		"consecutive_clean":     engine.ConsecutiveClean(st.AuditRounds),
		"required_clean_rounds": wf.AuditCleanRounds,
		"converged":             converged,
		"stale":                 stale,
	}
	// §2.5: the signal is computed AFTER the round is stored, so "the latest
	// recorded round" is the round just recorded — the convention harness_stale
	// already follows on this path. Condition 5 reads back the corpus hash
	// recordAuditRoundEntry just stamped on this round rather than hashing the
	// corpus a second time, so the equality holds by construction here.
	auditSignalFields(result, specPath, st.AuditRounds, wf.AuditCleanRounds, stale, converged, roundRolesHash)
	// §8.4: harness_stale and harness_note are explanatory and are omitted under
	// --compact; next_action is decision-critical and survives it. When emitted,
	// harness_note is the verbatim stored note; --record reports staleness AFTER
	// storing this round, so it is the latest of the two compared.
	if !IsCompact() {
		result["harness_stale"] = engine.HarnessStale(st.AuditRounds)
		if engine.HarnessStale(st.AuditRounds) {
			result["harness_note"] = engine.LatestHarnessNote(st.AuditRounds)
		}
	}
	// §8.1/§8.2: next_action names the single next step by the audit precedence.
	// Advisory/read-only — it changes nothing and never gates the exit code. The
	// just-recorded round is the latest, so its non-PASS rows are exactly !clean.
	result["next_action"] = engine.AuditNextAction(specPath, converged, !clean)
	return output.JSON(result)
}

// auditSignalFields writes §2.5's three fields onto an audit payload:
// role_streaks, spec_coverage_clean_rounds and divergence. It is called from the
// two outputs that carry them — `tp audit <spec> --status`, with or without
// --check and before its exit-code branch, and `tp audit <spec> --record <file>`
// after the round is stored — and from nowhere else, so prompt emission,
// `tp audit --merge` and every `tp review` mode stay free of them.
//
// All three survive --compact whole, so both call sites make this call
// unconditionally: role_streaks is always an emitted array, never null;
// spec_coverage_clean_rounds is always an emitted key whose value is null when
// the latest recorded round measured no conformance; and divergence is an
// omitted key — never a JSON null — whenever any of §2.4's five conditions
// fails.
//
// currentRolesHash is condition 5's right-hand side and is the one input that
// differs by path: on --status it is the auditor-corpus hash computed now, the
// same one roles_stale reports against, while on --record it is the hash the
// recording helper just stamped on this round, read back rather than recomputed.
func auditSignalFields(result map[string]any, specPath string, rounds []engine.ReviewRound,
	requiredCleanRounds int, stale, converged bool, currentRolesHash string,
) {
	streaks, latestRows := engine.ComputeAuditRoleStreaks(specPath, rounds)
	specCoverage := engine.SpecCoverageCleanRounds(streaks)
	result["role_streaks"] = streaks
	result["spec_coverage_clean_rounds"] = specCoverage
	if divergence := engine.ComputeAuditDivergence(&engine.DivergenceInputs{
		Rounds:                  rounds,
		LatestRows:              latestRows,
		SpecCoverageCleanRounds: specCoverage,
		RequiredCleanRounds:     requiredCleanRounds,
		Stale:                   stale,
		Converged:               converged,
		CurrentRolesHash:        currentRolesHash,
	}); divergence != nil {
		result["divergence"] = divergence
	}
}

// recordAuditRoundEntry copies the results file into the state directory as
// audit-round-<N>.ndjson and appends the round entry to state.json under the
// state flock (round file first, index entry second).
//
// It returns the auditor-corpus hash it stamped on the new round entry, so
// §2.4's condition 5 can compare the stored hash against the value this one
// computation produced instead of hashing the corpus a second time on the
// --record path.
func recordAuditRoundEntry(specPath string, data []byte, findings int, clean bool, specHash, harnessNote string) (st *engine.ReviewState, round int, rolesHash string, err error) {
	// Auditor corpus hash at record time (§9.2), stored on the round entry.
	rolesHash, _ = engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseAuditors)
	err = engine.WithReviewStateLock(specPath, func() error {
		loaded, loadErr := engine.LoadReviewState(specPath)
		if loadErr != nil {
			return loadErr
		}
		// LoadReviewState returns (nil, nil) when state.json is gone and no
		// round or snapshot artifacts remain, which an external delete between
		// EnsureReviewState and this lock can produce. Dereferencing it would
		// panic instead of aborting through the state-error path.
		if loaded == nil {
			// A StateCorruptError rather than a bare error: it carries the
			// repair hint exitStateError attaches, and it keeps this message
			// identical to the one the sibling record path produces.
			return &engine.StateCorruptError{
				Path:   engine.ReviewStateDir(specPath),
				Reason: vanishedStateReason,
			}
		}
		st = loaded
		round = len(st.AuditRounds) + 1
		fileName := fmt.Sprintf("audit-round-%d.ndjson", round)
		if writeErr := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), fileName), data, 0o600); writeErr != nil {
			return writeErr
		}
		st.AuditRounds = append(st.AuditRounds, engine.ReviewRound{
			Round:       round,
			Findings:    findings,
			Clean:       clean,
			RecordedAt:  time.Now().UTC().Format(time.RFC3339),
			File:        fileName,
			SpecHash:    specHash,
			RolesHash:   rolesHash,
			IDScheme:    engine.IDSchemeSlug,
			HarnessNote: harnessNote,
		})
		return engine.SaveReviewState(specPath, st)
	})
	return st, round, rolesHash, err
}

// countAuditFindings parses rows with the shared line rules and counts rows
// whose status is absent or not exactly "PASS".
func countAuditFindings(path string, data []byte) (findings int, err error) {
	lineNum := 0
	var rl rolelessRows
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
		rl.observe(row, lineNum)
		// engine.AuditRowIsPass is the same predicate §2.1 pins for the streak
		// walk, and its doc comment requires the two to stay identical. Calling
		// it here rather than restating it makes that identity structural: the
		// round's recorded findings count and other_roles_open cannot drift.
		if !engine.AuditRowIsPass(row) {
			findings++
		}
	}
	rl.notice(path)
	return findings, nil
}

// runAuditStatus implements `tp audit <spec> --status [--check]`. The shape
// has no mechanical_checks field — workflow checks guard review rounds.
func runAuditStatus(specPath string, check bool) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
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
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %s", specPath), err.Error())
		os.Exit(ExitFile)
		return nil
	}

	rounds := []engine.ReviewRound{}
	if st != nil {
		rounds = st.AuditRounds
	}
	converged := engine.Converged(rounds, wf.AuditCleanRounds, specHash)
	stale := engine.StateStale(rounds, specHash)
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseAuditors)

	result := map[string]any{
		"audit_rounds":          rounds,
		"consecutive_clean":     engine.ConsecutiveClean(rounds),
		"required_clean_rounds": wf.AuditCleanRounds,
		"converged":             converged,
		"stale":                 stale,
		"roles_stale":           engine.RolesStale(rounds, rolesHash),
	}
	// §2.5: the three fields are written HERE, before the --check exit-code
	// branch at the end of this function, so `--status --check` — the invocation
	// a gated driver actually runs — carries exactly the payload `--status`
	// carries without the flag. Condition 5 compares the latest round's stored
	// corpus hash against rolesHash, the one this path already computed for
	// roles_stale, so a corpus edited after the latest round withholds the object.
	auditSignalFields(result, specPath, rounds, wf.AuditCleanRounds, stale, converged, rolesHash)
	// §8.4: harness_stale and harness_note are explanatory and are omitted under
	// --compact; next_action is decision-critical and survives it. When emitted,
	// harness_note is the verbatim stored note of the latest recorded audit round
	// and appears only when harness_stale is true (§6.3).
	if !IsCompact() {
		result["harness_stale"] = engine.HarnessStale(rounds)
		if engine.HarnessStale(rounds) {
			result["harness_note"] = engine.LatestHarnessNote(rounds)
		}
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
		output.Error(ExitFile, jsonErr.Error(), internalEncodeHint)
	}

	if check && !converged {
		os.Exit(ExitValidation)
	}
	return nil
}
