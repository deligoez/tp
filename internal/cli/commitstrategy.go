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
// project config, then applying auto-detection. It emits any one-line stderr
// warning as a side effect: an unrecognized commit_strategy value, and — when
// the effective strategy is hc while hc is absent from PATH — a courtesy notice
// that the agent, not tp, will fail to run hc (§5.2, §5.3). Neither warning
// changes the exit code.
func resolveEffectiveStrategy(taskFilePath string) string {
	taskOverride, _ := engine.LoadTaskWorkflowOverride(taskFilePath)
	project := engine.ProjectWorkflowOverride(".")
	name, warning := engine.ResolveCommitStrategy(taskOverride.CommitStrategy, project.CommitStrategy)
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	effective, hcMissing := engine.EffectiveCommitStrategy(name, engine.HasHC())
	if hcMissing {
		fmt.Fprintln(os.Stderr, "warning: commit_strategy is hc but hc is not on PATH; commit with hc yourself")
	}
	return effective
}
