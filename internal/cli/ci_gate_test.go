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
	ci := string(workflow)

	for _, step := range strings.Split(gate, "&&") {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		assert.Contains(t, ci, step,
			"the project gate runs %q on every tp done; CI must run it too, or a direct "+
				"push passes a weaker gate than a task close does", step)
	}
}
