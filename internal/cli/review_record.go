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
func runReviewRecord(specPath, recordPath, harnessNote string) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	// Corrupt state aborts before any parsing or write
	stPre, err := engine.LoadReviewState(specPath)
	if err != nil {
		// Except the snapshot-only window, which tp audit's emission opens and
		// this command can land in: nothing was ever recorded there, so
		// EnsureReviewState below rebuilds the index rather than sending the
		// caller to repair a healthy directory. A round file with no index is
		// still lost history and still aborts.
		if !engine.IsRebuildableStateIndex(err) {
			exitStateError(err)
			return nil
		}
		stPre = nil
	}

	// Round-budget refusal comes before line parsing and any state write
	wfPre, _ := engine.ResolveWorkflow(specPath, flagFile)

	// A consuming command validates the resolved review_converge_on: an invalid
	// value winning from a stored layer (env, .tp/config.json, or a task
	// override) is a validation error (exit 1), not a usage error (§3.3).
	if !engine.ValidReviewConvergeOn(wfPre.ReviewConvergeOn) {
		output.Error(ExitValidation, fmt.Sprintf("invalid review_converge_on value %q", wfPre.ReviewConvergeOn), engine.ReviewConvergeOnHint)
		os.Exit(ExitValidation)
		return nil
	}

	preRounds := []engine.ReviewRound{}
	if stPre != nil {
		preRounds = stPre.ReviewRounds
	}
	refuseIfBudgetExhausted("review", specPath, preRounds, wfPre.ReviewMaxRounds, wfPre.ReviewCleanRounds, wfPre.ReviewConvergeOn)

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read findings file: %s: %v", recordPath, err), recordFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	findings, dirty, parseHint, parseErr := parseRecordRows(recordPath, data)
	if parseErr != nil {
		// A malformed row is a fault in the file read from disk, not the
		// invocation, so it is a validation error (exit 1), not a usage error
		// (exit 2) — mirroring an invalid stored review_converge_on above (§3.3).
		//
		// Most row rules carry no advice of their own, and an empty hint here is
		// the code-1 default: task-file advice over a file that is not the task
		// file (§9.2). The fallback is applied at this sink rather than inside
		// parseRecordRows so a rule added there cannot reintroduce the default.
		if parseHint == "" {
			parseHint = recordRowHint
		}
		output.Error(ExitValidation, parseErr.Error(), parseHint)
		os.Exit(ExitValidation)
		return nil
	}
	clean := dirty == 0

	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %s", specPath), err.Error())
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
		// Same guard as the audit record path: LoadReviewState returns
		// (nil, nil) when state.json is gone and no artifacts remain, and an
		// external delete can produce that between EnsureReviewState and this
		// lock. Without it the dereference below panics instead of aborting.
		if st == nil {
			// A StateCorruptError rather than a bare error: it carries the
			// repair hint exitStateError attaches, and it keeps this message
			// identical to the one the sibling record path produces.
			return &engine.StateCorruptError{
				Path:   engine.ReviewStateDir(specPath),
				Reason: vanishedStateReason,
			}
		}
		round = len(st.ReviewRounds) + 1
		fileName := fmt.Sprintf("review-round-%d.ndjson", round)
		// Round file first, index entry second
		if writeErr := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), fileName), data, 0o600); writeErr != nil {
			return writeErr
		}
		st.ReviewRounds = append(st.ReviewRounds, engine.ReviewRound{
			Round:       round,
			Findings:    findings,
			Clean:       clean,
			RecordedAt:  time.Now().UTC().Format(time.RFC3339),
			File:        fileName,
			SpecHash:    specHash,
			RolesHash:   rolesHash,
			HarnessNote: harnessNote,
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
			output.Notice(fmt.Sprintf("round %d file %s is missing; skipping its rows", r.Round, r.File))
			continue
		}
		roundFindings = append(roundFindings, rows)
	}
	candidates := computeMechanizeCandidates(roundFindings)

	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	// §3.2 candidate suppression, mode 1: a class mechanized by a valid `checks`
	// entry is withheld from all three of this mode's sinks — the emitted
	// mechanize_candidates array, the register-a-check hint that accompanies it,
	// and the class list handed to next_action below. The filter runs after the
	// frequency threshold rather than inside it, so suppressing one class never
	// changes whether another crosses it, and it keeps candidates a non-nil
	// slice so the array stays [] and never null on a round it empties.
	// §3.3: mechanizedClasses is the withheld half of that same split — the
	// classes this filter dropped, each once and sorted ascending — and is
	// emitted beside the filtered array on this mode alone. Deriving it from the
	// filter rather than re-testing membership keeps the two halves of the list
	// one decision. It lists the intersection: a registered class that never
	// reached candidate frequency was never a candidate, so the filter never saw
	// it and it is absent here.
	candidates, mechanizedClasses := filterMechanizedCandidates(candidates, wf.Checks)
	// clean/consecutive_clean/converged are recomputed live from the round's
	// recorded findings under the current review_converge_on (§3.4) — the
	// stored ReviewRound.Clean stays the frozen record-time value. This is the
	// same live predicate --status reports, so both agree on the just-recorded
	// round.
	liveClean := engine.ReviewRoundClean(specPath, &st.ReviewRounds[round-1], wf.ReviewConvergeOn)
	// One walk, not two: ReviewConverged re-reads every recorded round's file,
	// and it was previously called once for the payload and once for
	// next_action. The audit record path hoists the same value; keeping the
	// two paths consistent is what stops the cost from creeping back.
	converged := engine.ReviewConverged(specPath, st.ReviewRounds, wf.ReviewCleanRounds, specHash, wf.ReviewConvergeOn)
	result := map[string]any{
		"round":                 round,
		"findings":              findings,
		"clean":                 liveClean,
		"consecutive_clean":     engine.ReviewConsecutiveClean(specPath, st.ReviewRounds, wf.ReviewConvergeOn),
		"required_clean_rounds": wf.ReviewCleanRounds,
		"converged":             converged,
		"stale":                 engine.StateStale(st.ReviewRounds, specHash),
		"mechanize_candidates":  candidates,
		"mechanized_classes":    mechanizedClasses,
	}
	// §8.4: harness_stale and harness_note are explanatory and are omitted under
	// --compact; next_action and nonblocking_open are decision-critical and
	// survive it. When emitted, harness_note is the verbatim stored note (§6.2);
	// --record reports staleness AFTER storing this round, so the just-recorded
	// round is the latest of the two compared.
	if !IsCompact() {
		result["harness_stale"] = engine.HarnessStale(st.ReviewRounds)
		if engine.HarnessStale(st.ReviewRounds) {
			result["harness_note"] = engine.LatestHarnessNote(st.ReviewRounds)
		}
	}
	// §4.2: on the accepted-open case — the just-recorded round is clean ONLY
	// because every surviving finding is below the blocking severities — carry
	// nonblocking_open = the count of surviving non-blocking (medium/low)
	// findings. Emitted solely when clean and that count is positive, so the
	// field's mere presence signals the accepted-open state; absent on a
	// non-clean round, on a clean round with zero non-blocking survivors, and
	// under review_converge_on=all. accepted_open is intentionally not emitted.
	if liveClean {
		if nbo := engine.ReviewRoundNonBlockingOpen(specPath, &st.ReviewRounds[round-1], wf.ReviewConvergeOn); nbo > 0 {
			result["nonblocking_open"] = nbo
		}
	}
	if len(candidates) > 0 {
		result["hint"] = mechanizeRegisterHint
	}
	// §8.1/§8.2: next_action names the single next step by the fixed precedence.
	// Advisory/read-only — it changes nothing and never gates the exit code. The
	// just-recorded round is the latest, so branch 2's "convergence-blocking
	// finding survives in the latest round" is exactly !liveClean; branch 3 reads
	// the same mechanize candidates surfaced above.
	result["next_action"] = engine.ReviewNextAction(specPath, converged, !liveClean, mechanizeCandidateClasses(candidates))
	return output.JSON(result)
}

