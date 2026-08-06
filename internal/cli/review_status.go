package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// runReviewStatus implements `tp review <spec> --status [--check]`.
func runReviewStatus(specPath string, check bool) error {
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

	wf, taskFilePath := engine.ResolveWorkflow(specPath, flagFile)

	// A consuming command validates the resolved review_converge_on: an invalid
	// value winning from a stored layer (env, .tp/config.json, or a task
	// override) is a validation error (exit 1), not a usage error (§3.3).
	if !engine.ValidReviewConvergeOn(wf.ReviewConvergeOn) {
		output.Error(ExitValidation, fmt.Sprintf("invalid review_converge_on value %q", wf.ReviewConvergeOn), engine.ReviewConvergeOnHint)
		os.Exit(ExitValidation)
		return nil
	}

	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	rounds := []engine.ReviewRound{}
	if st != nil {
		rounds = st.ReviewRounds
	}

	// The review clean flag is recomputed live from each round's recorded
	// findings under the current review_converge_on and current resolution
	// state (§3.4) — never the stored boolean. Overwriting the in-memory
	// entries (this response is not persisted) keeps review_rounds[].clean,
	// consecutive_clean, and converged consistent, and lets switching the
	// setting or resolving a finding re-evaluate a round without re-recording.
	for i := range rounds {
		rounds[i].Clean = engine.ReviewRoundClean(specPath, &rounds[i], wf.ReviewConvergeOn)
	}

	converged := engine.Converged(rounds, wf.ReviewCleanRounds, specHash)
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseReviewers)

	var mechChecks []map[string]any
	allPass := true
	if check {
		mechChecks, allPass = runMechanicalChecks(&wf, taskFilePath)
	} else {
		mechChecks = registeredChecksList(&wf)
	}

	// §9.2: attribution_excludes surfaces the regression exclusion only when it
	// caused merged_count to exceed the overlap-report finding count of the
	// latest recorded round. overlap_report itself is decision-critical
	// (trim-candidate signal) and survives --compact (§8.4); attribution_excludes
	// is explanatory and is omitted under --compact.
	overlapReport, attributionExcludes := latestRoundOverlapAndAttribution(specPath, rounds)
	result := map[string]any{
		"review_rounds":         rounds,
		"consecutive_clean":     engine.ConsecutiveClean(rounds),
		"required_clean_rounds": wf.ReviewCleanRounds,
		"converged":             converged,
		"stale":                 engine.StateStale(rounds, specHash),
		"roles_stale":           engine.RolesStale(rounds, rolesHash),
		"mechanical_checks":     mechChecks,
		"overlap_report":        overlapReport,
	}
	// §8.4: harness_stale and harness_note are explanatory and are omitted under
	// --compact; next_action and nonblocking_open are decision-critical and
	// survive it. When emitted, harness_note is the verbatim stored note of the
	// latest recorded round and appears only when harness_stale is true (§6.2).
	if !IsCompact() {
		result["harness_stale"] = engine.HarnessStale(rounds)
		if engine.HarnessStale(rounds) {
			result["harness_note"] = engine.LatestHarnessNote(rounds)
		}
	}
	if !IsCompact() && len(attributionExcludes) > 0 {
		result["attribution_excludes"] = attributionExcludes
	}
	// §10.1: surface the effective cap and remaining budget next to
	// budget_exhausted; null when uncapped. Decision-critical, so these
	// survive --compact (§8.4).
	if wf.ReviewMaxRounds > 0 {
		result["max_rounds"] = wf.ReviewMaxRounds
		remaining := wf.ReviewMaxRounds - len(rounds)
		if remaining < 0 {
			remaining = 0
		}
		result["rounds_remaining"] = remaining
		result["budget_exhausted"] = len(rounds) >= wf.ReviewMaxRounds && !converged
	} else {
		result["max_rounds"] = nil
		result["rounds_remaining"] = nil
	}
	// §4.2: on the accepted-open case — the latest recorded round is clean ONLY
	// because every surviving finding is below the blocking severities — carry
	// nonblocking_open = the count of surviving non-blocking (medium/low)
	// findings. Emitted solely when that round is clean and the count is
	// positive, so the field's mere presence signals the accepted-open state;
	// absent on a non-clean latest round, on a clean one with zero non-blocking
	// survivors, and under review_converge_on=all. rounds[i].Clean above is the
	// live severity-aware value. accepted_open is intentionally not emitted.
	if n := len(rounds); n > 0 && rounds[n-1].Clean {
		if nbo := engine.ReviewRoundNonBlockingOpen(specPath, &rounds[n-1], wf.ReviewConvergeOn); nbo > 0 {
			result["nonblocking_open"] = nbo
		}
	}

	// §10.2: surface an interrupted round — a snapshot exists for the next
	// round but its review-round-N.ndjson was never recorded.
	if inFlight := engine.InFlightRound(specPath, engine.PhaseReview, len(rounds)); inFlight > 0 {
		result["in_flight_round"] = inFlight
	} else {
		result["in_flight_round"] = nil
	}

	// §8.1/§8.2: next_action names the single next step by the fixed precedence.
	// Advisory/read-only — it changes nothing and never gates the exit code.
	// blockingUnresolved is the latest recorded round holding a surviving
	// convergence-blocking finding (its live severity-aware clean flag is false);
	// mechanize candidates are derived here from the recorded rounds by the same
	// threshold --record uses, so branch 3 is reachable on --status too.
	blockingUnresolved := len(rounds) > 0 && !rounds[len(rounds)-1].Clean
	result["next_action"] = engine.ReviewNextAction(specPath, converged, blockingUnresolved, mechanizeClassesFromRounds(specPath, rounds))

	if jsonErr := output.JSON(result); jsonErr != nil {
		output.Error(ExitFile, jsonErr.Error())
	}

	if check && (!converged || !allPass) {
		os.Exit(ExitValidation)
	}
	return nil
}

// registeredChecksList renders the registered checks with no execution results.
func registeredChecksList(wf *model.Workflow) []map[string]any {
	list := make([]map[string]any, 0, len(wf.Checks))
	for i := range wf.Checks {
		list = append(list, map[string]any{"class": wf.Checks[i].Class, "cmd": wf.Checks[i].Cmd})
	}
	return list
}

// runMechanicalChecks executes every registered workflow check sequentially
// in the resolved task file's directory with the resolved gate timeout per
// check. output_tail is present only for failed checks. Entries failing the
// checks schema are skipped with an info line. When no task file resolves,
// no checks are registered and none run.
func runMechanicalChecks(wf *model.Workflow, taskFilePath string) (results []map[string]any, allPass bool) {
	results = make([]map[string]any, 0, len(wf.Checks))
	allPass = true
	if taskFilePath == "" {
		return results, allPass
	}
	dir := gateDir(taskFilePath)
	timeout := time.Duration(wf.EffectiveGateTimeoutSeconds()) * time.Second

	for i := range wf.Checks {
		c := wf.Checks[i]
		if err := engine.ValidateChecks([]model.Check{c}); err != nil {
			output.Notice(fmt.Sprintf("skipping invalid check %d (%s): %v", i, c.Class, err))
			continue
		}
		res := engine.RunCommand(c.Cmd, dir, timeout, gateOutputTailLines)
		entry := map[string]any{
			"class":     c.Class,
			"cmd":       c.Cmd,
			"passed":    res.Passed,
			"exit_code": res.ExitCode,
		}
		if !res.Passed {
			entry["output_tail"] = res.OutputTail
			allPass = false
		}
		results = append(results, entry)
	}
	return results, allPass
}
