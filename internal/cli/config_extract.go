package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// commonPtr returns the value every override sets identically for one pointer
// field, or nil when any override omits it or sets a different value. The
// overrides are addressed rather than copied: WorkflowOverride is wide enough
// that a per-element copy is a measurable waste.
func commonPtr[T comparable](overrides []model.WorkflowOverride, get func(*model.WorkflowOverride) *T) *T {
	first := get(&overrides[0])
	if first == nil {
		return nil
	}
	for i := 1; i < len(overrides); i++ {
		v := get(&overrides[i])
		if v == nil || *v != *first {
			return nil
		}
	}
	val := *first
	return &val
}

// computeCommonPolicy returns the workflow fields that EVERY override sets with
// an identical value — the only fields tp config --extract hoists.
//
// notify_cmd is never among them: §7 makes it per-operator, read from
// .tp/local.json only, so it is never a task-file override to hoist and never a
// project-config field to write.
func computeCommonPolicy(overrides []model.WorkflowOverride) model.WorkflowOverride {
	var common model.WorkflowOverride
	if len(overrides) == 0 {
		return common
	}
	common.GateTimeoutSeconds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.GateTimeoutSeconds })
	common.LockTimeoutSeconds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.LockTimeoutSeconds })
	common.ReviewCleanRounds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.ReviewCleanRounds })
	common.AuditCleanRounds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.AuditCleanRounds })
	common.ReviewMaxRounds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.ReviewMaxRounds })
	common.AuditMaxRounds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.AuditMaxRounds })
	// quality_gate is hoisted even though it is read-only at the task level
	// (readOnlyWorkflowFields, set.go) — being non-authorable by tp set does not
	// make a field unhoistable, because after the hoist it lands at the project
	// layer, where the project setter can author it.
	//
	// commit_strategy is in that same read-only set and is deliberately NOT
	// hoisted, and the reason is scheduling rather than technique. An audit
	// round recorded a different reason — that hoisting it would strip a field
	// tp keeps non-authorable — which this line refutes.
	//
	// Be exact about the fence, because half of it was lifted: v0.31.1's
	// Non-Goal 1 covered both the extract and the validate side, and v0.35.0
	// re-opened the validate half on the ground that reporting a deviation only
	// reports. The extract half stays closed because --extract *writes*, and
	// changing which fields a writing command hoists is a decision of its own.
	// spec/0.35.0-candidates.md section 15 carries the routing.
	common.QualityGate = commonPtr(overrides, func(o *model.WorkflowOverride) *string { return o.QualityGate })
	common.ReviewConvergeOn = commonPtr(overrides, func(o *model.WorkflowOverride) *string { return o.ReviewConvergeOn })
	// audit_converge_on is hoisted on the same terms as the twin above. v0.31.0
	// shipped that twin with its resolution and neither config surface, and
	// needed v0.31.1 to finish it; v0.37.0 §7 row 4 exists so this field does
	// not repeat that, and the fence §3 places on this command is what makes
	// hoisting it safe rather than the hoist being harmless.
	common.AuditConvergeOn = commonPtr(overrides, func(o *model.WorkflowOverride) *string { return o.AuditConvergeOn })
	common.RunMaxUnits = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.RunMaxUnits })
	common.RunMaxWallClockSeconds = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.RunMaxWallClockSeconds })
	common.RunMaxUnitRetries = commonPtr(overrides, func(o *model.WorkflowOverride) *int { return o.RunMaxUnitRetries })
	common.RunMaxBudgetUSD = commonPtr(overrides, func(o *model.WorkflowOverride) *float64 { return o.RunMaxBudgetUSD })
	common.RunMaxUnitBudgetUSD = commonPtr(overrides, func(o *model.WorkflowOverride) *float64 { return o.RunMaxUnitBudgetUSD })
	if first := overrides[0].Checks; first != nil {
		all := true
		for i := 1; i < len(overrides); i++ {
			if overrides[i].Checks == nil || !checksEqual(*overrides[i].Checks, *first) {
				all = false
				break
			}
		}
		if all {
			c := *first
			common.Checks = &c
		}
	}
	// runner is compared as raw JSON bytes: this layer hoists a value it never
	// interprets, so two spellings of the same runner are two values here.
	if first := overrides[0].Runner; first != nil {
		all := true
		for i := 1; i < len(overrides); i++ {
			if overrides[i].Runner == nil || !bytes.Equal(overrides[i].Runner, first) {
				all = false
				break
			}
		}
		if all {
			common.Runner = append(json.RawMessage(nil), first...)
		}
	}
	return common
}

