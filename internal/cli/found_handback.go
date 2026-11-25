package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// foundHandbackHint says which tasks a handback takes.
const foundHandbackHint = "tp found handback takes a found task (one tp found apply opened, carrying origin) that is not done, and the reason its reproduction did not fail; any other task closes with tp done"

func newFoundHandbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "handback <id> <why>",
		Short: "Hand a found task back when its reproduction does not fail",
		Long: `Close a found task whose reproduction does not fail for the stated reason.

The task closes as done, marked handed_back, with the reason as its closed
reason, and an open queue entry holds its request. Every task not done that
depends on it is a finder of that entry and waits, out of the ready set,
until the entry is settled. A task without origin exits 2 and nothing is
written.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runFoundHandback(args[0], args[1])
		},
	}
}

// runFoundHandback implements tp found handback (spec/1.31.0.md §3).
func runFoundHandback(id, why string) error {
	if strings.TrimSpace(why) == "" {
		return foundRefusal("a handback needs the reason the reproduction did not fail", foundHandbackHint)
	}
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file resolved, so there is no task to hand back.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	entry, err := handBackFoundTask(root, taskFilePath, id, why)
	if errors.Is(err, errFoundAnswerRefused) {
		return foundRefusal(strings.TrimSuffix(err.Error(), ": "+errFoundAnswerRefused.Error()), foundHandbackHint)
	}
	if err != nil {
		output.Error(ExitFile, "cannot hand the task back: "+err.Error(), foundStoreWriteHint)
		os.Exit(ExitFile)
		return nil
	}
	return output.JSON(map[string]any{"handed_back": id, "entry": entry.ID, "finders": entry.Finders})
}

// handBackFoundTask closes a found task as handed back and appends its queue
// entry, both under the task-file lock. A task that is not a found task, or
// is already done, is refused (errFoundAnswerRefused) and nothing is written.
func handBackFoundTask(root, taskFilePath, id, why string) (engine.FoundQueueEntry, error) {
	var entry engine.FoundQueueEntry
	reqs, err := engine.ReadFoundRequests(root, taskFilePath)
	if err != nil {
		return entry, err
	}
	err = engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			return err
		}
		task, _, err := model.FindTask(tf, id)
		if err != nil {
			return fmt.Errorf("%w: %w", err, errFoundAnswerRefused)
		}
		if task.Origin == nil {
			return fmt.Errorf("task %s has no origin, so it is not a found task: %w", id, errFoundAnswerRefused)
		}
		if task.Status == model.StatusDone {
			return fmt.Errorf("task %s is already done: %w", id, errFoundAnswerRefused)
		}
		queue, err := engine.ReadFoundQueue(taskFilePath)
		if err != nil {
			return err
		}
		entry = engine.FoundHandbackEntry(task, tf.Tasks, reqs)

		now := time.Now().UTC()
		reason := "Handed back: " + why
		task.Status = model.StatusDone
		task.HandedBack = true
		task.ClosedAt = &now
		task.ClosedReason = &reason
		task.Waiting = ""
		task.ClaimedBy = task.ClaimedBy.Released()
		tf.UpdatedAt = now
		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			return err
		}
		queue = append(queue, entry)
		if err := engine.WriteFoundQueue(taskFilePath, queue); err != nil {
			return err
		}
		entry = queue[len(queue)-1]
		return nil
	})
	return entry, err
}
