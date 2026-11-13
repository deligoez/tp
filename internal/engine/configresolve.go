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
// .tp/local.json discovered from the working directory, or nil when there is no
// .tp/, no local.json, or no defaults.
func LocalFlagDefaults() map[string]bool {
	tpDir := DiscoverTPDir(".")
	if tpDir == "" {
		return nil
	}
	lc, _, err := LoadLocalConfig(tpDir)
	if err != nil {
		return nil
	}
	return lc.Defaults
}

// ProjectWorkflowOverride returns the workflow override of the project config
// discovered from start, or an empty override when no .tp/ or config exists or
// the config is unreadable. start is the task file's directory wherever a task
// file is in play, since the project layer belongs to the task file
// (spec/backlog/project-layer-from-task-file.md); "." only where none is.
//
// It stays best-effort: an unreadable config is refused once, before a
// command that uses the layer does its work (ProjectConfigError, which the CLI
// asks for every such command), so the readers below it need not each repeat
// the refusal. Exported so tp config --resolved can attribute each field to
// the project layer.
func ProjectWorkflowOverride(start string) model.WorkflowOverride {
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

// pickFloat returns the first non-nil layer value in precedence order, or def.
func pickFloat(layers []*float64, def float64) float64 {
	for _, p := range layers {
		if p != nil {
			return *p
		}
	}
	return def
}

// pickRaw returns the first present raw-JSON layer in precedence order, or def.
// A nil RawMessage is absent, which is how the runner field tracks presence.
func pickRaw(layers []json.RawMessage, def json.RawMessage) json.RawMessage {
	for _, p := range layers {
		if p != nil {
			return p
		}
	}
	return def
}

// EffectiveWorkflowForTaskFile resolves the effective workflow for a task file:
// the project config discovered from the task file's own directory layered
// under the task file's presence-tracked workflow override. Best-effort — a
// missing or unreadable config or task file contributes no overrides — so a
// task file that omits quality_gate runs its project's quality_gate.
func EffectiveWorkflowForTaskFile(taskFilePath string) model.Workflow {
	override, _ := LoadTaskWorkflowOverride(taskFilePath)
	clampWorkflowRanges(&override)
	start := filepath.Dir(taskFilePath)
	project := ProjectWorkflowOverride(start)
	wf := ResolveWorkflowLayers(&override, &project)
	wf.NotifyCmd = LocalNotifyCmd(start)
	return wf
}

// ProjectConfigError reports a project config, discovered from start, that
// exists but cannot be read or parsed — nil when there is none or it loads.
// A command that uses the project layer refuses on it rather than running as
// if there were no project config (spec/backlog/project-layer-from-task-file.md).
func ProjectConfigError(start string) error {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return nil
	}
	_, _, err := LoadProjectConfig(tpDir)
	return err
}

// SameProjectRoot reports whether two directories share a project root
// (ProjectRoot), compared after symlink resolution, so /tmp and /private/tmp
// name one root.
func SameProjectRoot(a, b string) bool {
	return canonicalDir(ProjectRoot(a)) == canonicalDir(ProjectRoot(b))
}

// canonicalDir is a directory's absolute, symlink-resolved form, or the best
// form available when it cannot be resolved.
func canonicalDir(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return dir
}

// CanonicalPath is a file's path with its directory in canonicalDir's form, so
// a path printed beside a CanonicalProjectRoot shares its spelling.
func CanonicalPath(path string) string {
	return filepath.Join(canonicalDir(filepath.Dir(path)), filepath.Base(path))
}

// CanonicalProjectRoot is ProjectRoot(start) in canonicalDir's form, the
// spelling a refusal names.
func CanonicalProjectRoot(start string) string {
	return canonicalDir(ProjectRoot(start))
}

// LocalNotifyCmd returns the notify_cmd recorded in the .tp/local.json
// discovered from start, or "" when there is no .tp/, no local.json, or no
// notify_cmd. §7 makes this field per-operator rather than per-project, so
// local.json is the only layer that can supply it — a task file or
// .tp/config.json carrying notify_cmd reports an unknown key instead.
func LocalNotifyCmd(start string) string {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return ""
	}
	lc, _, err := LoadLocalConfig(tpDir)
	if err != nil || lc.NotifyCmd == nil {
		return ""
	}
	return *lc.NotifyCmd
}

// ResolveEffectiveWorkflow resolves the effective workflow for a start
// directory: it discovers the project config from start, loads its workflow
// defaults, and layers the given task-file override over them (a per-field
// sparse merge, so a task file that sets only one field inherits the rest from
// the project). With no .tp/ present, the override resolves over the built-in
// defaults exactly as in v0.23.0. Returns the effective workflow and any config
// validation warnings.
// ResolveEffectiveWorkflow resolves the effective workflow for a start
// directory: it discovers the project config from start, loads its workflow
// defaults, and layers the given task-file override over them (a per-field
// sparse merge, so a task file that sets only one field inherits the rest from
// the project). With no .tp/ present, the override resolves over the built-in
// defaults exactly as in v0.23.0. Returns the effective workflow and any config
// validation warnings.
func ResolveEffectiveWorkflow(start string, taskOverride *model.WorkflowOverride) (model.Workflow, []string, error) {
	// Clamp a local copy so an out-of-range field is dropped to absent — so it
	// resolves through the precedence rather than acting as an override
	// (§3.5/§7.1) — without ever mutating the caller's override.
	local := *taskOverride
	taskWarnings := clampWorkflowRanges(&local)
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		empty := model.WorkflowOverride{}
		return ResolveWorkflowLayers(&local, &empty), taskWarnings, nil
	}
	pc, warnings, err := LoadProjectConfig(tpDir)
	if err != nil {
		return model.Workflow{}, warnings, err
	}
	wf := ResolveWorkflowLayers(&local, &pc.Workflow)
	wf.NotifyCmd = LocalNotifyCmd(start)
	return wf, append(taskWarnings, warnings...), nil
}