// hoistedFields lists the field names set in common, in a deterministic order.
func hoistedFields(common *model.WorkflowOverride) []string {
	var fields []string
	if common.QualityGate != nil {
		fields = append(fields, "quality_gate")
	}
	if common.GateTimeoutSeconds != nil {
		fields = append(fields, "gate_timeout_seconds")
	}
	if common.LockTimeoutSeconds != nil {
		fields = append(fields, "lock_timeout_seconds")
	}
	if common.ReviewCleanRounds != nil {
		fields = append(fields, "review_clean_rounds")
	}
	if common.AuditCleanRounds != nil {
		fields = append(fields, "audit_clean_rounds")
	}
	if common.ReviewMaxRounds != nil {
		fields = append(fields, "review_max_rounds")
	}
	if common.AuditMaxRounds != nil {
		fields = append(fields, "audit_max_rounds")
	}
	if common.ReviewConvergeOn != nil {
		fields = append(fields, "review_converge_on")
	}
	if common.AuditConvergeOn != nil {
		fields = append(fields, "audit_converge_on")
	}
	if common.Checks != nil {
		fields = append(fields, "checks")
	}
	if common.RunMaxUnits != nil {
		fields = append(fields, "run_max_units")
	}
	if common.RunMaxWallClockSeconds != nil {
		fields = append(fields, "run_max_wall_clock_seconds")
	}
	if common.RunMaxBudgetUSD != nil {
		fields = append(fields, "run_max_budget_usd")
	}
	if common.RunMaxUnitBudgetUSD != nil {
		fields = append(fields, "run_max_unit_budget_usd")
	}
	if common.RunMaxUnitRetries != nil {
		fields = append(fields, "run_max_unit_retries")
	}
	if common.Runner != nil {
		fields = append(fields, "runner")
	}
	return fields
}

// mergeCommon overwrites dst's hoisted keys with common's values, preserving
// any other hand-set project field.
func mergeCommon(dst, common *model.WorkflowOverride) {
	if common.QualityGate != nil {
		dst.QualityGate = common.QualityGate
	}
	if common.GateTimeoutSeconds != nil {
		dst.GateTimeoutSeconds = common.GateTimeoutSeconds
	}
	if common.LockTimeoutSeconds != nil {
		dst.LockTimeoutSeconds = common.LockTimeoutSeconds
	}
	if common.ReviewCleanRounds != nil {
		dst.ReviewCleanRounds = common.ReviewCleanRounds
	}
	if common.AuditCleanRounds != nil {
		dst.AuditCleanRounds = common.AuditCleanRounds
	}
	if common.ReviewMaxRounds != nil {
		dst.ReviewMaxRounds = common.ReviewMaxRounds
	}
	if common.AuditMaxRounds != nil {
		dst.AuditMaxRounds = common.AuditMaxRounds
	}
	if common.ReviewConvergeOn != nil {
		dst.ReviewConvergeOn = common.ReviewConvergeOn
	}
	// Listing the field in hoistedFields without writing it here is worse than
	// not hoisting it at all: the thinning strips every listed field from the
	// task layer, so the value would be deleted rather than moved.
	if common.AuditConvergeOn != nil {
		dst.AuditConvergeOn = common.AuditConvergeOn
	}
	if common.Checks != nil {
		dst.Checks = common.Checks
	}
	if common.RunMaxUnits != nil {
		dst.RunMaxUnits = common.RunMaxUnits
	}
	if common.RunMaxWallClockSeconds != nil {
		dst.RunMaxWallClockSeconds = common.RunMaxWallClockSeconds
	}
	if common.RunMaxBudgetUSD != nil {
		dst.RunMaxBudgetUSD = common.RunMaxBudgetUSD
	}
	if common.RunMaxUnitBudgetUSD != nil {
		dst.RunMaxUnitBudgetUSD = common.RunMaxUnitBudgetUSD
	}
	if common.RunMaxUnitRetries != nil {
		dst.RunMaxUnitRetries = common.RunMaxUnitRetries
	}
	if common.Runner != nil {
		dst.Runner = common.Runner
	}
}

