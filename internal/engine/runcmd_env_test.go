package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGateEnv_DropsEveryVariableTheDriverOwns: a quality gate run inside a
// tp run unit inherited TP_FILE and TP_UNATTENDED, so the suite under test
// resolved the run's own task file and wrote fixture values into it. The gate
// is the project's command, not the unit's, and sees none of the unit's
// identity.
func TestGateEnv_DropsEveryVariableTheDriverOwns(t *testing.T) {
	round := 2
	unit := ChildEnv([]string{"HOME=/h", "TP_SPEC=spec/x.md"}, nil, &UnitEnv{
		RunID: "R", Root: t.TempDir(), TaskFile: "spec/x.tasks.json", Phase: "implement",
		Round: &round, Kind: UnitImplement, ID: "a", Seq: 3,
	})
	unit = append(unit, EnvGateMainSlots+"=/s", EnvGateMainTimes+"=/t")

	got := gateEnv(unit)

	for _, kv := range got {
		name, _, _ := strings.Cut(kv, "=")
		assert.NotContains(t, RunEnvVars, name, "gate env kept %s", kv)
	}
	assert.Contains(t, got, "HOME=/h")
	assert.Contains(t, got, "TP_SPEC=spec/x.md", "TP_SPEC is the gate's own input, not the unit's identity")
}
