package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	managedFields = map[string]string{
		"status":              "use `tp claim`, `tp close`, or `tp reopen`",
		"started_at":          "set automatically by `tp claim` / `tp done` / `tp next`",
		"closed_at":           "set automatically by `tp close` / `tp done`",
		"closed_reason":       "set automatically by `tp close` / `tp done`",
		"gate_passed_at":      "set automatically by `tp done --gate-passed`",
		"commit_sha":          "set automatically by `tp done --commit`",
		"commit_shas":         "set automatically by `tp done --commit`",
		"gate_skipped_reason": "set automatically by `tp done --skip-gate`",
	}
	setBulkFile     string
	setWorkflowFlag bool
	setProjectFlag  bool
	setLocalFlag    bool

	editableWorkflowFields = map[string]bool{
		"review_clean_rounds":  true,
		"audit_clean_rounds":   true,
		"gate_timeout_seconds": true,
		"lock_timeout_seconds": true,
		"review_max_rounds":    true,
		"audit_max_rounds":     true,
		"checks":               true,
	}
	readOnlyWorkflowFields = map[string]bool{
		"quality_gate":    true,
		"commit_strategy": true,
	}
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <id> <field>=<value>",
		Short: "Update a task field (except managed fields)",
		Args:  cobra.RangeArgs(0, 100),
		RunE:  runSet,
	}
	cmd.Flags().StringVar(&setBulkFile, "bulk", "", "NDJSON file with {id, field, value} lines")
	cmd.Flags().BoolVar(&setWorkflowFlag, "workflow", false, "update workflow-level fields instead of a task")
	cmd.Flags().BoolVar(&setProjectFlag, "project", false, "with --workflow: write to the project .tp/config.json")
	cmd.Flags().BoolVar(&setLocalFlag, "local", false, "write a flag default to .tp/local.json (defaults.<flag>=<bool>)")
	return cmd
}

// setLine represents a single bulk-set operation.
type setLine struct {
	ID    string          `json:"id"`
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

func runSet(_ *cobra.Command, args []string) error {
	if setLocalFlag {
		return runSetLocal(args)
	}

	if setWorkflowFlag {
		if setProjectFlag {
			return runSetProjectWorkflow(args)
		}
		return runSetWorkflow(args)
	}

	if setBulkFile != "" {
		return runSetBulk()
	}

	if len(args) != 2 {
		output.Error(ExitUsage, "expected <id> <field>=<value> or --bulk <file>")
		os.Exit(ExitUsage)
		return nil
	}

	parts := strings.SplitN(args[1], "=", 2)
	if len(parts) != 2 {
		output.Error(ExitUsage, "expected field=value format")
		os.Exit(ExitUsage)
		return nil
	}
	field, value := parts[0], parts[1]

	if hint, managed := managedFields[field]; managed {
		output.Error(ExitUsage, fmt.Sprintf("field %q is managed — %s", field, hint))
		os.Exit(ExitUsage)
		return nil
	}

	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		task, _, err := model.FindTask(tf, args[0])
		if err != nil {
			output.Error(ExitState, err.Error())
			os.Exit(ExitState)
			return nil
		}

		if err := applyField(task, field, value); err != nil {
			output.Error(ExitUsage, err.Error())
			os.Exit(ExitUsage)
			return nil
		}

		// §7.1: anchor changes recompute coverage when the spec is readable.
		// Non-anchor fields (title, estimate_minutes, tags, …) skip the recompute.
		if field == "source_sections" || field == "source_lines" {
			if specPath, specExists := engine.ResolveSpecPath(taskFilePath, tf.Spec); specExists {
				engine.AutoFillCoverage(tf, specPath)
			}
		}

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("updated %s.%s", task.ID, field))
		return output.JSON(task)
	})
}

