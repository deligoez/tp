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
	doneAutoCommit bool
	doneCoveredBy  string
	doneFiles      string
)

func newDoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done <id> [reason]",
		Short: "Close task with verification (preferred over tp close)",
		Long: `Close task(s) with closure verification. Implicitly claims open tasks.
Single-ID output: {closed: "id", remaining: {...}, has_next: bool}
Multi-ID output:  {closed: ["id1","id2"], failed: [...], remaining: {...}, has_next: bool}
On error: {error, code, acceptance, hint} on stderr. Task unchanged.`,
		Example: `  tp done auth-model "evidence"
  tp done auth-model "evidence" --gate-passed --commit abc123
  tp done task1 task2 task3 "shared evidence"     # multi-ID
  tp done --batch results.ndjson                  # NDJSON batch`,
		Args: cobra.ArbitraryArgs,
		RunE: runDone,
	}
	cmd.Flags().BoolVar(&doneStdin, "stdin", false, "read reason from stdin")
	cmd.Flags().StringVar(&doneReasonFile, "reason-file", "", "read reason from file")
	cmd.Flags().BoolVar(&doneGatePassed, "gate-passed", false, "attest quality gate passed")
	cmd.Flags().StringVar(&doneCommit, "commit", "", "record implementing commit SHA")
	cmd.Flags().StringVar(&doneBatch, "batch", "", "batch close from NDJSON file")
	cmd.Flags().BoolVar(&doneAutoCommit, "auto-commit", false, "stage + commit before closing (structured message)")
	cmd.Flags().StringVar(&doneCoveredBy, "covered-by", "", "close as covered by another done task (skips closure verification)")
	cmd.Flags().StringVar(&doneFiles, "files", "", "file globs to stage for --auto-commit (default: all changes)")
	return cmd
}

func runDone(_ *cobra.Command, args []string) error {
	// --batch mode: mutually exclusive with positional args
	if doneBatch != "" {
		if len(args) > 0 {
			output.Error(ExitUsage, "--batch is mutually exclusive with positional task IDs")
			os.Exit(ExitUsage)
			return nil
		}
		return runDoneBatch()
	}

	if len(args) < 1 {
		output.Error(ExitUsage, "task ID required")
		os.Exit(ExitUsage)
		return nil
	}

	// Parse task IDs and reason
	taskIDs, reason, err := resolveMultiReason(args, doneStdin, doneReasonFile)
	if err != nil {
		output.Error(ExitUsage, err.Error())
		os.Exit(ExitUsage)
		return nil
	}

	// --auto-commit forbidden with multi-ID
	if len(taskIDs) > 1 && doneAutoCommit {
		output.Error(ExitUsage, "--auto-commit is not supported with multiple task IDs. Use tp done --batch for multi-task auto-commit.")
		os.Exit(ExitUsage)
		return nil
	}

	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	if len(taskIDs) == 1 {
		return runDoneSingle(taskFilePath, taskIDs[0], reason)
	}
	return runDoneMulti(taskFilePath, taskIDs, reason)
}