// parseRecordRows applies the row rules: blank lines skipped, every remaining
// line a JSON object, pre-resolved wontfix needs evidence and does not dirty
// the round, pre-resolved fixed aborts, pre-resolved duplicate dirties.
func parseRecordRows(path string, data []byte) (findings, dirty int, hint string, err error) {
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
			return 0, 0, "", fmt.Errorf("line %d: invalid JSON: %w", lineNum, jsonErr)
		}
		findings++

		rl.observe(row, lineNum)

		status, evidence := resolvedStatusOf(row)
		switch status {
		case "fixed":
			return 0, 0, "re-review the changed spec", fmt.Errorf("line %d: row arrives pre-resolved fixed — a fix means the spec changed; record the round without it and re-review", lineNum)
		case "wontfix":
			if strings.TrimSpace(evidence) == "" {
				return 0, 0, "", fmt.Errorf("line %d: pre-resolved wontfix row requires non-empty resolved.evidence", lineNum)
			}
			// verified-rejected rows do not dirty the round
		default:
			// unresolved and pre-resolved duplicate rows dirty the round
			dirty++
		}
	}
	rl.notice(path)
	return findings, dirty, "", nil
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

// rolelessRows accumulates the rows of one NDJSON file that carry no role, so
// the condition costs the reader ONE advisory rather than one per row. Both
// record paths — review findings and audit results — apply the same rule to
// the same field, so the count, the first offending line, the wording and the
// noticeOnce key live here once and cannot drift apart per phase.
type rolelessRows struct {
	count     int
	firstLine int
}

