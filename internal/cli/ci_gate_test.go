package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCIRunsEveryStepOfTheProjectGate pins the two definitions of "this change
// is good" to each other.
//
// `.tp/config.json`'s `workflow.quality_gate` is what `tp done` runs before it
// will close a task, so every change that goes through the loop passes all of
// it. GitHub Actions is what a direct push or a pull request passes. When the
// two drift, the weaker one silently becomes the real gate for anything that
// does not arrive through tp — and the drift is invisible, because both are
// green.
//
// It had drifted: the gate ran `go test -race`, CI ran `go test`; the gate ran
// the deadcode script, CI ran nothing of the sort. A race the gate would catch
// and an unreachable exported symbol it would report could both reach main
// through a pull request. Found by a measurement pass over this repository, not
// by either check failing.
//
// The assertion is per-command rather than string equality on purpose: CI
// legitimately adds installation steps around the commands, and pinning the
// whole file would make every unrelated workflow edit a test failure.
func TestCIRunsEveryStepOfTheProjectGate(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, ".tp", "config.json"))
	require.NoError(t, err)
	var project struct {
		Workflow struct {
			QualityGate string `json:"quality_gate"`
		} `json:"workflow"`
	}
	require.NoError(t, json.Unmarshal(raw, &project))
	gate := project.Workflow.QualityGate
	require.NotEmpty(t, gate, ".tp/config.json must carry the project gate")

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	require.NoError(t, err)

	// The search space is ci.yml's EXECUTABLE lines, never its whole text, and
	// that distinction was measured rather than reasoned. This assertion used to
	// run Contains over the file body, and audit round 12 built two inputs it
	// could not see: deleting the deadcode step while leaving
	// `# TODO: re-enable ./scripts/check-deadcode.sh` behind, and replacing
	// `run: ./scripts/check-suite-state.sh` with `run: go test ./...` plus a
	// comment naming the wrapper. Both passed. The second one left CI running
	// neither the race detector nor the state check with all five gate guards
	// green -- a comment is not a step, and a guard that cannot tell them apart
	// certifies the comment.
	executable := ciRunLines(string(workflow))
	require.NotEmpty(t, executable, "ci.yml must still have run: steps for this guard to have a subject")

	for _, step := range strings.Split(gate, "&&") {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		assert.Contains(t, executable, step,
			"the project gate runs %q on every tp done; CI must run it too, or a direct "+
				"push passes a weaker gate than a task close does", step)
	}
}

// ciRunLines returns only what a GitHub workflow actually executes: the value of
// every `run:` key plus the continuation lines of its block scalars, joined by
// newlines. Comments are dropped, because a step named only in a comment is a
// step CI does not run.
func ciRunLines(workflow string) string {
	var out []string
	inBlock := false
	blockIndent := 0
	for _, line := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if inBlock {
			if trimmed == "" {
				continue
			}
			if indent > blockIndent {
				// A commented-out line inside a block scalar is not a step, and
				// this is where round 12's third input lived: the deadcode step
				// replaced by `# TODO: re-enable ./scripts/check-deadcode.sh`,
				// still indented under its own run: |, still matched by a guard
				// that only skipped comments OUTSIDE blocks. CI ran no deadcode
				// and every gate guard stayed green.
				if !strings.HasPrefix(trimmed, "#") {
					out = append(out, stripShellComment(trimmed))
				}
				continue
			}
			inBlock = false
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		_, value, found := strings.Cut(trimmed, "run:")
		if !found || !strings.HasPrefix(trimmed, "run:") && !strings.HasPrefix(trimmed, "- run:") {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "|" || value == ">" || value == "|-" || value == ">-" {
			inBlock, blockIndent = true, indent
			continue
		}
		out = append(out, stripShellComment(value))
	}
	return strings.Join(out, "\n")
}

// stripShellComment drops a trailing `#` comment from a run line. Measured in
// audit round 12's replay: `run: go test ./...   # runs ./scripts/check-suite-state.sh`
// put the wrapper's name inside the run VALUE, so narrowing the search to run:
// lines was not on its own enough -- the comment rode along as executable text
// and the guard certified a step CI does not run. Erring strict is correct
// here: a step hidden behind a `#` is a step that does not execute.
func stripShellComment(line string) string {
	if i := strings.Index(line, " #"); i >= 0 {
		return strings.TrimSpace(line[:i])
	}
	return line
}
