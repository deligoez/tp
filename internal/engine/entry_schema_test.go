package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// entrySchemaTask is a task every per-task check of tp validate passes.
func entrySchemaTask() model.Task {
	return model.Task{
		ID:              "t1",
		Title:           "do one thing",
		Status:          model.StatusOpen,
		Acceptance:      "it is done",
		EstimateMinutes: 5,
		SourceSections:  []string{"A"},
	}
}

// TestEntryRefusesEveryPerTaskSchemaError pins write-time parity: a task tp
// validate reports a per-task schema error for is refused by the entry check
// tp add, tp add --bulk and tp import share, so no write leaves a file the
// next tp validate rejects.
func TestEntryRefusesEveryPerTaskSchemaError(t *testing.T) {
	cases := map[string]func(*model.Task){
		"estimate missing":   func(t *model.Task) { t.EstimateMinutes = 0 },
		"invalid status":     func(t *model.Task) { t.Status = "bogus" },
		"touches mid **":     func(t *model.Task) { t.Touches = []string{"a/**/b"} },
		"title missing":      func(t *model.Task) { t.Title = "" },
		"acceptance missing": func(t *model.Task) { t.Acceptance = "" },
		"anchor missing":     func(t *model.Task) { t.SourceSections = nil },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			task := entrySchemaTask()
			breakIt(&task)

			tf := &model.TaskFile{Version: 1, Spec: "s.md", Tasks: []model.Task{task}}
			var schemaErrs []string
			for _, f := range validateSchema(tf) {
				if f.Severity == "error" {
					schemaErrs = append(schemaErrs, f.Message)
				}
			}
			require.NotEmpty(t, schemaErrs, "the fixture must break a tp validate schema check")

			f := ValidateTaskEntry(&task, map[string]bool{})
			require.NotNil(t, f, "entry accepted a task tp validate rejects: %s", strings.Join(schemaErrs, "; "))
		})
	}
}

// TestEntryAcceptsWhatValidateAccepts guards the other direction: the parity
// check must not refuse a task tp validate passes.
func TestEntryAcceptsWhatValidateAccepts(t *testing.T) {
	task := entrySchemaTask()
	tf := &model.TaskFile{Version: 1, Spec: "s.md", Tasks: []model.Task{task}}
	assert.Empty(t, validateSchema(tf))
	assert.Nil(t, ValidateTaskEntry(&task, map[string]bool{}))
}
