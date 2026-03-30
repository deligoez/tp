package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	doneStdin      bool
	doneReasonFile string
	doneGatePassed bool
	doneCommit     string
	doneBatch      string
)

func newDoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done <id> [reason]",
		Short: "Close task with verification (preferred over tp close)",
		Args:  cobra.RangeArgs(0, 2),
		RunE:  runDone,
	}
	cmd.Flags().BoolVar(&doneStdin, "stdin", false, "read reason from stdin")
	cmd.Flags().StringVar(&doneReasonFile, "reason-file", "", "read reason from file")
	cmd.Flags().BoolVar(&doneGatePassed, "gate-passed", false, "attest quality gate passed")
	cmd.Flags().StringVar(&doneCommit, "commit", "", "record implementing commit SHA")
	cmd.Flags().StringVar(&doneBatch, "batch", "", "batch close from NDJSON file")
	return cmd
}

func runDone(_ *cobra.Command, args []string) error {
	if doneBatch != "" {
		return runDoneBatch()
	}

	if len(args) < 1 {
		output.Error(ExitUsage, "task ID required")
		os.Exit(ExitUsage)
		return nil
	}

	// Determine reason
	reason, err := resolveReason(args, doneStdin, doneReasonFile)
	if err != nil {
		output.Error(ExitUsage, err.Error())
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
		tf, readErr := model.ReadTaskFile(taskFilePath)
		if readErr != nil {
			output.Error(ExitFile, readErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		task, _, findErr := model.FindTask(tf, args[0])
		if findErr != nil {
			output.Error(ExitState, findErr.Error())
			os.Exit(ExitState)
			return nil
		}

		// Implicit claim: open -> wip -> done
		if task.Status == model.StatusOpen {
			// Check deps
			done := make(map[string]bool)
			for i := range tf.Tasks {
				if tf.Tasks[i].Status == model.StatusDone {
					done[tf.Tasks[i].ID] = true
				}
			}
			for _, dep := range task.DependsOn {
				if !done[dep] {
					output.Error(ExitState, fmt.Sprintf("cannot done: task %s is blocked by %s", task.ID, dep))
					os.Exit(ExitState)
					return nil
				}
			}
			task.Status = model.StatusWIP
		}

		if task.Status == model.StatusDone {
			output.Error(ExitState, fmt.Sprintf("task %s is already done", task.ID))
			os.Exit(ExitState)
			return nil
		}

		if task.Status != model.StatusWIP {
			output.Error(ExitState, fmt.Sprintf("cannot done: task %s is %s", task.ID, task.Status))
			os.Exit(ExitState)
			return nil
		}

		// Closure verification
		if verifyErr := engine.VerifyClosure(task.Acceptance, reason); verifyErr != nil {
			errOut := map[string]any{
				"error":      fmt.Sprintf("closure verification failed: %v", verifyErr),
				"code":       ExitValidation,
				"acceptance": task.Acceptance,
				"hint":       "Rewrite reason to address all acceptance criteria, then retry tp done.",
			}
			data, _ := json.Marshal(errOut)
			fmt.Fprintln(os.Stderr, string(data))
			os.Exit(ExitValidation)
			return nil
		}

		now := time.Now().UTC()
		task.Status = model.StatusDone
		task.ClosedAt = &now
		task.ClosedReason = &reason
		if doneGatePassed {
			task.GatePassedAt = &now
		}
		if doneCommit != "" {
			task.CommitSHA = &doneCommit
		}
		tf.UpdatedAt = now

		if writeErr := model.WriteTaskFile(taskFilePath, tf); writeErr != nil {
			output.Error(ExitFile, writeErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		if !doneGatePassed {
			output.Info("quality gate not attested. Consider using --gate-passed.")
		}

		// Compute has_next
		doneSet := make(map[string]bool)
		for i := range tf.Tasks {
			if tf.Tasks[i].Status == model.StatusDone {
				doneSet[tf.Tasks[i].ID] = true
			}
		}
		readyCount := 0
		openCount := 0
		wipCount := 0
		for i := range tf.Tasks {
			switch tf.Tasks[i].Status {
			case model.StatusOpen:
				openCount++
				allDone := true
				for _, dep := range tf.Tasks[i].DependsOn {
					if !doneSet[dep] {
						allDone = false
						break
					}
				}
				if allDone {
					readyCount++
				}
			case model.StatusWIP:
				wipCount++
			}
		}

		return output.JSON(map[string]any{
			"closed": task.ID,
			"remaining": map[string]any{
				"total": len(tf.Tasks),
				"open":  openCount,
				"wip":   wipCount,
				"done":  len(tf.Tasks) - openCount - wipCount,
				"ready": readyCount,
			},
			"has_next": readyCount > 0,
		})
	})
}

type batchEntry struct {
	ID         string `json:"id"`
	Reason     string `json:"reason"`
	GatePassed bool   `json:"gate_passed"`
	Commit     string `json:"commit"`
}

type batchFailure struct {
	ID         string `json:"id"`
	Error      string `json:"error"`
	Acceptance string `json:"acceptance,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

func runDoneBatch() error {
	// Read NDJSON file
	entries, err := readBatchEntries(doneBatch)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, readErr := model.ReadTaskFile(taskFilePath)
		if readErr != nil {
			output.Error(ExitFile, readErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		closedCount := 0
		var failures []batchFailure
		now := time.Now().UTC()

		// Build done set for dep checking
		doneSet := make(map[string]bool)
		for i := range tf.Tasks {
			if tf.Tasks[i].Status == model.StatusDone {
				doneSet[tf.Tasks[i].ID] = true
			}
		}

		for _, entry := range entries {
			task, _, findErr := model.FindTask(tf, entry.ID)
			if findErr != nil {
				failures = append(failures, batchFailure{ID: entry.ID, Error: findErr.Error()})
				continue
			}

			// Skip already done (idempotent)
			if task.Status == model.StatusDone {
				closedCount++ // count as success for idempotent retries
				continue
			}

			// Implicit claim
			if task.Status == model.StatusOpen {
				blocked := false
				for _, dep := range task.DependsOn {
					if !doneSet[dep] {
						failures = append(failures, batchFailure{
							ID:    entry.ID,
							Error: fmt.Sprintf("blocked by %s", dep),
							Hint:  fmt.Sprintf("Close %s first.", dep),
						})
						blocked = true
						break
					}
				}
				if blocked {
					continue
				}
				task.Status = model.StatusWIP
			}

			// Closure verification
			if verifyErr := engine.VerifyClosure(task.Acceptance, entry.Reason); verifyErr != nil {
				failures = append(failures, batchFailure{
					ID:         entry.ID,
					Error:      fmt.Sprintf("closure verification failed: %v", verifyErr),
					Acceptance: task.Acceptance,
					Hint:       "Fix reason to address all acceptance criteria.",
				})
				continue
			}

			task.Status = model.StatusDone
			task.ClosedAt = &now
			reason := entry.Reason
			task.ClosedReason = &reason
			if entry.GatePassed {
				task.GatePassedAt = &now
			}
			if entry.Commit != "" {
				commit := entry.Commit
				task.CommitSHA = &commit
			}
			closedCount++
			doneSet[task.ID] = true // update for subsequent dep checks
		}

		tf.UpdatedAt = now
		if writeErr := model.WriteTaskFile(taskFilePath, tf); writeErr != nil {
			output.Error(ExitFile, writeErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		// Compute remaining
		openCount := 0
		wipCount := 0
		doneCount := 0
		readyCount := 0
		for i := range tf.Tasks {
			switch tf.Tasks[i].Status {
			case model.StatusOpen:
				openCount++
				allDone := true
				for _, dep := range tf.Tasks[i].DependsOn {
					if !doneSet[dep] {
						allDone = false
						break
					}
				}
				if allDone {
					readyCount++
				}
			case model.StatusWIP:
				wipCount++
			case model.StatusDone:
				doneCount++
			}
		}

		result := map[string]any{
			"closed": closedCount,
			"failed": len(failures),
			"remaining": map[string]any{
				"total": len(tf.Tasks),
				"open":  openCount,
				"wip":   wipCount,
				"done":  doneCount,
				"ready": readyCount,
			},
		}
		if len(failures) > 0 {
			result["failures"] = failures
		}

		if jsonErr := output.JSON(result); jsonErr != nil {
			output.Error(ExitFile, jsonErr.Error())
		}

		if len(failures) == len(entries) {
			os.Exit(ExitState)
		} else if len(failures) > 0 {
			os.Exit(ExitValidation)
		}
		return nil
	})
}

func resolveReason(args []string, useStdin bool, reasonFile string) (string, error) {
	sources := 0
	if len(args) > 1 {
		sources++
	}
	if useStdin {
		sources++
	}
	if reasonFile != "" {
		sources++
	}
	if sources > 1 {
		return "", fmt.Errorf("multiple reason sources. Use exactly one: positional argument, --stdin, or --reason-file")
	}
	if sources == 0 {
		return "", fmt.Errorf("reason is required")
	}

	switch {
	case len(args) > 1:
		return args[1], nil
	case useStdin:
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			return "", fmt.Errorf("read stdin: %w", readErr)
		}
		return string(data), nil
	default:
		data, readErr := os.ReadFile(reasonFile)
		if readErr != nil {
			return "", fmt.Errorf("read reason file: %w", readErr)
		}
		return string(data), nil
	}
}

func readBatchEntries(path string) ([]batchEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []batchEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e batchEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("invalid NDJSON line: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
