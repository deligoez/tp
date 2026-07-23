package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	var qualityGate *string
	var commitStrategy *string
	var checksValue *[]model.Check
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			output.Error(ExitUsage, fmt.Sprintf("expected field=value format, got %q", arg))
			os.Exit(ExitUsage)
			return nil
		}
		field, valueStr := parts[0], parts[1]
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
		case editableWorkflowFields[field]:
			val, convErr := strconv.Atoi(valueStr)
			if convErr != nil {
				output.Error(ExitValidation, fmt.Sprintf("%s must be an integer", field))
				os.Exit(ExitValidation)
				return nil
			}
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
		if checksValue != nil {
			pc.Workflow.Checks = checksValue
			updated["checks"] = *checksValue
		}

		if err := engine.WriteProjectConfig(tpDir, pc); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		return output.JSON(map[string]any{"updated": updated, "config": configPath})
	})
}
