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

	converged := engine.Converged(rounds, wf.ReviewCleanRounds, specHash)
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseReviewers)

	var mechChecks []map[string]any
	allPass := true
	if check {
		mechChecks, allPass = runMechanicalChecks(&wf, taskFilePath)
	} else {
		mechChecks = registeredChecksList(&wf)
	}

	result := map[string]any{
		"review_rounds":         rounds,
		"consecutive_clean":     engine.ConsecutiveClean(rounds),
		"required_clean_rounds": wf.ReviewCleanRounds,
		"converged":             converged,
		"stale":                 engine.StateStale(rounds, specHash),
		"roles_stale":           engine.RolesStale(rounds, rolesHash),
		"mechanical_checks":     mechChecks,
		"overlap_report":        latestRoundOverlapReport(specPath, rounds),
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
	// §10.2: surface an interrupted round — a snapshot exists for the next
	// round but its review-round-N.ndjson was never recorded.
	if inFlight := engine.InFlightRound(specPath, len(rounds)); inFlight > 0 {
		result["in_flight_round"] = inFlight
	} else {
		result["in_flight_round"] = nil
	}

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
			output.Info(fmt.Sprintf("skipping invalid check %d (%s): %v", i, c.Class, err))
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
