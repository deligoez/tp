package engine

import (
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// DefaultWorkflow returns the resolution-chain defaults: 2 clean rounds,
// no round caps, 600s gate timeout, no checks.
func DefaultWorkflow() model.Workflow {
	return model.Workflow{
		ReviewCleanRounds:  2,
		AuditCleanRounds:   2,
		GateTimeoutSeconds: 600,
		LockTimeoutSeconds: 5,
		Checks:             []model.Check{},
	}
}

// ResolveWorkflow resolves workflow parameters for the spec under review or
// audit. The task file is resolved via the standard discovery chain (--file >
// TP_FILE > .tp/local.json active > auto-detect); when the discovered file's spec field
// does not resolve to specPath, or no file is found, the spec-adjacent
// <spec-base>.tasks.json is used; when neither exists, defaults apply.
// Returns the workflow and the task file path it came from ("" for defaults).
func ResolveWorkflow(specPath, explicitFile string) (wf model.Workflow, source string) {
	// The project config (discovered once from the working directory) is layered
	// under the task file's own sparse workflow override, so a thinned task file
	// inherits project policy. With no .tp/ this reduces to the v0.23.0 behavior.
	project := projectWorkflowOverride(".")

	if tfPath, err := DiscoverTaskFile(".", explicitFile); err == nil {
		if tf, readErr := model.ReadTaskFile(tfPath); readErr == nil && specMatches(tfPath, tf.Spec, specPath) {
			override, _ := LoadTaskWorkflowOverride(tfPath)
			return ResolveWorkflowLayers(override, project), tfPath
		}
	}

	adjacent := SpecAdjacentTaskFile(specPath)
	if _, err := model.ReadTaskFile(adjacent); err == nil {
		override, _ := LoadTaskWorkflowOverride(adjacent)
		return ResolveWorkflowLayers(override, project), adjacent
	}

	return ResolveWorkflowLayers(model.WorkflowOverride{}, project), ""
}

// specMatches reports whether a task file's spec field resolves to the spec
// under review: the spec field is cleaned and made absolute against the task
// file's directory, the CLI argument against the CWD, then compared for equality.
func specMatches(taskFilePath, specField, specArg string) bool {
	if specField == "" {
		return false
	}
	fieldPath := specField
	if !filepath.IsAbs(fieldPath) {
		fieldPath = filepath.Join(filepath.Dir(taskFilePath), specField)
	}
	fieldAbs, err := filepath.Abs(filepath.Clean(fieldPath))
	if err != nil {
		return false
	}
	argAbs, err := filepath.Abs(filepath.Clean(specArg))
	if err != nil {
		return false
	}
	return fieldAbs == argAbs
}

// SpecAdjacentTaskFile derives the <spec-base>.tasks.json path next to a spec.
func SpecAdjacentTaskFile(specPath string) string {
	return strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
}
