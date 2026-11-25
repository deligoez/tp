package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// foundTask is a task a found request opened: origin in place of
// source_sections (spec/1.31.0.md §3).
func foundTask() model.Task {
	task := entrySchemaTask()
	task.ID = "f1"
	task.SourceSections = nil
	task.Origin = &model.Origin{FinderTask: "t1", Location: "internal/a.go:40"}
	return task
}

// TestFoundTaskPassesValidateAndEntry: origin is an anchor, so a task with
// origin and no source_sections passes tp validate and the entry check.
func TestFoundTaskPassesValidateAndEntry(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n\n## A\n\nText.\n"), 0o600))

	for name, task := range map[string]model.Task{
		"task finder": foundTask(),
		"audit finder": func() model.Task {
			f := foundTask()
			f.Origin = &model.Origin{AuditRound: 2, AuditRow: "go-safety:file-a", Location: "internal/a.go:40"}
			return f
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, TaskSchemaErrors(&task))
			assert.Nil(t, ValidateTaskEntry(&task, map[string]bool{}))

			tf := &model.TaskFile{Version: 1, Spec: "s.md", Tasks: []model.Task{entrySchemaTask(), task}}
			tf.Tasks[0].SourceSections = []string{"## A"}
			AutoFillCoverage(tf, spec)
			res := Validate(tf, spec, true, false)
			for _, f := range res.Findings {
				assert.NotEqual(t, "error", f.Severity, "tp validate rejected a found task: %s", f.Message)
			}
		})
	}
}

// TestOriginAndSourceSectionsAreExclusive: a task carrying both fails
// validation, and the message names both fields.
func TestOriginAndSourceSectionsAreExclusive(t *testing.T) {
	task := foundTask()
	task.SourceSections = []string{"## A"}

	msgs := TaskSchemaErrors(&task)
	require.NotEmpty(t, msgs)
	joined := strings.Join(msgs, "; ")
	assert.Contains(t, joined, "origin")
	assert.Contains(t, joined, "source_sections")

	f := ValidateTaskEntry(&task, map[string]bool{})
	require.NotNil(t, f)
	assert.Contains(t, f.Msg, "origin")
	assert.Contains(t, f.Msg, "source_sections")
}

// TestOriginWithoutLocationIsRefused: an origin is an anchor only when it
// names the request's location (spec/1.31.0.md §3), so `"origin":{}` fails
// tp validate and the entry check instead of passing as an anchor and
// dropping the task out of spec coverage.
func TestOriginWithoutLocationIsRefused(t *testing.T) {
	for name, origin := range map[string]*model.Origin{
		"empty":       {},
		"blank":       {FinderTask: "t1", Location: "  "},
		"finder only": {AuditRound: 2, AuditRow: "go-safety:file-a"},
	} {
		t.Run(name, func(t *testing.T) {
			task := foundTask()
			task.Origin = origin

			msgs := TaskSchemaErrors(&task)
			require.NotEmpty(t, msgs)
			assert.Contains(t, strings.Join(msgs, "; "), "origin needs a location")

			f := ValidateTaskEntry(&task, map[string]bool{})
			require.NotNil(t, f)
			assert.Contains(t, f.Msg, "origin needs a location")
		})
	}
}
