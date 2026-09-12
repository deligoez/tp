package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAssembleResume_TheDecomposeGateFollowsTheRegisteredChecks pins the wiring
// AssembleResume does, not the decision underneath it: it reads how many checks
// the resolved workflow registers and hands that to the decompose action. The
// engine's own tests called BuildNextAction directly, so nothing here exercised
// the read — a mutant that always reported "a check is registered", or inverted
// it, survived every engine test while the CLI's subprocess tests caught it.
func TestAssembleResume_TheDecomposeGateFollowsTheRegisteredChecks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		workflow string
		gated    bool
	}{
		{"a registered check gates the step", `{"checks":[{"class":"vague-number","cmd":"true"}]}`, true},
		{"no registered check leaves it alone", `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, spec, tfp := setupResumeProject(t, fmtTasks(tc.workflow))
			// The project layer discovers .tp/ from the process cwd, not from
			// the task file, so without this the resolved workflow carries
			// tp's own registered checks and the ungated case reads as gated.
			t.Chdir(dir)
			recordRounds(t, spec, 2, 0, true) // review converged
			res := assemble(t, dir, spec, tfp)
			require.Equal(t, PhaseDecompose, res.Phase)
			require.NotNil(t, res.NextAction)

			if !tc.gated {
				assert.Nil(t, res.NextAction.Command, "an ungated decompose names no command")
				assert.Nil(t, res.NextAction.BriefCommand)
				return
			}
			want := CheckGateCommand(spec)
			require.NotNil(t, res.NextAction.Command)
			assert.Equal(t, want, *res.NextAction.Command)
			require.NotNil(t, res.NextAction.BriefCommand, "a gated step briefs itself")
			assert.Equal(t, want, *res.NextAction.BriefCommand)
		})
	}
}

// fmtTasks fills the workflow block of the decompose-phase task file.
func fmtTasks(workflow string) string {
	return `{"spec":"s.md","workflow":` + workflow + `,"tasks":[]}`
}
