package cli

import (
	"errors"
	"os"

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
func resolvedConfig(wf *model.Workflow, override model.WorkflowOverride) map[string]any {
	project := engine.ProjectWorkflowOverride(".")
	vs := func(value any, o, p bool) map[string]any {
		return map[string]any{"value": value, "source": sourceLabel(o, p)}
	}
	return map[string]any{"workflow": map[string]any{
		"quality_gate":         vs(wf.QualityGate, override.QualityGate != nil, project.QualityGate != nil),
		"gate_timeout_seconds": vs(wf.GateTimeoutSeconds, override.GateTimeoutSeconds != nil, project.GateTimeoutSeconds != nil),
		"review_clean_rounds":  vs(wf.ReviewCleanRounds, override.ReviewCleanRounds != nil, project.ReviewCleanRounds != nil),
		"audit_clean_rounds":   vs(wf.AuditCleanRounds, override.AuditCleanRounds != nil, project.AuditCleanRounds != nil),
		"review_max_rounds":    vs(wf.ReviewMaxRounds, override.ReviewMaxRounds != nil, project.ReviewMaxRounds != nil),
		"audit_max_rounds":     vs(wf.AuditMaxRounds, override.AuditMaxRounds != nil, project.AuditMaxRounds != nil),
		"checks":               vs(wf.Checks, override.Checks != nil, project.Checks != nil),
	}}
}

// resolveConfigWorkflow resolves the effective workflow for tp config: the
// project config (from CWD) layered under the active/--file task file's own
// workflow override. A missing task file yields the project layer alone. A
// malformed config aborts with exit 3 and a repair-or-delete hint.
func resolveConfigWorkflow() (model.Workflow, model.WorkflowOverride) {
	var override model.WorkflowOverride
	if taskFilePath, err := engine.DiscoverTaskFile(".", flagFile); err == nil && taskFilePath != "" {
		override, _ = engine.LoadTaskWorkflowOverride(taskFilePath)
	}
	wf, warnings, err := engine.ResolveEffectiveWorkflow(".", override)
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
		output.Info(w)
	}
	return wf, override
}

func runConfig(_ *cobra.Command, _ []string) error {
	if configExtract {
		return runConfigExtract()
	}
	wf, override := resolveConfigWorkflow()
	if configResolved {
		return output.JSON(resolvedConfig(&wf, override))
	}
	// The effective workflow as JSON on stdout. --compact is a documented no-op
	// (the output is not task-shaped), so tp config always emits this shape.
	return output.JSON(map[string]any{"workflow": wf})
}
