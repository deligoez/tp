package cli

import (
	"fmt"
	"os"
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
		"mechanical_checks":     mechChecks,
	}
	if wf.ReviewMaxRounds > 0 {
		result["budget_exhausted"] = len(rounds) >= wf.ReviewMaxRounds && !converged
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
