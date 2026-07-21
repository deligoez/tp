package engine

import "github.com/deligoez/tp/internal/model"

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

// ResolveWorkflowLayers merges workflow overrides by precedence: the task-file
// override outranks the project config, which outranks the built-in default.
// Each field resolves independently — a nil field inherits the next lower layer
// — so presence, not value, defines an override.
func ResolveWorkflowLayers(taskOverride, project model.WorkflowOverride) model.Workflow {
	def := DefaultWorkflow()
	return model.Workflow{
		QualityGate:        pickString([]*string{taskOverride.QualityGate, project.QualityGate}, def.QualityGate),
		GateTimeoutSeconds: pickInt([]*int{taskOverride.GateTimeoutSeconds, project.GateTimeoutSeconds}, def.GateTimeoutSeconds),
		ReviewCleanRounds:  pickInt([]*int{taskOverride.ReviewCleanRounds, project.ReviewCleanRounds}, def.ReviewCleanRounds),
		AuditCleanRounds:   pickInt([]*int{taskOverride.AuditCleanRounds, project.AuditCleanRounds}, def.AuditCleanRounds),
		ReviewMaxRounds:    pickInt([]*int{taskOverride.ReviewMaxRounds, project.ReviewMaxRounds}, def.ReviewMaxRounds),
		AuditMaxRounds:     pickInt([]*int{taskOverride.AuditMaxRounds, project.AuditMaxRounds}, def.AuditMaxRounds),
		Checks:             pickChecks([]*[]model.Check{taskOverride.Checks, project.Checks}, def.Checks),
	}
}
