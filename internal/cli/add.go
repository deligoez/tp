package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	addStdin bool
	addFile  string
	addSpec  string
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [task-json]",
		Short: "Add a single task",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAdd,
	}
	cmd.Flags().BoolVar(&addStdin, "stdin", false, "read task JSON from stdin")
	cmd.Flags().StringVar(&addFile, "file", "", "bulk add from NDJSON file")
	cmd.Flags().StringVar(&addSpec, "spec", "", "spec path (required when creating new task file)")
	return cmd
}

func runAdd(_ *cobra.Command, args []string) error {
	if addFile != "" {
		return runAddBulk()
	}

	var taskJSON string
	switch {
	case len(args) > 0:
		taskJSON = args[0]
	case addStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("read stdin: %v", err))
			os.Exit(ExitFile)
			return nil
		}
		taskJSON = string(data)
	default:
		output.Error(ExitUsage, "task JSON required as argument or via --stdin")
		os.Exit(ExitUsage)
		return nil
	}

	var task model.Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		output.Error(ExitFile, fmt.Sprintf("invalid task JSON: %v", err))
		os.Exit(ExitFile)
		return nil
	}

	if task.Status == "" {
		task.Status = model.StatusOpen
	}

	return addTask(&task)
}

func runAddBulk() error {
	tasks := readBulkTasks(addFile)

	for i := range tasks {
		if err := addTask(&tasks[i]); err != nil {
			return err
		}
	}
	return nil
}

func readBulkTasks(path string) []model.Task {
	data, err := os.ReadFile(path)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var tasks []model.Task
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var task model.Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			output.Error(ExitFile, fmt.Sprintf("line %d: invalid JSON: %v", lineNum, err))
			os.Exit(ExitFile)
			return nil
		}
		if task.Status == "" {
			task.Status = model.StatusOpen
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func addTask(task *model.Task) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		if addSpec == "" {
			output.Error(ExitFile, "no task file found. Use --spec to create one, or run tp init first")
			os.Exit(ExitFile)
			return nil
		}
		// Create a new task file
		if initErr := runInit(nil, []string{addSpec}); initErr != nil {
			return initErr
		}
		taskFilePath, err = engine.DiscoverTaskFile(".", flagFile)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		// Check duplicate ID
		for i := range tf.Tasks {
			if tf.Tasks[i].ID == task.ID {
				output.Error(ExitState, fmt.Sprintf("task ID already exists: %s (use tp set to update)", task.ID))
				os.Exit(ExitState)
				return nil
			}
		}

		tf.Tasks = append(tf.Tasks, *task)

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("added %s", task.ID))
		return nil
	})
}