// runDoneSingle handles the single-ID case with backward-compatible output.
func runDoneSingle(taskFilePath, taskID, reason string) error {
	return engine.WithFileLock(taskFilePath, func() error {
		tf, readErr := model.ReadTaskFile(taskFilePath)
		if readErr != nil {
			output.Error(ExitFile, readErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		task, _, findErr := model.FindTask(tf, taskID)
		if findErr != nil {
			output.Error(ExitState, findErr.Error())
			os.Exit(ExitState)
			return nil
		}

		// Implicit claim: open -> wip -> done
		if task.Status == model.StatusOpen {
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
			claimTime := time.Now().UTC()
			task.Status = model.StatusWIP
			task.StartedAt = &claimTime
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

		isCoveredBy := doneCoveredBy != ""
		if isCoveredBy {
			ref, _, refErr := model.FindTask(tf, doneCoveredBy)
			if refErr != nil {
				output.Error(ExitState, fmt.Sprintf("--covered-by: %v", refErr))
				os.Exit(ExitState)
				return nil
			}
			if ref.Status != model.StatusDone {
				output.Error(ExitState, fmt.Sprintf("--covered-by: task %s is %s (must be done)", ref.ID, ref.Status))
				os.Exit(ExitState)
				return nil
			}
		}

		if verifyErr := engine.VerifyClosure(task.Acceptance, reason, doneGatePassed, isCoveredBy); verifyErr != nil {
			errOut := map[string]any{
				"error":      fmt.Sprintf("closure verification failed: %v", verifyErr),
				"code":       ExitValidation,
				"acceptance": task.Acceptance,
				"hint":       "Rewrite reason to address all acceptance criteria, then retry tp done. Use --gate-passed to relax keyword matching.",
			}
			data, _ := json.Marshal(errOut)
			fmt.Fprintln(os.Stderr, string(data))
			os.Exit(ExitValidation)
			return nil
		}

		if doneAutoCommit && doneCommit == "" {
			if err := gitStage(doneFiles); err != nil {
				output.Error(ExitFile, fmt.Sprintf("auto-commit: git stage failed: %v", err))
				os.Exit(ExitFile)
				return nil
			}
			if gitHasStagedChanges() {
				msg := buildCommitMessage(task, reason)
				sha, commitErr := gitCommit(msg)
				if commitErr != nil {
					output.Error(ExitFile, fmt.Sprintf("auto-commit: git commit failed: %v", commitErr))
					os.Exit(ExitFile)
					return nil
				}
				doneCommit = sha
			}
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

		result := map[string]any{
			"closed": task.ID,
			"remaining": map[string]any{
				"total": len(tf.Tasks),
				"open":  openCount,
				"wip":   wipCount,
				"done":  len(tf.Tasks) - openCount - wipCount,
				"ready": readyCount,
			},
			"has_next": readyCount > 0,
		}

		if openCount == 0 && wipCount == 0 {
			_, summary := computeReport(tf)
			result["report"] = summary
		}

		return output.JSON(result)
	})
}

// runDoneMulti handles the multi-ID case with array output.
func runDoneMulti(taskFilePath string, taskIDs []string, reason string) error {
	return engine.WithFileLock(taskFilePath, func() error {
		tf, readErr := model.ReadTaskFile(taskFilePath)
		if readErr != nil {
			output.Error(ExitFile, readErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		closedIDs := make([]string, 0)
		failed := make([]map[string]any, 0)
		now := time.Now().UTC()
		isCoveredBy := doneCoveredBy != ""

		// Build done set for dep checking (updated as tasks close)
		doneSet := make(map[string]bool)
		for i := range tf.Tasks {
			if tf.Tasks[i].Status == model.StatusDone {
				doneSet[tf.Tasks[i].ID] = true
			}
		}

		for _, id := range taskIDs {
			task, _, findErr := model.FindTask(tf, id)
			if findErr != nil {
				failed = append(failed, map[string]any{"id": id, "error": findErr.Error()})
				continue
			}

			// Implicit claim: open -> wip
			if task.Status == model.StatusOpen {
				blocked := false
				for _, dep := range task.DependsOn {
					if !doneSet[dep] {
						failed = append(failed, map[string]any{
							"id":    id,
							"error": fmt.Sprintf("blocked by %s", dep),
							"hint":  fmt.Sprintf("Close %s first or place it earlier in the argument list.", dep),
						})
						blocked = true
						break
					}
				}
				if blocked {
					continue
				}
				task.Status = model.StatusWIP
				task.StartedAt = &now
			}

			if task.Status == model.StatusDone {
				failed = append(failed, map[string]any{"id": id, "error": fmt.Sprintf("task %s is already done", id)})
				continue
			}

			if task.Status != model.StatusWIP {
				failed = append(failed, map[string]any{"id": id, "error": fmt.Sprintf("cannot done: task %s is %s", id, task.Status)})
				continue
			}

			// Verify covered-by reference if provided
			if isCoveredBy {
				ref, _, refErr := model.FindTask(tf, doneCoveredBy)
				if refErr != nil || ref.Status != model.StatusDone {
					errMsg := ""
					if refErr != nil {
						errMsg = fmt.Sprintf("--covered-by: %v", refErr)
					} else {
						errMsg = fmt.Sprintf("--covered-by: task %s is %s (must be done)", ref.ID, ref.Status)
					}
					failed = append(failed, map[string]any{"id": id, "error": errMsg})
					continue
				}
			}

			// Closure verification
			if verifyErr := engine.VerifyClosure(task.Acceptance, reason, doneGatePassed, isCoveredBy); verifyErr != nil {
				failed = append(failed, map[string]any{
					"id":         id,
					"error":      fmt.Sprintf("closure verification failed: %v", verifyErr),
					"acceptance": task.Acceptance,
					"hint":       "Rewrite reason to address all acceptance criteria.",
				})
				continue
			}

			// Close the task
			task.Status = model.StatusDone
			task.ClosedAt = &now
			r := reason
			task.ClosedReason = &r
			if doneGatePassed {
				task.GatePassedAt = &now
			}
			if doneCommit != "" {
				c := doneCommit
				task.CommitSHA = &c
			}
			closedIDs = append(closedIDs, id)
			doneSet[id] = true
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
			}
		}

		result := map[string]any{
			"closed": closedIDs,
			"failed": failed,
			"remaining": map[string]any{
				"total": len(tf.Tasks),
				"open":  openCount,
				"wip":   wipCount,
				"done":  len(tf.Tasks) - openCount - wipCount,
				"ready": readyCount,
			},
			"has_next": readyCount > 0,
		}

		if openCount == 0 && wipCount == 0 {
			_, summary := computeReport(tf)
			result["report"] = summary
		}

		if jsonErr := output.JSON(result); jsonErr != nil {
			output.Error(ExitFile, jsonErr.Error())
		}

		// Exit code: 0 if any closed, 1 if all failed
		if len(closedIDs) == 0 {
			os.Exit(ExitValidation)
		}
		return nil
	})
}

type batchEntry struct {
	ID         string     `json:"id"`
	Reason     string     `json:"reason"`
	GatePassed bool       `json:"gate_passed"`
	Commit     string     `json:"commit"`
	StartedAt  *time.Time `json:"started_at"`
	CoveredBy  string     `json:"covered_by"`
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
				if entry.StartedAt != nil {
					task.StartedAt = entry.StartedAt
				} else {
					task.StartedAt = &now
				}
			}

			// Verify covered-by reference if provided
			isBatchCoveredBy := entry.CoveredBy != ""
			if isBatchCoveredBy {
				ref, _, refErr := model.FindTask(tf, entry.CoveredBy)
				if refErr != nil || ref.Status != model.StatusDone {
					if refErr != nil {
						failures = append(failures, batchFailure{ID: entry.ID, Error: fmt.Sprintf("covered_by: %v", refErr)})
					} else {
						failures = append(failures, batchFailure{ID: entry.ID, Error: fmt.Sprintf("covered_by: task %s is %s (must be done)", ref.ID, ref.Status)})
					}
					continue
				}
			}

			// Closure verification
			if verifyErr := engine.VerifyClosure(task.Acceptance, entry.Reason, entry.GatePassed, isBatchCoveredBy); verifyErr != nil {
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

		// Auto-report when all tasks are done
		if openCount == 0 && wipCount == 0 {
			_, summary := computeReport(tf)
			result["report"] = summary
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

// resolveMultiReason parses args into task IDs and reason.
// When --stdin or --reason-file is set, ALL args are task IDs.
// Otherwise, last arg is the reason, all preceding are task IDs.
func resolveMultiReason(args []string, useStdin bool, reasonFile string) ([]string, string, error) {
	if useStdin && reasonFile != "" {
		return nil, "", fmt.Errorf("--stdin and --reason-file are mutually exclusive")
	}

	if useStdin {
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			return nil, "", fmt.Errorf("read stdin: %w", readErr)
		}
		return args, string(data), nil
	}

	if reasonFile != "" {
		data, readErr := os.ReadFile(reasonFile)
		if readErr != nil {
			return nil, "", fmt.Errorf("read reason file: %w", readErr)
		}
		return args, string(data), nil
	}

	// Positional: last arg is reason, rest are task IDs
	if len(args) < 2 {
		return nil, "", fmt.Errorf("reason is required (use positional reason, --stdin, or --reason-file)")
	}
	return args[:len(args)-1], args[len(args)-1], nil
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