// ResolveWorkflowLayers merges workflow overrides by precedence: the task-file
// override outranks the project config, which outranks the built-in default.
// Each field resolves independently — a nil field inherits the next lower layer
// — so presence, not value, defines an override.
// ResolveWorkflowLayers merges workflow overrides by precedence: the task-file
// override outranks the project config, which outranks the built-in default.
// Each field resolves independently — a nil field inherits the next lower layer
// — so presence, not value, defines an override.
//
// notify_cmd is not resolved here: it is per-operator and comes from
// .tp/local.json only (§7), which the layers this function sees do not carry.
func ResolveWorkflowLayers(taskOverride, project *model.WorkflowOverride) model.Workflow {
	// The built-in default layer: 2 clean rounds, a 3-round cap on each loop,
	// 600s gate timeout, 5s lock timeout, no checks, blocking-severity review
	// convergence, every-row audit convergence, and §7's run caps and runner.
	// The two convergence defaults differ deliberately: v0.37.0 §2.1 measures
	// why. The caps are finite because unoracled review stops paying for itself
	// within a few rounds; an explicit 0 at any layer still means uncapped.
	def := model.Workflow{
		ReviewCleanRounds:      2,
		AuditCleanRounds:       2,
		ReviewMaxRounds:        DefaultMaxRounds,
		AuditMaxRounds:         DefaultMaxRounds,
		GateTimeoutSeconds:     600,
		LockTimeoutSeconds:     5,
		Checks:                 []model.Check{},
		ReviewConvergeOn:       ReviewConvergeOnBlocking,
		AuditConvergeOn:        AuditConvergeOnAll,
		RunMaxUnits:            RunMaxUnitsDefault,
		RunMaxWallClockSeconds: RunMaxWallClockSecondsDefault,
		RunMaxBudgetUSD:        RunMaxBudgetUSDDefault,
		RunMaxUnitBudgetUSD:    RunMaxUnitBudgetUSDDefault,
		RunMaxUnitRetries:      RunMaxUnitRetriesDefault,
		RunMaxParallel:         RunMaxParallelDefault,
		Runner:                 DefaultRunner(),
	}
	return model.Workflow{
		QualityGate:        pickString([]*string{taskOverride.QualityGate, project.QualityGate}, def.QualityGate),
		CommitStrategy:     pickString([]*string{taskOverride.CommitStrategy, project.CommitStrategy}, def.CommitStrategy),
		GateTimeoutSeconds: pickInt([]*int{taskOverride.GateTimeoutSeconds, project.GateTimeoutSeconds}, def.GateTimeoutSeconds),
		LockTimeoutSeconds: pickInt([]*int{taskOverride.LockTimeoutSeconds, project.LockTimeoutSeconds}, def.LockTimeoutSeconds),
		ReviewCleanRounds:  pickInt([]*int{taskOverride.ReviewCleanRounds, project.ReviewCleanRounds}, def.ReviewCleanRounds),
		AuditCleanRounds:   pickInt([]*int{taskOverride.AuditCleanRounds, project.AuditCleanRounds}, def.AuditCleanRounds),
		ReviewMaxRounds:    pickInt([]*int{taskOverride.ReviewMaxRounds, project.ReviewMaxRounds}, def.ReviewMaxRounds),
		AuditMaxRounds:     pickInt([]*int{taskOverride.AuditMaxRounds, project.AuditMaxRounds}, def.AuditMaxRounds),
		Checks:             pickChecks([]*[]model.Check{taskOverride.Checks, project.Checks}, def.Checks),
		ReviewConvergeOn:   pickString([]*string{taskOverride.ReviewConvergeOn, project.ReviewConvergeOn}, def.ReviewConvergeOn),
		AuditConvergeOn:    pickString([]*string{taskOverride.AuditConvergeOn, project.AuditConvergeOn}, def.AuditConvergeOn),

		RunMaxUnits:            pickInt([]*int{taskOverride.RunMaxUnits, project.RunMaxUnits}, def.RunMaxUnits),
		RunMaxWallClockSeconds: pickInt([]*int{taskOverride.RunMaxWallClockSeconds, project.RunMaxWallClockSeconds}, def.RunMaxWallClockSeconds),
		RunMaxBudgetUSD:        pickFloat([]*float64{taskOverride.RunMaxBudgetUSD, project.RunMaxBudgetUSD}, def.RunMaxBudgetUSD),
		RunMaxUnitBudgetUSD:    pickFloat([]*float64{taskOverride.RunMaxUnitBudgetUSD, project.RunMaxUnitBudgetUSD}, def.RunMaxUnitBudgetUSD),
		RunMaxUnitRetries:      pickInt([]*int{taskOverride.RunMaxUnitRetries, project.RunMaxUnitRetries}, def.RunMaxUnitRetries),
		RunMaxParallel:         pickInt([]*int{taskOverride.RunMaxParallel, project.RunMaxParallel}, def.RunMaxParallel),
		Runner:                 pickRaw([]json.RawMessage{taskOverride.Runner, project.Runner}, def.Runner),
	}
}
