package engine

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// taskAuditUnitsRepo is a spec whose one done task closed over n files and
// owes the security role, the only role task_audit_roles lists.
func taskAuditUnitsRepo(t *testing.T, n int) (root, taskFile, spec string, tf *model.TaskFile) {
	t.Helper()
	fs := make([]string, n)
	for i := range fs {
		fs[i] = fmt.Sprintf("f%02d.go", i)
	}
	spec = presenceSpec(t, []string{"security"}, closedTask("t1", fs, "security"))
	root = filepath.Dir(spec)
	taskFile = filepath.Join(root, "spec.tasks.json")
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	return root, taskFile, spec, tf
}

// TestTaskAuditUnits_AOneRoleBatchRunsBeforeImplement: a full batch owing one
// role is one task-audit unit, and it runs before any implement unit.
func TestTaskAuditUnits_AOneRoleBatchRunsBeforeImplement(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, tf := taskAuditUnitsRepo(t, TaskAuditBatchFiles)
	units, round := BuildNextUnits(root, taskFile, spec, PhaseImplement, tf, nil, nil)
	assert.Equal(t, []UnitKind{UnitTaskAuditRole}, unitKinds(units))
	assert.Equal(t, []string{"security"}, unitIDs(units))
	require.NotNil(t, round)
	assert.Equal(t, 1, *round)
}
