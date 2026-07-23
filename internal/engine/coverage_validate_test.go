package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// §7.2: when the spec exists but cannot be read/parsed, ValidateCoverage's
// coverage finding carries a hint naming the unreadable spec path.
func TestValidateCoverage_UnreadableSpec_HintsPath(t *testing.T) {
	// A directory is unreadable as a spec (os.ReadFile errors), exercising the
	// "could not parse spec" path without depending on filesystem permissions.
	specPath := t.TempDir()
	tf := &model.TaskFile{
		Version: 1,
		Spec:    filepath.Base(specPath),
		Tasks: []model.Task{
			{ID: "t1", Title: "T", Status: "open", Acceptance: "done", EstimateMinutes: 5, SourceSections: []string{"## 1. Setup"}},
		},
	}
	findings := ValidateCoverage(tf, specPath)
	require.Len(t, findings, 1)
	assert.Equal(t, "coverage", findings[0].Rule)
	assert.Contains(t, findings[0].Message, "could not parse spec")
	assert.Equal(t, specPath, findings[0].Hint, "hint must name the unreadable spec path")
}
