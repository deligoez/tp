package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	managedFields = map[string]string{
		"status":         "use `tp claim`, `tp close`, or `tp reopen`",
		"started_at":     "set automatically by `tp claim` / `tp done` / `tp next`",
		"closed_at":      "set automatically by `tp close` / `tp done`",
		"closed_reason":  "set automatically by `tp close` / `tp done`",
		"gate_passed_at": "set automatically by `tp done --gate-passed`",
		"commit_sha":     "set automatically by `tp done --commit`",
	}
	setBulkFile     string
	setWorkflowFlag bool

	editableWorkflowFields = map[string]bool{
		"review_clean_rounds": true,
		"audit_clean_rounds":  true,
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
	return cmd
}

// setLine represents a single bulk-set operation.
type setLine struct {
	ID    string          `json:"id"`
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

func runSet(_ *cobra.Command, args []string) error {
	if setWorkflowFlag {
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

			results = append(results, setResult{ID: sl.ID, Field: sl.Field, OK: true})
			updated++
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
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			output.Error(ExitUsage, fmt.Sprintf("expected field=value format, got %q", arg))
			os.Exit(ExitUsage)
			return nil
		}
		field, valueStr := parts[0], parts[1]

		if readOnlyWorkflowFields[field] {
			return output.JSON(map[string]string{"error": fmt.Sprintf("%s is read-only; edit the task file directly to change it", field)})
		}
		if !editableWorkflowFields[field] {
			return output.JSON(map[string]string{"error": fmt.Sprintf("unknown workflow field: %s", field)})
		}

		var val int
		if _, err := fmt.Sscanf(valueStr, "%d", &val); err != nil {
			return output.JSON(map[string]string{"error": fmt.Sprintf("%s must be an integer", field)})
		}
		if val < 1 || val > 10 {
			return output.JSON(map[string]string{"error": fmt.Sprintf("%s must be between 1 and 10", field)})
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

		updated := make(map[string]int)
		for field, val := range pairs {
			switch field {
			case "review_clean_rounds":
				tf.Workflow.ReviewCleanRounds = val
			case "audit_clean_rounds":
				tf.Workflow.AuditCleanRounds = val
			}
			updated[field] = val
		}

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		return output.JSON(map[string]any{"updated": updated})
	})
}