// fenceAuditConvergeOnExtract applies v0.37.0 §3's change rule to tp config
// --extract, the fourth write path. A draft of §3 exempted this command as
// "resolution-preserving by construction"; that was reasoned rather than run,
// and running it refuted it — --extract preserves resolution for the task files
// it *scanned*, and changes it for every context without a task-level override.
// --force does not exempt it: it is not tp import --force, which is a named
// unattended decision, and it must not become one silently.
//
// This is the sink, evaluated over every base the repository resolves: each
// scanned task file's own override, plus one empty override standing for every
// base with no task-level override. Each is resolved over the project block
// before and after the merge, with the hoisted fields stripped from the task
// layer exactly as StripTaskWorkflowFields will strip them. The bases with no
// task file are the ones §3's measurement moved from default to project, and
// the empty override is the only shape they have.
//
// The empty override is appended UNCONDITIONALLY rather than only when a spec
// without a task file exists today, and that is a deliberate over-refusal:
// whether some base currently lacks a task file is a property of the tree's
// shape at this instant, not of the write. §3's prospective case is exactly
// what that shape hides — after the hoist the next tp init or tp import writes
// a task file whose empty workflow block resolves blocking from the project
// layer, so the relax lands with no fenced write ever occurring and the later
// import compares equal and passes. The cost is that in a tree where every base
// already has a task file and all of them carry blocking, the hoist changes
// nothing today and is still refused. The routes out are an attended operator
// and tp escalate --decision audit-converge-on; a fence that opened on tree
// shape is not one.
func fenceAuditConvergeOnExtract(overrides []model.WorkflowOverride, project, common *model.WorkflowOverride, hoisted []string) {
	if !engine.Unattended() {
		return
	}
	after := *project
	mergeCommon(&after, common)
	stripped := slices.Contains(hoisted, "audit_converge_on")
	// The evaluated population: every scanned task file's own override, plus
	// one empty override. The empty override is the whole of "every base the
	// repository resolves" that the scan cannot see, and it is ONE element
	// rather than one per spec because engine.ResolveWorkflowLayers is a pure
	// function of the two layers it is handed — no base contributes anything
	// else — so every base without a task-level override resolves identically
	// and enumerating the repository's specs would run the same comparison
	// once per spec.
	population := make([]model.WorkflowOverride, 0, len(overrides)+1)
	population = append(population, overrides...)
	population = append(population, model.WorkflowOverride{})
	for i := range population {
		beforeValue := engine.ResolveWorkflowLayers(&population[i], project).AuditConvergeOn
		taskLayer := population[i]
		if stripped {
			taskLayer.AuditConvergeOn = nil
		}
		afterValue := engine.ResolveWorkflowLayers(&taskLayer, &after).AuditConvergeOn
		if engine.AuditConvergeOnRelaxes(beforeValue, afterValue) {
			refuseUnattendedAuditConvergeOn("tp config --extract", fenceSinkExtract, beforeValue)
			return
		}
	}
}

