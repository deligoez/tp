package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// addFoundOpen puts the number of open queue entries beside taskFilePath on a
// converged review or audit payload (spec/1.31.0.md §6), present at zero. It
// is a count for the reader and never part of the verdict: an open entry holds
// only its finders, so convergence reads as it would without it. A queue file
// that cannot be read or parsed is the error, not a count of zero.
func addFoundOpen(result map[string]any, taskFilePath string, converged bool) error {
	if !converged {
		return nil
	}
	open, err := engine.FoundOpenCount(taskFilePath)
	if err != nil {
		return err
	}
	result["found_open"] = open
	return nil
}

// noteFoundOpen is addFoundOpen for a --record payload: the round is already
// stored, so an unreadable queue is a notice and the count is left out,
// rather than an exit that reads as the record having failed.
func noteFoundOpen(result map[string]any, taskFilePath string, converged bool) {
	if err := addFoundOpen(result, taskFilePath, converged); err != nil {
		output.Notice(fmt.Sprintf("found_open omitted: cannot read found queue %s (%v)", engine.FoundQueuePath(taskFilePath), err))
	}
}

// exitFoundQueueError ends a --status read whose queue file cannot be read or
// parsed, as tp found list does, instead of reporting it as an empty queue.
func exitFoundQueueError(err error) {
	output.Error(ExitFile, fmt.Sprintf("cannot read found queue: %v", err), "fix or restore the queue file beside the task file")
	os.Exit(ExitFile)
}
