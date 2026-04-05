package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// helper to build a minimal valid TaskFile for cycle tests.
func taskFileWithDeps(tasks []model.Task) *model.TaskFile {
	return &model.TaskFile{
		Version: 1,
		Spec:    "test.md",
		Tasks:   tasks,
	}
}

func TestValidateCycles_Acyclic(t *testing.T) {
	tf := taskFileWithDeps([]model.Task{
		{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
		{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{"A"}},
		{ID: "C", Title: "C", Status: "open", Acceptance: "c", EstimateMinutes: 5, DependsOn: []string{"B"}},
	})
	findings := validateCycles(tf)
	assert.Empty(t, findings)
}

func TestValidateCycles_SelfDependency(t *testing.T) {
	tf := taskFileWithDeps([]model.Task{
		{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{"A"}},
	})
	findings := validateCycles(tf)
	require.NotEmpty(t, findings)
	assert.Equal(t, "error", findings[0].Severity)
	assert.Contains(t, findings[0].Message, "circular")
}

func TestValidateCycles_TwoNodeCycle(t *testing.T) {
	tf := taskFileWithDeps([]model.Task{
		{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{"B"}},
		{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{"A"}},
	})
	findings := validateCycles(tf)
	require.NotEmpty(t, findings)
	assert.Equal(t, "error", findings[0].Severity)
	assert.Contains(t, findings[0].Message, "circular")
}

func TestValidateCycles_ComplexDAG(t *testing.T) {
	// Diamond: A→B, A→C, B→D, C→D — no cycle
	tf := taskFileWithDeps([]model.Task{
		{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
		{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{"A"}},
		{ID: "C", Title: "C", Status: "open", Acceptance: "c", EstimateMinutes: 5, DependsOn: []string{"A"}},
		{ID: "D", Title: "D", Status: "open", Acceptance: "d", EstimateMinutes: 5, DependsOn: []string{"B", "C"}},
	})
	findings := validateCycles(tf)
	assert.Empty(t, findings)
}

func TestAtomicity_EstimateOutOfRange(t *testing.T) {
	tests := []struct {
		name    string
		minutes int
		warn    bool
	}{
		{"zero minutes", 0, true},
		{"over 15 minutes", 16, true},
		{"exactly 15 minutes", 15, false},
		{"within range", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tf := taskFileWithDeps([]model.Task{
				{ID: "T1", Title: "Task", Status: "open", Acceptance: "ok", EstimateMinutes: tt.minutes, DependsOn: []string{}},
			})
			findings := validateAtomicity(tf)
			if tt.warn {
				hasEstimateWarning := false
				for _, f := range findings {
					if strings.Contains(f.Message, "estimate_minutes") {
						hasEstimateWarning = true
						break
					}
				}
				assert.True(t, hasEstimateWarning, "expected estimate warning for %d minutes", tt.minutes)
			} else {
				for _, f := range findings {
					assert.NotContains(t, f.Message, "estimate_minutes")
				}
			}
		})
	}
}

func TestAtomicity_TitleWarnings(t *testing.T) {
	tests := []struct {
		name  string
		title string
		rule  string
		warn  bool
	}{
		{
			name:  "title with 9 words",
			title: "one two three four five six seven eight nine",
			rule:  "words",
			warn:  true,
		},
		{
			name:  "title with and conjunction",
			title: "setup and configure",
			rule:  "conjunction",
			warn:  true,
		},
		{
			name:  "short clean title",
			title: "Setup config",
			rule:  "",
			warn:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tf := taskFileWithDeps([]model.Task{
				{ID: "T1", Title: tt.title, Status: "open", Acceptance: "ok", EstimateMinutes: 5, DependsOn: []string{}},
			})
			findings := validateAtomicity(tf)
			if tt.warn {
				hasWarning := false
				for _, f := range findings {
					if strings.Contains(f.Message, tt.rule) {
						hasWarning = true
						break
					}
				}
				assert.True(t, hasWarning, "expected warning containing %q", tt.rule)
			}
		})
	}
}

func TestAtomicity_DescriptionTooLong(t *testing.T) {
	longDesc := strings.Repeat("x", 301)
	tf := taskFileWithDeps([]model.Task{
		{ID: "T1", Title: "Task", Status: "open", Acceptance: "ok", EstimateMinutes: 5, Description: longDesc, DependsOn: []string{}},
	})
	findings := validateAtomicity(tf)
	hasDescWarning := false
	for _, f := range findings {
		if strings.Contains(f.Message, "description") {
			hasDescWarning = true
			break
		}
	}
	assert.True(t, hasDescWarning)
}

func TestAtomicity_TooManyAcceptanceCriteria(t *testing.T) {
	tf := taskFileWithDeps([]model.Task{
		{ID: "T1", Title: "Task", Status: "open", Acceptance: "A. B. C. D.", EstimateMinutes: 5, DependsOn: []string{}},
	})
	findings := validateAtomicity(tf)
	hasAccWarning := false
	for _, f := range findings {
		if strings.Contains(f.Message, "acceptance") && strings.Contains(f.Message, "criteria") {
			hasAccWarning = true
			assert.Contains(t, f.Message, "hint: split into")
			assert.Contains(t, f.Message, "~2 tasks")
			break
		}
	}
	assert.True(t, hasAccWarning)
}

func TestAtomicity_SplitHintCount(t *testing.T) {
	// 10 criteria → ceil(10/3) = 4 suggested tasks
	acceptance := "A. B. C. D. E. F. G. H. I. J."
	tf := taskFileWithDeps([]model.Task{
		{ID: "T1", Title: "Task", Status: "open", Acceptance: acceptance, EstimateMinutes: 5, DependsOn: []string{}},
	})
	findings := validateAtomicity(tf)
	for _, f := range findings {
		if strings.Contains(f.Message, "criteria") {
			assert.Contains(t, f.Message, "~4 tasks")
			return
		}
	}
	t.Fatal("expected atomicity warning with split hint")
}