func runSetBulk() error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	f, err := os.Open(setBulkFile)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("open bulk file: %v", err))
		os.Exit(ExitFile)
		return nil
	}
	defer f.Close()

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		type setResult struct {
			ID    string `json:"id"`
			Field string `json:"field"`
			OK    bool   `json:"ok"`
			Error string `json:"error,omitempty"`
		}
		results := make([]setResult, 0)
		updated, failed := 0, 0
		anchorChanged := false // §7.1: any source_sections/source_lines edit triggers a coverage recompute

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var sl setLine
			if err := json.Unmarshal([]byte(line), &sl); err != nil {
				results = append(results, setResult{Error: fmt.Sprintf("invalid JSON: %s", line)})
				failed++
				continue
			}

			if hint, managed := managedFields[sl.Field]; managed {
				results = append(results, setResult{ID: sl.ID, Field: sl.Field, Error: fmt.Sprintf("field %q is managed — %s", sl.Field, hint)})
				failed++
				continue
			}

			task, _, err := model.FindTask(tf, sl.ID)
			if err != nil {
				results = append(results, setResult{ID: sl.ID, Field: sl.Field, Error: err.Error()})
				failed++
				continue
			}

			// Use raw JSON value as the string form for applyField
			valueStr := string(sl.Value)
			// Strip surrounding quotes for string fields
			var unquoted string
			if json.Unmarshal(sl.Value, &unquoted) == nil {
				valueStr = unquoted
			}

			if err := applyField(task, sl.Field, valueStr); err != nil {
				results = append(results, setResult{ID: sl.ID, Field: sl.Field, Error: err.Error()})
				failed++
				continue
			}

			if sl.Field == "source_sections" || sl.Field == "source_lines" {
				anchorChanged = true
			}

			results = append(results, setResult{ID: sl.ID, Field: sl.Field, OK: true})
			updated++
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stopped reading %s early (%v); set lines after the over-long line were dropped (line cap is 64KB)\n", setBulkFile, err)
		}

		// §7.1: recompute coverage if any anchor field changed in this batch.
		if anchorChanged {
			if specPath, specExists := engine.ResolveSpecPath(taskFilePath, tf.Spec); specExists {
				engine.AutoFillCoverage(tf, specPath)
			}
		}

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("bulk set: %d updated, %d failed", updated, failed))
		return output.JSON(map[string]any{"updated": updated, "failed": failed, "results": results})
	})
}

func applyField(task *model.Task, field, value string) error {
	switch field {
	case "title":
		task.Title = value
	case "description":
		task.Description = value
	case "acceptance":
		task.Acceptance = value
	case "estimate_minutes":
		var v int
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return fmt.Errorf("invalid integer: %s", value)
		}
		task.EstimateMinutes = v
	case "tags":
		var v []string
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return fmt.Errorf("invalid JSON array: %s", value)
		}
		task.Tags = v
	case "depends_on":
		var v []string
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return fmt.Errorf("invalid JSON array: %s", value)
		}
		task.DependsOn = v
	case "source_sections":
		var v []string
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return fmt.Errorf("invalid JSON array: %s", value)
		}
		task.SourceSections = v
	case "source_lines":
		task.SourceLines = value
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
	return nil
}

func runSetWorkflow(args []string) error {
	if len(args) == 0 {
		output.Error(ExitUsage, "tp set --workflow requires at least one field=value pair")
		os.Exit(ExitUsage)
		return nil
	}

	// Parse all field=value pairs first
	pairs := make(map[string]int)
	var checksValue []model.Check
	checksSet := false
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			output.Error(ExitUsage, fmt.Sprintf("expected field=value format, got %q", arg))
			os.Exit(ExitUsage)
			return nil
		}
		field, valueStr := parts[0], parts[1]

		if readOnlyWorkflowFields[field] {
			msg := fmt.Sprintf("%s is not settable via tp set --workflow; it is authored only by tp init", field)
			if field == "commit_strategy" {
				msg = fmt.Sprintf("%s is not settable via tp set --workflow; it is authored only by tp init — set the project default with `tp set --workflow --project commit_strategy=<builtin|auto|hc>`", field)
			}
			output.Error(ExitUsage, msg)
			os.Exit(ExitUsage)
			return nil
		}
		if !editableWorkflowFields[field] {
			output.Error(ExitUsage, fmt.Sprintf("unknown workflow field: %s", field))
			os.Exit(ExitUsage)
			return nil
		}

		// checks: JSON-array replace semantics (§15.2)
		if field == "checks" {
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
			checksValue = checks
			checksSet = true
			continue
		}

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
		pairs[field] = val
	}

	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		updated := make(map[string]any)
		for field, val := range pairs {
			v := val
			switch field {
			case "review_clean_rounds":
				tf.Workflow.ReviewCleanRounds = &v
			case "audit_clean_rounds":
				tf.Workflow.AuditCleanRounds = &v
			case "gate_timeout_seconds":
				tf.Workflow.GateTimeoutSeconds = &v
			case "lock_timeout_seconds":
				tf.Workflow.LockTimeoutSeconds = &v
			case "review_max_rounds":
				tf.Workflow.ReviewMaxRounds = &v
			case "audit_max_rounds":
				tf.Workflow.AuditMaxRounds = &v
			}
			updated[field] = val
		}
		if checksSet {
			tf.Workflow.Checks = &checksValue
			updated["checks"] = checksValue
		}

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		return output.JSON(map[string]any{"updated": updated})
	})
}

// workflowFieldRange returns the valid write range for an editable workflow field.
func workflowFieldRange(field string) (lo, hi int) {
	switch field {
	case "gate_timeout_seconds":
		return 30, 3600
	case "lock_timeout_seconds":
		return 1, 60
	case "review_max_rounds", "audit_max_rounds":
		return 0, 50
	default:
		return 1, 10 // convergence fields
	}
}
