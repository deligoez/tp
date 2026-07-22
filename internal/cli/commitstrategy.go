package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
)

// hcCommitHint is the hint shown when a commit-making command is used under an
// effective hc strategy: making the commit is the agent's job (via hc), and tp
// only records the SHA (§5.4).
const hcCommitHint = "commit_strategy is hc: commit with hc, then tp done --commit <sha>"

// resolveEffectiveStrategy resolves the effective commit strategy (builtin or
// hc) for taskFilePath by layering the task-file workflow override over the
// project config, warning as a side effect (see warnCommitStrategy).
func resolveEffectiveStrategy(taskFilePath string) string {
	// A parse error yields an empty override (commit_strategy treated as unset);
	// the command's own model.ReadTaskFile then aborts with exit 3 on a truly
	// malformed file, so dropping the error here loses no safety (cf. config.go).
	taskOverride, _ := engine.LoadTaskWorkflowOverride(taskFilePath)
	project := engine.ProjectWorkflowOverride(".")
	return warnCommitStrategy(taskOverride.CommitStrategy, project.CommitStrategy)
}

// warnCommitStrategy resolves the effective commit strategy (builtin or hc) for
// an override/project layer pair and emits the strategy-reading warnings as a
// side effect: an unrecognized commit_strategy value, and — when the effective
// strategy is hc while hc is absent from PATH — a courtesy notice that the
// agent, not tp, will fail to run hc (§5.2, §5.3). Neither changes the exit code.
func warnCommitStrategy(override, project *string) string {
	name, warning := engine.ResolveCommitStrategy(override, project)
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	eff, hcMissing := engine.EffectiveCommitStrategy(name, engine.HasHC())
	if hcMissing {
		fmt.Fprintln(os.Stderr, "warning: commit_strategy is hc but hc is not on PATH; commit with hc yourself")
	}
	return eff
}
