package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// computeCommonPolicy returns the workflow fields that EVERY override sets with
// an identical value — the only fields tp config --extract hoists.
func computeCommonPolicy(overrides []model.WorkflowOverride) model.WorkflowOverride {
	var common model.WorkflowOverride
	if len(overrides) == 0 {
		return common
	}
	commonInt := func(get func(model.WorkflowOverride) *int) *int {
		first := get(overrides[0])
		if first == nil {
			return nil
		}
		for _, o := range overrides[1:] {
			v := get(o)
			if v == nil || *v != *first {
				return nil
			}
		}
		val := *first
		return &val
	}
	common.GateTimeoutSeconds = commonInt(func(o model.WorkflowOverride) *int { return o.GateTimeoutSeconds })
	common.ReviewCleanRounds = commonInt(func(o model.WorkflowOverride) *int { return o.ReviewCleanRounds })
	common.AuditCleanRounds = commonInt(func(o model.WorkflowOverride) *int { return o.AuditCleanRounds })
	common.ReviewMaxRounds = commonInt(func(o model.WorkflowOverride) *int { return o.ReviewMaxRounds })
	common.AuditMaxRounds = commonInt(func(o model.WorkflowOverride) *int { return o.AuditMaxRounds })
	if first := overrides[0].QualityGate; first != nil {
		all := true
		for _, o := range overrides[1:] {
			if o.QualityGate == nil || *o.QualityGate != *first {
				all = false
				break
			}
		}
		if all {
			v := *first
			common.QualityGate = &v
		}
	}
	if first := overrides[0].Checks; first != nil {
		all := true
		for _, o := range overrides[1:] {
			if o.Checks == nil || !checksEqual(*o.Checks, *first) {
				all = false
				break
			}
		}
		if all {
			c := *first
			common.Checks = &c
		}
	}
	return common
}

// hoistedFields lists the field names set in common, in a deterministic order.
func hoistedFields(common model.WorkflowOverride) []string {
	var fields []string
	if common.QualityGate != nil {
		fields = append(fields, "quality_gate")
	}
	if common.GateTimeoutSeconds != nil {
		fields = append(fields, "gate_timeout_seconds")
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
	if common.Checks != nil {
		fields = append(fields, "checks")
	}
	return fields
}

// mergeCommon overwrites dst's hoisted keys with common's values, preserving
// any other hand-set project field.
func mergeCommon(dst *model.WorkflowOverride, common model.WorkflowOverride) {
	if common.QualityGate != nil {
		dst.QualityGate = common.QualityGate
	}
	if common.GateTimeoutSeconds != nil {
		dst.GateTimeoutSeconds = common.GateTimeoutSeconds
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
	if common.Checks != nil {
		dst.Checks = common.Checks
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
	fields := hoistedFields(common)
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
	if fileExists(configPath) && !configExtractForce {
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
	pc, _, _ := engine.LoadProjectConfig(tpDir)
	mergeCommon(&pc.Workflow, common)
	if err := engine.WriteProjectConfig(tpDir, pc); err != nil {
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
}
