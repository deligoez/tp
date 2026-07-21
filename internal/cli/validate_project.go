package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// workflowDeviations reports each workflow field where a task file's override
// differs from a value the project config explicitly sets. A field the project
// does not set carries no policy and is not a deviation.
func workflowDeviations(file string, override, project model.WorkflowOverride) []map[string]any {
	devs := make([]map[string]any, 0)
	add := func(field, ov, pv string) {
		devs = append(devs, map[string]any{"file": file, "field": field, "override": ov, "project": pv})
	}
	cmpInt := func(field string, o, p *int) {
		if o != nil && p != nil && *o != *p {
			add(field, strconv.Itoa(*o), strconv.Itoa(*p))
		}
	}
	cmpInt("gate_timeout_seconds", override.GateTimeoutSeconds, project.GateTimeoutSeconds)
	cmpInt("review_clean_rounds", override.ReviewCleanRounds, project.ReviewCleanRounds)
	cmpInt("audit_clean_rounds", override.AuditCleanRounds, project.AuditCleanRounds)
	cmpInt("review_max_rounds", override.ReviewMaxRounds, project.ReviewMaxRounds)
	cmpInt("audit_max_rounds", override.AuditMaxRounds, project.AuditMaxRounds)
	if override.QualityGate != nil && project.QualityGate != nil && *override.QualityGate != *project.QualityGate {
		add("quality_gate", *override.QualityGate, *project.QualityGate)
	}
	return devs
}

// runValidateProject implements `tp validate --project`: it reports each task
// file's workflow-field deviations from the project config. Deviations are
// informational (exit 0) unless --strict promotes them to exit 1.
func runValidateProject() error {
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" || !fileExists(filepath.Join(tpDir, "config.json")) {
		output.Info("no project config found (.tp/config.json)")
		return output.JSON(map[string]any{"project_config": false, "deviations": []any{}})
	}

	project := engine.ProjectWorkflowOverride(".")
	root := engine.ProjectRoot(".")
	files, _ := engine.ScanProjectTaskFiles(root)

	deviations := make([]map[string]any, 0)
	skipped := make([]string, 0)
	for _, f := range files {
		override, err := engine.LoadTaskWorkflowOverride(f)
		if err != nil {
			rel, _ := filepath.Rel(root, f)
			skipped = append(skipped, rel)
			fmt.Fprintf(os.Stderr, "warning: skipping malformed task file %s: %v\n", rel, err)
			continue
		}
		rel, _ := filepath.Rel(root, f)
		deviations = append(deviations, workflowDeviations(rel, override, project)...)
	}

	result := map[string]any{"project_config": true, "deviations": deviations}
	if len(skipped) > 0 {
		result["skipped"] = skipped
	}
	if validateStrict && len(deviations) > 0 {
		_ = output.JSON(result)
		os.Exit(ExitValidation)
	}
	return output.JSON(result)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