// gitWorkingTreeDirty reports whether `git status --porcelain` is non-empty in
// dir. inRepo is false when dir is not inside a git repository.
func gitWorkingTreeDirty(dir string) (dirty, inRepo bool) {
	if engine.FindGitBoundary(dir) == "" {
		return false, false
	}
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return false, true
	}
	return strings.TrimSpace(string(out)) != "", true
}

// runConfigExtract implements `tp config --extract`.
func runConfigExtract() error {
	root := engine.ProjectRoot(".")
	files, err := engine.ScanProjectTaskFiles(root)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	overrides := make([]model.WorkflowOverride, 0, len(files))
	for _, f := range files {
		o, loadErr := engine.LoadTaskWorkflowOverride(f)
		if loadErr != nil {
			rel, _ := filepath.Rel(root, f)
			var mce *engine.MalformedConfigError
			if errors.As(loadErr, &mce) {
				output.Error(ExitFile, fmt.Sprintf("malformed task file %s during --extract", rel), mce.Hint())
			} else {
				output.Error(ExitFile, fmt.Sprintf("cannot read %s during --extract: %v", rel, loadErr))
			}
			os.Exit(ExitFile)
			return nil
		}
		overrides = append(overrides, o)
	}

	common := computeCommonPolicy(overrides)
	fields := hoistedFields(&common)
	if len(fields) == 0 {
		output.Info("nothing to hoist: no workflow field is common to all task files")
		return output.JSON(map[string]any{"hoisted": []string{}, "files": len(files)})
	}

	if configExtractDry {
		return output.JSON(map[string]any{"dry_run": true, "hoisted": fields, "files": len(files)})
	}

	// The clean-tree gate always holds — --force never bypasses it.
	if dirty, inRepo := gitWorkingTreeDirty(root); inRepo && dirty {
		output.Error(ExitState, "refusing to run --extract on a dirty working tree; commit or stash first")
		os.Exit(ExitState)
		return nil
	} else if !inRepo {
		output.Info("not a git repository; skipping the clean-tree check")
	}

	tpDir := engine.ProjectConfigDir(".")
	configPath := filepath.Join(tpDir, "config.json")
	// A stat error is not permission to overwrite: the guard fails closed, so
	// an unreadable config is treated as present rather than as absent.
	configPresent, statErr := fileExists(configPath)
	if (configPresent || statErr != nil) && !configExtractForce {
		output.Error(ExitState, ".tp/config.json already exists; re-run with --force to merge",
			"--force merges the hoisted fields into the existing workflow block, preserving other fields")
		os.Exit(ExitState)
		return nil
	}

	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	return engine.WithFileLock(configPath, func() error {
		pc, _, loadErr := engine.LoadProjectConfig(tpDir)
		if loadErr != nil {
			var mce *engine.MalformedConfigError
			if errors.As(loadErr, &mce) {
				output.Error(ExitFile, mce.Error(), mce.Hint())
			} else {
				output.Error(ExitFile, loadErr.Error())
			}
			os.Exit(ExitFile)
			return nil
		}
		// The fence runs on the loaded project block rather than a
		// re-discovered one, so it compares the exact bytes the merge below
		// is about to change. It is deliberately past the --dry-run return
		// above: --dry-run writes nothing, so it changes no resolved value
		// and §3's rule has nothing to refuse.
		fenceAuditConvergeOnExtract(overrides, &pc.Workflow, &common, fields)
		mergeCommon(&pc.Workflow, &common)
		if err := engine.WriteProjectConfig(tpDir, &pc); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		for _, f := range files {
			if err := engine.StripTaskWorkflowFields(f, fields); err != nil {
				output.Error(ExitFile, fmt.Sprintf("cannot thin %s: %v", f, err))
				os.Exit(ExitFile)
				return nil
			}
		}
		return output.JSON(map[string]any{"hoisted": fields, "files": len(files), "config": configPath})
	})
}
