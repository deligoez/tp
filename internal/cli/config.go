package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	configResolved     bool
	configExtract      bool
	configExtractDry   bool
	configExtractForce bool
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print the resolved effective configuration as JSON",
		RunE:  runConfig,
	}
	cmd.Flags().BoolVar(&configResolved, "resolved", false, "Annotate each setting with its {value, source} layer")
	cmd.Flags().BoolVar(&configExtract, "extract", false, "Hoist shared workflow policy from task files into .tp/config.json")
	cmd.Flags().BoolVar(&configExtractDry, "dry-run", false, "With --extract: print the plan without writing")
	cmd.Flags().BoolVar(&configExtractForce, "force", false, "With --extract: merge into an existing .tp/config.json")
	return cmd
}

// sourceLabel names the layer a resolved field came from: override (task file),
// project (.tp/config.json), or default (built-in).
func sourceLabel(fromOverride, fromProject bool) string {
	switch {
	case fromOverride:
		return "override"
	case fromProject:
		return "project"
	default:
		return "default"
	}
}

// resolvedConfig annotates each workflow field with its value and source layer.
// Workflow fields resolve across override/project/default only.
// resolvedConfig annotates each workflow field with its value and source layer.
// Workflow fields resolve across override/project/default only — except
// notify_cmd, which is per-operator and can only come from .tp/local.json, so
// it reports local or default and never override or project (§7).
func resolvedConfig(wf *model.Workflow, override *model.WorkflowOverride) map[string]any {
	project := engine.ProjectWorkflowOverride()
	vs := func(value any, o, p bool) map[string]any {
		return map[string]any{"value": value, "source": sourceLabel(o, p)}
	}
	// commit_strategy's resolved value is the normalized name (builtin/auto/hc),
	// so a present unrecognized value reports as builtin, not the raw string (§5.2).
	csName, _ := engine.ResolveCommitStrategy(override.CommitStrategy, project.CommitStrategy)
	notifySource := "default"
	if wf.NotifyCmd != "" {
		notifySource = "local"
	}
	result := map[string]any{"workflow": map[string]any{
		"quality_gate":         vs(wf.QualityGate, override.QualityGate != nil, project.QualityGate != nil),
		"commit_strategy":      vs(csName, override.CommitStrategy != nil, project.CommitStrategy != nil),
		"gate_timeout_seconds": vs(wf.GateTimeoutSeconds, override.GateTimeoutSeconds != nil, project.GateTimeoutSeconds != nil),
		"lock_timeout_seconds": vs(wf.LockTimeoutSeconds, override.LockTimeoutSeconds != nil, project.LockTimeoutSeconds != nil),
		"review_clean_rounds":  vs(wf.ReviewCleanRounds, override.ReviewCleanRounds != nil, project.ReviewCleanRounds != nil),
		"audit_clean_rounds":   vs(wf.AuditCleanRounds, override.AuditCleanRounds != nil, project.AuditCleanRounds != nil),
		"review_max_rounds":    vs(wf.ReviewMaxRounds, override.ReviewMaxRounds != nil, project.ReviewMaxRounds != nil),
		"audit_max_rounds":     vs(wf.AuditMaxRounds, override.AuditMaxRounds != nil, project.AuditMaxRounds != nil),
		"checks":               vs(wf.Checks, override.Checks != nil, project.Checks != nil),
		// review_converge_on reports the raw resolved value and its source; an
		// invalid stored value is surfaced, not rejected here (§3.3).
		"review_converge_on": vs(wf.ReviewConvergeOn, override.ReviewConvergeOn != nil, project.ReviewConvergeOn != nil),
		// audit_converge_on reports on the same terms as its twin: the raw
		// resolved value and the layer it won on, unvalidated here (§2).
		"audit_converge_on": vs(wf.AuditConvergeOn, override.AuditConvergeOn != nil, project.AuditConvergeOn != nil),

		// §7's run fields. runner reports its raw JSON value whatever shape it
		// takes, so a per-kind map is visible here before the runner resolver
		// ever interprets it.
		"run_max_units":              vs(wf.RunMaxUnits, override.RunMaxUnits != nil, project.RunMaxUnits != nil),
		"run_max_wall_clock_seconds": vs(wf.RunMaxWallClockSeconds, override.RunMaxWallClockSeconds != nil, project.RunMaxWallClockSeconds != nil),
		"run_max_budget_usd":         vs(wf.RunMaxBudgetUSD, override.RunMaxBudgetUSD != nil, project.RunMaxBudgetUSD != nil),
		"run_max_unit_budget_usd":    vs(wf.RunMaxUnitBudgetUSD, override.RunMaxUnitBudgetUSD != nil, project.RunMaxUnitBudgetUSD != nil),
		"run_max_unit_retries":       vs(wf.RunMaxUnitRetries, override.RunMaxUnitRetries != nil, project.RunMaxUnitRetries != nil),
		"runner":                     vs(wf.Runner, override.Runner != nil, project.Runner != nil),
		"notify_cmd":                 map[string]any{"value": wf.NotifyCmd, "source": notifySource},
	}}
	// active provenance: the resolved active file and its discovery-chain rank
	// (cli/env/local/legacy/autodetect).
	if path, source := resolvedActiveSource(); path != "" {
		result["active"] = map[string]any{"value": path, "source": source}
	}
	// defaults provenance: flag defaults from .tp/local.json report local.
	if defaults := engine.LocalFlagDefaults(); len(defaults) > 0 {
		dmap := make(map[string]any, len(defaults))
		for k, v := range defaults {
			dmap[k] = map[string]any{"value": v, "source": "local"}
		}
		result["defaults"] = dmap
	}
	return result
}