// observe folds one parsed row into the tally, remembering the first line that
// lacked a role.
func (r *rolelessRows) observe(row map[string]any, lineNum int) {
	if role, _ := row["role"].(string); role != "" {
		return
	}
	r.count++
	if r.firstLine == 0 {
		r.firstLine = lineNum
	}
}

// notice emits the single advisory for path, when any row lacked a role. Per
// row this printed N byte-identical-but-for-the-line-number copies (a 48-row
// file cost ~5KB), and on raw os.Stderr it ignored --quiet; output.Notice
// honours it, and the count is the part a reader needs that the per-row form
// never gave. Call it only on the success path: a round that aborts mid-parse
// records nothing, so its partial tally would advise about rows no state ever
// kept.
func (r *rolelessRows) notice(path string) {
	if r.count == 0 {
		return
	}
	noticeOnce("roleless-rows:"+path, fmt.Sprintf(
		"warning: %d row(s) in %s are missing the role field (first at line %d); they will not appear in the per-role overlap report",
		r.count, path, r.firstLine))
}

// vanishedStateReason is the StateCorruptError reason both record paths raise
// when the state directory disappears between EnsureReviewState and the write
// lock. One string, so the two guards cannot report the same condition in two
// different words.
const vanishedStateReason = "it disappeared while a round was being recorded"

// exitStateError reports state-layer failures: corrupt state exits 3 with the
// repair hint, write-lock contention exits 4 with the lock hint, and anything
// else exits 3 with the raw error.
func exitStateError(err error) {
	var ce *engine.StateCorruptError
	if errors.As(err, &ce) {
		output.Error(ExitFile, ce.Error(), ce.Hint())
		os.Exit(ExitFile)
		return
	}
	// §12.2: contention that retried past lock_timeout_seconds is a state error
	// (exit 4), not a file error, and its hint names the lock and the wait.
	// root.go maps it that way for commands that return the error up; the record
	// paths abort here instead, so without this branch a parallel --record
	// fan-out reports exit 3 with a hint pointing at unrelated commands.
	var lockErr *engine.LockTimeoutError
	if errors.As(err, &lockErr) {
		output.Error(ExitState, lockErr.Error(), lockErr.Hint())
		os.Exit(ExitState)
		return
	}
	output.Error(ExitFile, err.Error(), stateWriteHint)
	os.Exit(ExitFile)
}
