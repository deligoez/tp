package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// runSetProjectWorkflow implements `tp set --workflow --project field=value`,
// writing project-wide workflow defaults to the .tp/config.json workflow block.
// Unlike per-task set, quality_gate is authorable here; out-of-range integer
// values are rejected with exit 1. Writes acquire the standard flock.
func runSetProjectWorkflow(args []string) error {
	if len(args) == 0 {
		output.Error(ExitUsage, "tp set --workflow --project requires at least one field=value pair")
		os.Exit(ExitUsage)
		return nil
	}
	surfaceConfigWarnings()

	ints := make(map[string]int)
	floats := make(map[string]float64)
	var qualityGate *string
	var commitStrategy *string
	var reviewConvergeOn *string
	var auditConvergeOn *string
	var checksValue *[]model.Check
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			output.Error(ExitUsage, fmt.Sprintf("expected field=value format, got %q", arg))
			os.Exit(ExitUsage)
			return nil
		}
		field, valueStr := parts[0], parts[1]
		// §5.1: the project config is a layer like any other, and runner and
		// notify_cmd are fenced at every one of them.
		if engine.Unattended() && engine.FencedCommandField(field) {
			refuseUnattendedCommandField(field)
			return nil
		}
		switch {
		case field == "quality_gate":
			v := valueStr
			qualityGate = &v
		case field == "commit_strategy":
			switch valueStr {
			case engine.CommitStrategyBuiltin, engine.CommitStrategyAuto, engine.CommitStrategyHC:
			default:
				output.Error(ExitValidation, fmt.Sprintf("commit_strategy must be one of builtin, auto, hc; got %q", valueStr))
				os.Exit(ExitValidation)
				return nil
			}
			v := valueStr
			commitStrategy = &v
		case field == "review_converge_on":
			// The literal argument is validated before it is persisted; a bad
			// value is a usage error (exit 2), since it is a command-line
			// argument (§3.3).
			if !engine.ValidReviewConvergeOn(valueStr) {
				output.Error(ExitUsage, fmt.Sprintf("invalid review_converge_on value %q", valueStr), engine.ReviewConvergeOnHint)
				os.Exit(ExitUsage)
				return nil
			}
			v := valueStr
			reviewConvergeOn = &v
		case field == "audit_converge_on":
			// v0.37.0 §2's audit twin, refused on the same terms and with the
			// same hint. This case sits above the editableWorkflowFields arm
			// below, which would otherwise parse the literal as an integer and
			// report ExitValidation — the very code §7 row 3 keeps distinct
			// from the write sinks'.
			if !engine.ValidAuditConvergeOn(valueStr) {
				output.Error(ExitUsage, fmt.Sprintf("invalid audit_converge_on value %q", valueStr), engine.AuditConvergeOnHint)
				os.Exit(ExitUsage)
				return nil
			}
			v := valueStr
			auditConvergeOn = &v
		case field == "checks":
			var checks []model.Check
			if err := json.Unmarshal([]byte(valueStr), &checks); err != nil {
				output.Error(ExitValidation, fmt.Sprintf("checks must be a JSON array of {class, cmd} objects: %v", err))
				os.Exit(ExitValidation)
				return nil
			}
			if err := engine.ValidateChecks(checks); err != nil {
				output.Error(ExitValidation, err.Error())
				os.Exit(ExitValidation)
				return nil
			}
			if checks == nil {
				checks = []model.Check{}
			}
			checksValue = &checks
		case floatWorkflowFields[field]:
			val, convErr := strconv.ParseFloat(valueStr, 64)
			if convErr != nil {
				output.Error(ExitValidation, fmt.Sprintf("%s must be a number", field))
				os.Exit(ExitValidation)
				return nil
			}
			fenceWorkflowWrite(field, val)
			validateWorkflowFloat(field, val)
			floats[field] = val
		case editableWorkflowFields[field]:
			val, convErr := strconv.Atoi(valueStr)
			if convErr != nil {
				output.Error(ExitValidation, fmt.Sprintf("%s must be an integer", field))
				os.Exit(ExitValidation)
				return nil
			}
			fenceWorkflowWrite(field, float64(val))
			lo, hi := workflowFieldRange(field)
			if val < lo || val > hi {
				output.Error(ExitValidation, fmt.Sprintf("%s must be between %d and %d", field, lo, hi))
				os.Exit(ExitValidation)
				return nil
			}
			ints[field] = val
		default:
			output.Error(ExitValidation, fmt.Sprintf("unknown workflow field: %s", field))
			os.Exit(ExitValidation)
			return nil
		}
	}

	tpDir := engine.ProjectConfigDir(".")
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	configPath := filepath.Join(tpDir, "config.json")
	return engine.WithFileLock(configPath, func() error {
		pc, _, err := engine.LoadProjectConfig(tpDir)
		if err != nil {
			var mce *engine.MalformedConfigError
			if errors.As(err, &mce) {
				output.Error(ExitFile, mce.Error(), mce.Hint())
			} else {
				output.Error(ExitFile, err.Error())
			}
			os.Exit(ExitFile)
			return nil
		}

		updated := make(map[string]any)
		for field, val := range ints {
			v := val
			switch field {
			case "review_clean_rounds":
				pc.Workflow.ReviewCleanRounds = &v
			case "audit_clean_rounds":
				pc.Workflow.AuditCleanRounds = &v
			case "gate_timeout_seconds":
				pc.Workflow.GateTimeoutSeconds = &v
			case "lock_timeout_seconds":
				pc.Workflow.LockTimeoutSeconds = &v
			case "review_max_rounds":
				pc.Workflow.ReviewMaxRounds = &v
			case "audit_max_rounds":
				pc.Workflow.AuditMaxRounds = &v
			case "run_max_units":
				pc.Workflow.RunMaxUnits = &v
			case "run_max_wall_clock_seconds":
				pc.Workflow.RunMaxWallClockSeconds = &v
			case "run_max_unit_retries":
				pc.Workflow.RunMaxUnitRetries = &v
			}
			updated[field] = val
		}
		for field, val := range floats {
			v := val
			switch field {
			case "run_max_budget_usd":
				pc.Workflow.RunMaxBudgetUSD = &v
			case "run_max_unit_budget_usd":
				pc.Workflow.RunMaxUnitBudgetUSD = &v
			}
			updated[field] = val
		}
		if qualityGate != nil {
			pc.Workflow.QualityGate = qualityGate
			updated["quality_gate"] = *qualityGate
		}
		if commitStrategy != nil {
			pc.Workflow.CommitStrategy = commitStrategy
			updated["commit_strategy"] = *commitStrategy
		}
		if reviewConvergeOn != nil {
			pc.Workflow.ReviewConvergeOn = reviewConvergeOn
			updated["review_converge_on"] = *reviewConvergeOn
		}
		if auditConvergeOn != nil {
			pc.Workflow.AuditConvergeOn = auditConvergeOn
			updated["audit_converge_on"] = *auditConvergeOn
		}
		if checksValue != nil {
			pc.Workflow.Checks = checksValue
			updated["checks"] = *checksValue
		}

		if err := engine.WriteProjectConfig(tpDir, &pc); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		warnShadowedProjectWrites(updated)
		return output.JSON(map[string]any{"updated": updated, "config": configPath})
	})
}

