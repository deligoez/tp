package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var managedFields = map[string]string{
	"status":         "use `tp claim`, `tp close`, or `tp reopen`",
	"closed_at":      "set automatically by `tp close` / `tp done`",
	"closed_reason":  "set automatically by `tp close` / `tp done`",
	"gate_passed_at": "set automatically by `tp done --gate-passed`",
	"commit_sha":     "set automatically by `tp done --commit`",
}

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <id> <field>=<value>",
		Short: "Update a task field (except managed fields)",
		Args:  cobra.ExactArgs(2),
		RunE:  runSet,
	}
}

func runSet(_ *cobra.Command, args []string) error {
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
				output.Error(ExitUsage, fmt.Sprintf("invalid integer: %s", value))
				os.Exit(ExitUsage)
				return nil
			}
			task.EstimateMinutes = v
		case "tags":
			var v []string
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				output.Error(ExitUsage, fmt.Sprintf("invalid JSON array: %s", value))
				os.Exit(ExitUsage)
				return nil
			}
			task.Tags = v
		case "depends_on":
			var v []string
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				output.Error(ExitUsage, fmt.Sprintf("invalid JSON array: %s", value))
				os.Exit(ExitUsage)
				return nil
			}
			task.DependsOn = v
		case "source_sections":
			var v []string
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				output.Error(ExitUsage, fmt.Sprintf("invalid JSON array: %s", value))
				os.Exit(ExitUsage)
				return nil
			}
			task.SourceSections = v
		case "source_lines":
			task.SourceLines = value
		default:
			output.Error(ExitUsage, fmt.Sprintf("unknown field: %s", field))
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
