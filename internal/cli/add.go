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
	cmd.Flags().StringVar(&addFile, "bulk", "", "bulk add from NDJSON file")
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

	task, err := decodeTaskJSON(taskJSON)
	if err != nil {
		// §6.1 rule 7: invalid JSON is a usage error (exit 2); the decoder
		// detail goes in the hint, never in the error message.
		output.Error(ExitUsage, "invalid task JSON", err.Error())
		os.Exit(ExitUsage)
		return nil
	}

	return addTasks([]model.Task{*task})
}

// decodeTaskJSON parses a single task JSON value, defaulting empty status to
// "open". It does not perform §6.1 entry validation — the caller handles that
// once the whole batch is staged.
func decodeTaskJSON(s string) (*model.Task, error) {
	var task model.Task
	if err := json.Unmarshal([]byte(s), &task); err != nil {
		return nil, err
	}
	if task.Status == "" {
		task.Status = model.StatusOpen
	}
	return &task, nil
}

func runAddBulk() error {
	tasks := readBulkTasks(addFile)
	return addTasks(tasks)
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
		if strings.TrimSpace(line) == "" {
			continue
		}
		var task model.Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			// §6.1 rule 7: invalid JSON is a usage error (exit 2); the decoder
			// detail goes in the hint, never in the error message.
			output.Error(ExitUsage, fmt.Sprintf("line %d: invalid task JSON", lineNum), err.Error())
			os.Exit(ExitUsage)
			return nil
		}
		if task.Status == "" {
			task.Status = model.StatusOpen
		}
		tasks = append(tasks, task)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: stopped reading %s early (%v); tasks after the over-long line were dropped (line cap is 64KB)\n", path, err)
	}
	return tasks
}

// computeAccuracyRatio returns estimated/actual ratio from done tasks with timing data.
// Returns 0 if insufficient data.
func computeAccuracyRatio(tf *model.TaskFile) float64 {
	var totalEstimated, totalActual float64
	tracked := 0
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusDone || t.StartedAt == nil || t.ClosedAt == nil {
			continue
		}
		actual := t.ClosedAt.Sub(*t.StartedAt).Minutes()
		if actual <= 0 {
			continue
		}
		totalEstimated += float64(t.EstimateMinutes)
		totalActual += actual
		tracked++
	}
	if tracked < 3 || totalActual == 0 {
		return 0 // not enough data
	}
	return totalEstimated / totalActual
}

// addTasks stages a batch of one or more tasks into the active task file in a
// single locked write. Per spec §6.1, every task is validated at entry (the
// same seven rules tp import enforces) rather than deferred to tp validate:
//
//   - Rule 2 (duplicate id, against the file or earlier tasks in this batch) is
//     checked incrementally as tasks are walked.
//   - Rules 1, 3, 4, 5, 6 are delegated to engine.ValidateTaskEntry.
//   - Rule 7 (invalid JSON) was already rejected at decode time.
//
// depends_on resolution uses the file's ids PLUS every id in this batch, so a
// task may depend on one appearing later in the same --bulk/--stdin run.
func addTasks(tasks []model.Task) error {
	if len(tasks) == 0 {
		return nil
	}

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

		// Resolvable-id set = existing file ids + every id in this batch, so a
		// task may depend_on a later task staged in the same batch (§6.1).
		resolvable := make(map[string]bool, len(tf.Tasks)+len(tasks))
		for i := range tf.Tasks {
			resolvable[tf.Tasks[i].ID] = true
		}
		for i := range tasks {
			resolvable[tasks[i].ID] = true
		}

		// Existing ids grow incrementally to catch intra-batch duplicates.
		existing := make(map[string]bool, len(tf.Tasks))
		for i := range tf.Tasks {
			existing[tf.Tasks[i].ID] = true
		}

		for i := range tasks {
			t := &tasks[i]
			if existing[t.ID] {
				// §6.1 rule 2: duplicate id.
				output.Error(ExitValidation, fmt.Sprintf("duplicate task id: %s", t.ID),
					fmt.Sprintf("a task with id %q already exists; use tp set to update it", t.ID))
				os.Exit(ExitValidation)
				return nil
			}
			if f := engine.ValidateTaskEntry(t, resolvable); f != nil {
				output.Error(ExitValidation, f.Msg, f.Hint)
				os.Exit(ExitValidation)
				return nil
			}
			existing[t.ID] = true
		}

		// Estimation calibration: warn if historical accuracy suggests overestimation
		for i := range tasks {
			t := &tasks[i]
			if t.EstimateMinutes > 0 {
				if ratio := computeAccuracyRatio(tf); ratio > 2.0 {
					output.Info(fmt.Sprintf("estimation calibration: historical accuracy %.1fx — estimate %d min may be high, consider %d min",
						ratio, t.EstimateMinutes, max(1, int(float64(t.EstimateMinutes)/ratio))))
				}
			}
		}

		// Resolve the spec path once for both source_sections normalization and
		// the §7.1 coverage recompute below.
		specPath, specExists := engine.ResolveSpecPath(taskFilePath, tf.Spec)
		if specExists {
			if headings, perr := engine.ParseHeadings(specPath); perr == nil && len(headings) > 0 {
				if nerr := engine.NormalizeSourceSections(tasks, headings); nerr != nil {
					output.Error(ExitValidation, nerr.Error())
					os.Exit(ExitValidation)
					return nil
				}
			}
		}

		tf.Tasks = append(tf.Tasks, tasks...)

		// §7.1: recompute coverage now that the task set changed. AutoFillCoverage
		// no-ops when the spec can't be read, leaving the block untouched (§7.2).
		if specExists {
			engine.AutoFillCoverage(tf, specPath)
		}

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		ids := make([]string, len(tasks))
		for i := range tasks {
			ids[i] = tasks[i].ID
		}
		if len(ids) == 1 {
			output.Success(fmt.Sprintf("added %s", ids[0]))
		} else {
			output.Success(fmt.Sprintf("added %d tasks: %s", len(ids), strings.Join(ids, ", ")))
		}
		return output.JSON(map[string]any{"added": ids})
	})
}