// warnShadowedProjectWrites reports any field just written to .tp/config.json
// whose resolved value still comes from somewhere else.
//
// The project layer is outranked by a task file's own workflow block, so a
// write here can be accepted, reported as updated, and have no effect — which
// is what a project hit after adding a step to its gate: `tp set --workflow
// --project quality_gate=...` answered {"updated":{...}} while the gate that
// actually ran stayed the one `tp init --quality-gate` had authored.
//
// It warns rather than refuses because the write is legitimate wherever no
// override exists, which is the ordinary case and the one tp's own repository
// is in. What was wrong was reporting success without saying the value is
// shadowed. The check is per field and layer-agnostic, so it covers any field
// and any future layer rather than the one that surfaced it.
//
// The resolved source comes from resolvedConfig, the same function behind
// `tp config --resolved`, so this warning cannot disagree with what a reader
// sees when they go and look.
func warnShadowedProjectWrites(updated map[string]any) {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		// No task file, so no override layer exists to shadow anything.
		return
	}
	override, err := engine.LoadTaskWorkflowOverride(taskFilePath)
	if err != nil {
		return
	}
	wf := engine.EffectiveWorkflowForTaskFile(taskFilePath)

	resolved, ok := resolvedConfig(&wf, &override)["workflow"].(map[string]any)
	if !ok {
		return
	}

	shadowed := make([]string, 0, len(updated))
	for field := range updated {
		entry, isMap := resolved[field].(map[string]any)
		if !isMap {
			continue
		}
		if source, _ := entry["source"].(string); source != "" && source != "project" {
			shadowed = append(shadowed, fmt.Sprintf("%s (still resolves from %s)", field, source))
		}
	}
	if len(shadowed) == 0 {
		return
	}
	sort.Strings(shadowed)

	output.Notice(fmt.Sprintf(
		"warning: written to .tp/config.json but shadowed by %s: %s; run 'tp config --resolved' to see what actually applies",
		filepath.Base(taskFilePath), strings.Join(shadowed, ", ")))
}
