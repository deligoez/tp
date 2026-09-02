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
		if !engine.IsRebuildableStateIndex(err) {
			exitStateError(err)
			return nil
		}
		// The emission wrote a round snapshot and tp audit never calls
		// EnsureReviewState, so state.json is legitimately absent here — the
		// in-flight window refuseAuditIfBudgetExhausted documents. Reading it as
		// corruption made the FIRST audit round of any spec with no prior tp
		// review unrecordable: emit, then --record, then exit 3 telling the
		// caller to delete a healthy state directory. EnsureReviewState below
		// rebuilds the index when nothing but snapshots is there, and still
		// aborts when a round file says recorded history went missing.
		stPre = nil
	}

	// Round-budget refusal comes before line parsing and any state write
	wfPre, _ := engine.ResolveWorkflow(specPath, flagFile)

	// A consuming command validates the resolved audit_converge_on: an illegal
	// value winning from a stored layer (.tp/config.json or a task override) is
	// a validation error (exit 1), not the usage error (exit 2) a write sink
	// reports for an illegal literal argument (§2).
	//
	// It is checked HERE, on wfPre, and not on the wf resolved further down:
	// EnsureReviewState and recordAuditRoundEntry both run before that second
	// resolution, so a refusal placed there would exit non-zero with round N
	// already written into a store §2 declares immutable — recorded under a
	// policy tp refused to read. Nothing above this line writes.
	if !engine.ValidAuditConvergeOn(wfPre.AuditConvergeOn) {
		output.Error(ExitValidation, fmt.Sprintf("invalid audit_converge_on value %q", wfPre.AuditConvergeOn), engine.AuditConvergeOnHint)
		os.Exit(ExitValidation)
		return nil
	}

	preRounds := []engine.ReviewRound{}
	if stPre != nil {
		preRounds = stPre.AuditRounds
	}
	// The audit half of the same exemption: a re-record adds no round, so it
	// cannot exhaust the round budget (section 6.3).
	if _, rewrite := engine.RecordRound(len(preRounds)); !rewrite {
		refuseIfBudgetExhausted("audit", specPath, preRounds, wfPre.AuditMaxRounds, wfPre.AuditCleanRounds, "")
	}

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read results file: %s: %v", recordPath, err), recordFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	rows, findings, parseErr := parseAuditRows(recordPath, data)
	if parseErr != nil {
		// A malformed row is a fault in the results file, not in the invocation,
		// so it is a validation error (exit 1) — and its hint names that file,
		// not the task file the code-1 default would send the reader to (§9.2).
		output.Error(ExitValidation, parseErr.Error(), recordRowHint)
		os.Exit(ExitValidation)
		return nil
	}
	// §2: `clean` is stamped here, once, under the policy in force at record
	// time, and nothing downstream recomputes it. The policy comes from wfPre
	// and not from the wf resolved after recordAuditRoundEntry, for the reason
	// the refusal above uses wfPre: the round is written before that second
	// resolution, so a stamp taken from it would be a verdict computed after
	// the round it grades was already in a store §2 declares immutable.
	//
	// This is where the two values part company. `findings` stays the raw count
	// of non-PASS rows — under `blocking` a round can be clean while findings
	// is non-zero, which is exactly the accepted-rows case §2 describes — so
	// the pre-release equality `clean := findings == 0` is not a shortcut worth
	// keeping beside this call.
	clean := engine.AuditRowsClean(rows, wfPre.AuditConvergeOn)

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
	// just-recorded round is the latest, and v0.37.0 §2 makes its two facts two
	// arguments: the stamped `clean` verdict picks the branch, while `findings`
	// — the raw non-PASS count, positive on a clean round under `blocking` — is
	// what the converged and clean-but-not-converged branches render.
	result["next_action"] = engine.AuditNextAction(specPath, converged, clean, findings)
	return output.JSON(result)
}

// auditSignalFields writes §2.5's three fields onto an audit payload:
// role_streaks, spec_coverage_clean_rounds and divergence. Three outputs carry
// them: `tp audit <spec> --status`, with or without --check and before its
// exit-code branch; `tp audit <spec> --record <file>` after the round is
// stored; and `tp run --status` on an audit-phase stop, through
// addAuditSignals, which §1/§3.5 requires to report the same numbers as the
// audit command. Prompt emission, `tp audit --merge` and every `tp review` mode
// stay free of them.
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
		// Section 6.3, the audit half of the same rule the review recorder
		// applies: idempotent on TP_ROUND, additive by hand.
		var rewrite bool
		round, rewrite = engine.RecordRound(len(st.AuditRounds))
		fileName := fmt.Sprintf("audit-round-%d.ndjson", round)
		if writeErr := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), fileName), data, 0o600); writeErr != nil {
			return writeErr
		}
		entry := engine.ReviewRound{
			Round:       round,
			Findings:    findings,
			Clean:       clean,
			RecordedAt:  time.Now().UTC().Format(time.RFC3339),
			File:        fileName,
			SpecHash:    specHash,
			RolesHash:   rolesHash,
			IDScheme:    engine.IDSchemeSlug,
			HarnessNote: harnessNote,
		}
		if rewrite {
			st.AuditRounds[round-1] = entry
		} else {
			st.AuditRounds = append(st.AuditRounds, entry)
		}
		return engine.SaveReviewState(specPath, st)
	})
	return st, round, rolesHash, err
}