// surfaceConfigWarnings prints .tp/config.json and .tp/local.json validation
// warnings (unknown keys, type mismatches, out-of-range fallbacks) to stderr,
// so every command that reads the config reports them. A malformed config is
// handled by each command's own loader (exit 3), not here.
func surfaceConfigWarnings() {
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		return
	}
	// The third return is the malformed-config error, and dropping it made
	// `tp validate --project` exit 0 with an empty deviation list over a
	// truncated .tp/config.json — indistinguishable from a genuinely clean
	// project, which is the failure that file already guards against for scan
	// errors and malformed task files. A malformed config is still not fatal
	// here (each command's own loader owns exit 3); it is reported, so a clean
	// report means the config was read.
	_, cw, cErr := engine.LoadProjectConfig(tpDir)
	_, lw, lErr := engine.LoadLocalConfig(tpDir)
	for _, e := range []error{cErr, lErr} {
		if e != nil {
			output.Notice(fmt.Sprintf("warning: %v", e))
		}
	}
	// Sorted because both loaders walk a map: unsorted, the same config emitted
	// its warnings in a different order run to run, which makes two identical
	// invocations differ in bytes and costs a driving agent context on churn.
	warnings := make([]string, 0, len(cw)+len(lw))
	warnings = append(warnings, cw...)
	warnings = append(warnings, lw...)
	sort.Strings(warnings)
	for _, w := range warnings {
		// output.Notice, not a raw stderr write: this is the channel that
		// honours --quiet. The raw form left `tp validate --project --quiet`
		// printing config warnings one line above a repair that had just made
		// its own two warnings quiet.
		output.Notice(fmt.Sprintf("warning: %s", w))
	}
}

// resolveConfigWorkflow resolves the effective workflow for tp config: the
// project config (from CWD) layered under the active/--file task file's own
// workflow override. A missing task file yields the project layer alone. A
// malformed config aborts with exit 3 and a repair-or-delete hint.
func resolveConfigWorkflow() (model.Workflow, model.WorkflowOverride) {
	var override model.WorkflowOverride
	if taskFilePath, err := engine.DiscoverTaskFile(".", flagFile); err == nil && taskFilePath != "" {
		// A parse error yields an empty override; ResolveEffectiveWorkflow below
		// re-reads and aborts with exit 3 on a truly malformed config.
		override, _ = engine.LoadTaskWorkflowOverride(taskFilePath)
	}
	wf, warnings, err := engine.ResolveEffectiveWorkflow(".", &override)
	if err != nil {
		var mce *engine.MalformedConfigError
		if errors.As(err, &mce) {
			output.Error(ExitFile, mce.Error(), mce.Hint())
		} else {
			output.Error(ExitFile, err.Error())
		}
		os.Exit(ExitFile)
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	// Clamp the override so a present but out-of-range field is attributed to the
	// layer that supplied the resolved value, not "override" (§3.4).
	engine.ClampWorkflowRanges(&override)
	return wf, override
}

func runConfig(_ *cobra.Command, _ []string) error {
	if configExtract {
		return runConfigExtract()
	}
	wf, override := resolveConfigWorkflow()
	// commit_strategy is a strategy-reading surface: warn on an unrecognized value
	// or an hc-absent hc strategy, and expose the concrete builtin/hc behavior
	// after auto resolution (§5.2).
	effective := warnCommitStrategy(override.CommitStrategy, engine.ProjectWorkflowOverride().CommitStrategy)
	if configResolved {
		return output.JSON(resolvedConfig(&wf, &override))
	}
	// The effective workflow as JSON on stdout, plus commit_strategy_effective (the
	// concrete builtin/hc behavior). --compact is a documented no-op (the output is
	// not task-shaped), so tp config always emits this shape.
	return output.JSON(map[string]any{"workflow": wf, "commit_strategy_effective": effective})
}
