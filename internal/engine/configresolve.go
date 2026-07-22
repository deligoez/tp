package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// ResolveLocalActive returns the active task-file path recorded in
// .tp/local.json, resolved against the project root. It returns "" when there
// is no .tp/, no active key, or the stored value is absolute or escapes the
// project root (those are rejected). File existence is not checked here — the
// discovery chain decides whether to use or fall past the resolved path.
func ResolveLocalActive(start string) string {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return ""
	}
	lc, _, err := LoadLocalConfig(tpDir)
	if err != nil || lc.Active == nil {
		return ""
	}
	active := *lc.Active
	if active == "" {
		return ""
	}
	if filepath.IsAbs(active) {
		fmt.Fprintf(os.Stderr, "warning: .tp/local.json active %q is absolute; treating as unset\n", active)
		return ""
	}
	root := ProjectRoot(start)
	resolved := filepath.Join(root, active)
	if rel, relErr := filepath.Rel(root, resolved); relErr != nil || strings.HasPrefix(rel, "..") {
		fmt.Fprintf(os.Stderr, "warning: .tp/local.json active %q escapes the project root; treating as unset\n", active)
		return ""
	}
	return resolved
}

// LoadTaskWorkflowOverride parses a task file's own "workflow" block into a
// presence-tracked WorkflowOverride, so a sparse task-file workflow layers
// correctly over the project config. An absent block yields an empty override.
func LoadTaskWorkflowOverride(taskFilePath string) (model.WorkflowOverride, error) {
	data, err := os.ReadFile(taskFilePath)
	if err != nil {
		return model.WorkflowOverride{}, err
	}
	var top struct {
		Workflow json.RawMessage `json:"workflow"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return model.WorkflowOverride{}, err
	}
	if len(top.Workflow) == 0 {
		return model.WorkflowOverride{}, nil
	}
	wo, _ := parseWorkflowOverride(top.Workflow)
	return wo, nil
}

// LocalFlagDefaults returns the boolean flag defaults recorded in
// .tp/local.json discovered from start, or nil when there is no .tp/, no
// local.json, or no defaults.
func LocalFlagDefaults(start string) map[string]bool {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return nil
	}
	lc, _, err := LoadLocalConfig(tpDir)
	if err != nil {
		return nil
	}
	return lc.Defaults
}

// ProjectWorkflowOverride returns the project config's workflow override
// discovered from start — exported so tp config --resolved can attribute each
// field to the project layer.
func ProjectWorkflowOverride(start string) model.WorkflowOverride {
	return projectWorkflowOverride(start)
}

// projectWorkflowOverride returns the project config's workflow override
// discovered from start, or an empty override when no .tp/ or config exists or
// the config is unreadable (best-effort; commands that must abort on a
// malformed config call LoadProjectConfig directly).
func projectWorkflowOverride(start string) model.WorkflowOverride {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return model.WorkflowOverride{}
	}
	pc, _, err := LoadProjectConfig(tpDir)
	if err != nil {
		return model.WorkflowOverride{}
	}
	return pc.Workflow
}

// pickInt returns the first non-nil layer value in precedence order (highest
// layer first), or def when every layer is absent.
func pickInt(layers []*int, def int) int {
	for _, p := range layers {
		if p != nil {
			return *p
		}
	}
	return def
}

// pickString returns the first non-nil layer value in precedence order, or def.
func pickString(layers []*string, def string) string {
	for _, p := range layers {
		if p != nil {
			return *p
		}
	}
	return def
}

// pickChecks returns the first present checks layer in precedence order — replace
// semantics, so a present slice (even empty) wins over lower layers — or def.
func pickChecks(layers []*[]model.Check, def []model.Check) []model.Check {
	for _, p := range layers {
		if p != nil {
			return *p
		}
	}
	return def
}

// EffectiveWorkflowForTaskFile resolves the effective workflow for a task file:
// the project config discovered from the working directory layered under the
// task file's own presence-tracked workflow override. Best-effort — a missing
// or unreadable config or task file contributes no overrides — so a task file
// that omits quality_gate runs the project quality_gate.
func EffectiveWorkflowForTaskFile(taskFilePath string) model.Workflow {
	override, _ := LoadTaskWorkflowOverride(taskFilePath)
	return ResolveWorkflowLayers(override, projectWorkflowOverride("."))
}

// ResolveEffectiveWorkflow resolves the effective workflow for a start
// directory: it discovers the project config from start, loads its workflow
// defaults, and layers the given task-file override over them (a per-field
// sparse merge, so a task file that sets only one field inherits the rest from
// the project). With no .tp/ present, the override resolves over the built-in
// defaults exactly as in v0.23.0. Returns the effective workflow and any config
// validation warnings.
func ResolveEffectiveWorkflow(start string, taskOverride model.WorkflowOverride) (model.Workflow, []string, error) {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return ResolveWorkflowLayers(taskOverride, model.WorkflowOverride{}), nil, nil
	}
	pc, warnings, err := LoadProjectConfig(tpDir)
	if err != nil {
		return model.Workflow{}, warnings, err
	}
	return ResolveWorkflowLayers(taskOverride, pc.Workflow), warnings, nil
}

// ResolveWorkflowLayers merges workflow overrides by precedence: the task-file
// override outranks the project config, which outranks the built-in default.
// Each field resolves independently — a nil field inherits the next lower layer
// — so presence, not value, defines an override.
func ResolveWorkflowLayers(taskOverride, project model.WorkflowOverride) model.Workflow {
	def := DefaultWorkflow()
	return model.Workflow{
		QualityGate:        pickString([]*string{taskOverride.QualityGate, project.QualityGate}, def.QualityGate),
		CommitStrategy:     pickString([]*string{taskOverride.CommitStrategy, project.CommitStrategy}, def.CommitStrategy),
		GateTimeoutSeconds: pickInt([]*int{taskOverride.GateTimeoutSeconds, project.GateTimeoutSeconds}, def.GateTimeoutSeconds),
		ReviewCleanRounds:  pickInt([]*int{taskOverride.ReviewCleanRounds, project.ReviewCleanRounds}, def.ReviewCleanRounds),
		AuditCleanRounds:   pickInt([]*int{taskOverride.AuditCleanRounds, project.AuditCleanRounds}, def.AuditCleanRounds),
		ReviewMaxRounds:    pickInt([]*int{taskOverride.ReviewMaxRounds, project.ReviewMaxRounds}, def.ReviewMaxRounds),
		AuditMaxRounds:     pickInt([]*int{taskOverride.AuditMaxRounds, project.AuditMaxRounds}, def.AuditMaxRounds),
		Checks:             pickChecks([]*[]model.Check{taskOverride.Checks, project.Checks}, def.Checks),
	}
}