// parseAuditRows parses rows with the shared line rules, returning them
// alongside the count of rows whose status is absent or not exactly "PASS".
//
// The rows are returned and not only their count because §2 stamps the round's
// `clean` from engine.AuditRowsClean, whose subject is the non-PASS rows
// themselves: under `blocking` the verdict turns on each row's severity, which
// a tally has already thrown away. One walk produces both, so the count the
// round records and the rows it is graded from cannot describe different files.
func parseAuditRows(path string, data []byte) (rows []map[string]any, findings int, err error) {
	lineNum := 0
	var rl rolelessRows
	var ic invalidCategoryRows
	for line := range strings.SplitSeq(string(data), "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var row map[string]any
		if jsonErr := json.Unmarshal([]byte(trimmed), &row); jsonErr != nil {
			return nil, 0, fmt.Errorf("line %d: invalid JSON: %w", lineNum, jsonErr)
		}
		rows = append(rows, row)
		rl.observe(row, lineNum)
		ic.observe(row, lineNum)
		// engine.AuditRowIsPass is the same predicate §2.1 pins for the streak
		// walk, and its doc comment requires the two to stay identical. Calling
		// it here rather than restating it makes that identity structural: the
		// round's recorded findings count and other_roles_open cannot drift.
		if !engine.AuditRowIsPass(row) {
			findings++
		}
	}
	if err := ic.err(); err != nil {
		return nil, 0, err
	}
	rl.notice(path)
	return rows, findings, nil
}

// invalidCategoryRows collects rows whose category is outside the enum
// engine.IsValidCategory accepts, so one bad file costs the operator ONE
// round-trip: every offending line is named at once rather than the first one
// aborting and the next appearing on the retry.
//
// It exists because tp renders the enum into every auditor prompt through
// engine.RenderAuditCategoryText, in words that promise "unknown values are
// rejected" — and then never looked at what came back. A row could carry any
// string at all and be recorded into the permanent round file that convergence
// is computed from. The validator had been written and tested since the
// category enum landed, and nothing ever called it.
//
// Rejecting rather than warning is what the prompt already promises, and it
// matches the invalid-JSON abort a few lines above: a row that does not meet
// the stated schema does not enter the record.
type invalidCategoryRows struct {
	lines []string
}

// observe folds one parsed row into the tally, remembering each line that
// carried a category outside the enum and what it said.
func (r *invalidCategoryRows) observe(row map[string]any, lineNum int) {
	category, _ := row["category"].(string)
	if category == "" || engine.IsValidCategory(category) {
		return
	}
	r.lines = append(r.lines, fmt.Sprintf("line %d: %q", lineNum, category))
}

// err returns the abort for path, or nil when every category was in the enum.
func (r *invalidCategoryRows) err() error {
	if len(r.lines) == 0 {
		return nil
	}
	return fmt.Errorf("category outside the enum at %s; tp names only %s",
		strings.Join(r.lines, "; "), strings.Join(engine.AuditCategories(), ", "))
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
		if !engine.IsRebuildableStateIndex(err) {
			exitStateError(err)
			return nil
		}
		// Same in-flight window as runAuditRecord: after the first emission a
		// snapshot exists with no index yet, and --status reporting that as
		// corruption put the whole §2.5 signal out of reach exactly when a
		// caller would first ask for it. Rebuildable only — a round file with
		// no index is history tp lost, and reporting an empty audit_rounds for
		// it would be the false-clean this release exists to prevent.
		st = nil
	}

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)

	// The read half of the same rule (§2): --status reports convergence, so an
	// illegal stored audit_converge_on is refused here too, with the same
	// validation code (exit 1) and the same hint. Refusing on only one of the
	// two sinks would let a gated driver read `converged` off a policy tp would
	// not accept from the sink that records it.
	if !engine.ValidAuditConvergeOn(wf.AuditConvergeOn) {
		output.Error(ExitValidation, fmt.Sprintf("invalid audit_converge_on value %q", wf.AuditConvergeOn), engine.AuditConvergeOnHint)
		os.Exit(ExitValidation)
		return nil
	}

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
		remaining := max(wf.AuditMaxRounds-len(rounds), 0)
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
	// latest recorded round supplies both inputs v0.37.0 §2 separated, and both
	// are read from the store rather than recomputed: its stamped Clean verdict
	// picks the branch, its stored Findings count is the numeral the converged
	// and clean-but-not-converged branches render. This sink is changed with the
	// --record one and not after it — otherwise `--status`, the invocation a
	// gated driver actually runs, stays silent about rows `--record` named. With
	// no round recorded there is nothing to be unclean about and nothing to
	// count, which is the state the pre-release `len(rounds) > 0 &&` guarded.
	latestClean, latestFindings := true, 0
	if n := len(rounds); n > 0 {
		latestClean, latestFindings = rounds[n-1].Clean, rounds[n-1].Findings
	}
	result["next_action"] = engine.AuditNextAction(specPath, converged, latestClean, latestFindings)

	if jsonErr := output.JSON(result); jsonErr != nil {
		// Exiting, not falling through: without this the process printed a
		// code-3 envelope on stderr and then returned 0 (or 1 under --check),
		// so a truncated payload read as a successful status.
		output.Error(ExitFile, jsonErr.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}

	if check && !converged {
		os.Exit(ExitValidation)
	}
	return nil
}
