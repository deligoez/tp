package cli

import (
	"fmt"
	"slices"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// checkRanClasses returns the classes a registered check ran for in one
// invocation: an entry of the class the runner reported `ran: true`, and no
// entry of it reported `ran: false`. A class with no entry at all never
// started — no task file resolving leaves the runner with none — so it is
// absent too (engine.CheckRan states the contract).
func checkRanClasses(results []map[string]any) map[string]bool {
	ranFor := make(map[string]bool)
	notRun := make(map[string]bool)
	for _, entry := range results {
		class, _ := entry["class"].(string)
		if ran, _ := entry["ran"].(bool); ran {
			ranFor[class] = true
		} else {
			notRun[class] = true
		}
	}
	for class := range notRun {
		delete(ranFor, class)
	}
	return ranFor
}

// registeredMechanized is §3.2's registration rule: a valid checks entry of the
// class mechanizes it. Plain --status keeps it, because it runs no check.
func registeredMechanized(checks []model.Check) func(string) bool {
	return func(class string) bool { return engine.IsMechanizedClass(checks, class) }
}

// ranMechanized is the rule of every mode that runs the checks: the class is
// registered, and a check of it ran in this invocation. A check that could not
// run, or never started, verified nothing, so it mechanizes nothing — the same
// rule the reviewer exclusion applies (mechanizedExclusion).
func ranMechanized(checks []model.Check, results []map[string]any) func(string) bool {
	ran := checkRanClasses(results)
	return func(class string) bool { return ran[class] && engine.IsMechanizedClass(checks, class) }
}

// checkVerdict reads the runner's results for next_action: every entry that
// did not pass, in the order the runner reported it, then every registered
// check the runner holds no entry for. It lists exactly what makes
// runMechanicalChecks' allPass false, so next_action names a check whenever
// `--status --check` would exit 1 on one.
func checkVerdict(checks []model.Check, results []map[string]any, taskFilePath string) engine.CheckVerdict {
	var v engine.CheckVerdict
	for _, entry := range results {
		if passed, _ := entry["passed"].(bool); passed {
			continue
		}
		class, _ := entry["class"].(string)
		ran, _ := entry["ran"].(bool)
		outcome := "timed out or failed to start"
		if code, _ := entry["exit_code"].(*int); code != nil {
			outcome = fmt.Sprintf("exited %d", *code)
		}
		v.Failing = append(v.Failing, engine.CheckFailure{Class: class, Ran: ran, Outcome: outcome})
	}
	for i := range checks {
		switch {
		case taskFilePath == "":
			v.Failing = append(v.Failing, engine.CheckFailure{Class: checks[i].Class, Outcome: "never started: no task file resolves"})
		case engine.ValidateChecks([]model.Check{checks[i]}) != nil:
			v.Failing = append(v.Failing, engine.CheckFailure{Class: checks[i].Class, Outcome: "never started: its entry fails the checks schema"})
		}
	}
	return v
}

// recordChecks runs the registered checks for --record where their result can
// change its payload, and returns the membership rule and the verdict it reads,
// plus the runner's results for the payload's mechanical_checks — nil when
// nothing ran. A candidate class with a registered check needs the result to
// know whether the check mechanizes it, and a loop that is done or at its cap
// needs it before next_action names an import step. Anywhere else nothing
// runs: with no candidate registered both rules keep every candidate, and no
// next_action branch that reads the verdict can fire. With no check registered
// there is nothing to run at all.
func recordChecks(wf *model.Workflow, taskFilePath string, candidates []mechanizeCandidate, done engine.LoopDone) (mechanized func(string) bool, verdict engine.CheckVerdict, results []map[string]any) {
	registered := registeredMechanized(wf.Checks)
	candidateRegistered := slices.ContainsFunc(candidates, func(c mechanizeCandidate) bool { return registered(c.Class) })
	if len(wf.Checks) == 0 || (!done.Done && !done.CapReached && !candidateRegistered) {
		return registered, engine.CheckVerdict{}, nil
	}
	results, _ = runMechanicalChecks(wf, taskFilePath)
	return ranMechanized(wf.Checks, results), checkVerdict(wf.Checks, results, taskFilePath), results
}
