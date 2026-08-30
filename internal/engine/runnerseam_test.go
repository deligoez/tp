package engine_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// seamArgs is §3.2.1's documented positional argument list for the seam runner,
// written out literally rather than derived from the implementation, so a change
// to the shipped list has to be made twice on purpose.
var seamArgs = []string{"{unit_kind}", "{unit_id}", "{log_path}", "{max_budget_usd}"}

// layeredRunner stands for whatever §7's precedence resolved before the seam is
// considered — a runner object of the kind a CLI flag supplies, which is the
// highest ordinary layer and therefore the one the seam has to outrank.
const layeredRunner = `{"cmd":"cli-runner","args":["--from-cli"]}`

// The seam pins a runner whose cmd is the variable's value, whose args are
// §3.2.1's positional list and whose spend_key is the same key the claude
// template declares — which is what makes the spend and budget-cap paths
// testable without an agent.
func TestSeamRunner_Shape(t *testing.T) {
	t.Setenv(engine.EnvRunnerSeam, "/tmp/tp-fake-runner")

	runner, ok := engine.SeamRunner()
	require.True(t, ok, "a set TP_RUNNER_SEAM is a seam")
	assert.Equal(t, "/tmp/tp-fake-runner", runner.Cmd, "the variable's value is the cmd")
	assert.Equal(t, seamArgs, runner.Args, "§3.2.1's positional list, byte for byte")
	assert.Equal(t, "total_cost_usd", runner.SpendKey)
}

// Absent, and present but blank, are both "no seam": a runner with nothing to
// spawn is not a runner, and an exported variable left empty by a shell is the
// ordinary way that happens.
func TestSeamRunner_AbsentOrBlankIsNoSeam(t *testing.T) {
	t.Setenv(engine.EnvRunnerSeam, "unused")

	require.NoError(t, os.Unsetenv(engine.EnvRunnerSeam))
	runner, ok := engine.SeamRunner()
	assert.False(t, ok, "an unset variable pins nothing")
	assert.Nil(t, runner)

	t.Setenv(engine.EnvRunnerSeam, "   ")
	runner, ok = engine.SeamRunner()
	assert.False(t, ok, "a blank value has no cmd to spawn")
	assert.Nil(t, runner)
}

// The precedence claim: the seam outranks every layer of §7, including a CLI
// flag. Both arms are measured, because an assertion that only sets the seam
// passes whether or not the precedence exists at all — the control arm proves
// the layered value is really what wins when the seam is absent.
func TestResolveUnitRunner_SeamOutranksEveryLayer(t *testing.T) {
	v := values(0)

	t.Run("without the seam the layered value wins", func(t *testing.T) {
		runner, err := engine.ResolveUnitRunner(json.RawMessage(layeredRunner), v)
		require.NoError(t, err)
		assert.Equal(t, "cli-runner", runner.Cmd)
		assert.Equal(t, []string{"--from-cli"}, runner.Args)
	})

	t.Run("with the seam the seam wins", func(t *testing.T) {
		t.Setenv(engine.EnvRunnerSeam, "/tmp/tp-fake-runner")

		runner, err := engine.ResolveUnitRunner(json.RawMessage(layeredRunner), v)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/tp-fake-runner", runner.Cmd)
		assert.Equal(t, "total_cost_usd", runner.SpendKey)
		assert.Equal(t, []string{"implement", "runner-templates", v.LogPath, "0"}, runner.Args,
			"the seam's placeholders expand like any other template's, and a 0 budget resolves to a literal 0")
	})
}

// A runner value that is a usage error is still outranked: the seam is read
// before the field is resolved at all, so a test can pin the runner whatever
// the repo's config says — including a config the resolver would reject.
func TestResolveUnitRunner_SeamOutranksAnUnusableValue(t *testing.T) {
	// A per-kind map with no default, and no entry for the kind being spawned.
	broken := json.RawMessage(`{"audit-role":"claude"}`)

	_, err := engine.ResolveUnitRunner(broken, values(0))
	require.Error(t, err, "control: without the seam this value is a usage error")

	t.Setenv(engine.EnvRunnerSeam, "/tmp/tp-fake-runner")
	runner, err := engine.ResolveUnitRunner(broken, values(0))
	require.NoError(t, err)
	assert.Equal(t, "/tmp/tp-fake-runner", runner.Cmd)
}

// The seam is a runner like any other, so {max_budget_usd} carries the resolved
// budget rather than the flag pair the claude template drops at 0.
func TestResolveUnitRunner_SeamCarriesTheBudgetPositionally(t *testing.T) {
	t.Setenv(engine.EnvRunnerSeam, "/tmp/tp-fake-runner")

	v := values(2.5)
	runner, err := engine.ResolveUnitRunner(engine.DefaultRunner(), v)
	require.NoError(t, err)
	assert.Equal(t, []string{"implement", "runner-templates", v.LogPath, "2.5"}, runner.Args)
}
