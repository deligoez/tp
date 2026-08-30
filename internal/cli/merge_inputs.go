package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/output"
)

// mergeInputCounts is one input file's share of a merge (§8a.4): how many of its
// content lines became a usable row, and how many were skipped as malformed or
// incomplete. Blank and whitespace-only lines are neither parsed nor skipped, so
// a file padded with newlines never reads as a dropped role.
type mergeInputCounts struct {
	Path    string `json:"path"`
	Parsed  int    `json:"parsed"`
	Skipped int    `json:"skipped"`
}

// droppedInputHint answers the one thing an operator can act on here: the file
// is where they said it was and tp read all of it, so the repair is in whatever
// wrote the file. The exit-1 default ("run 'tp validate' to audit the task
// file") names a file this mode never touches.
const droppedInputHint = "the input parsed as zero rows — re-emit it as one JSON object per line (a trailing comma or a wrapping array breaks every line at once), then merge again"

// droppedInputs names the inputs that had at least one content line and parsed
// none of them — the shape a whole role's file takes when its emitter got the
// format wrong, which used to merge clean and freeze an undercounted round
// (§8a.4). A zero-byte or blank-only file has nothing to drop and is never
// named: that stays the documented way a role reports nothing found.
func droppedInputs(inputs []mergeInputCounts) []string {
	dropped := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if in.Parsed == 0 && in.Skipped > 0 {
			dropped = append(dropped, in.Path)
		}
	}
	return dropped
}

// finishMerge applies §8a.4's exit rule after the merge has already emitted its
// payload: the surviving roles still merge and `-o` is still written, because
// the accounting an operator reads is in that payload — only the exit code
// changes, since an unattended driver reads nothing else. encodeErr is the
// payload's own write error and is reported first: a merge that could not emit
// has nothing to say about its inputs.
func finishMerge(encodeErr error, dropped []string) error {
	if encodeErr != nil {
		return encodeErr
	}
	if len(dropped) == 0 {
		return nil
	}
	output.Error(ExitValidation, fmt.Sprintf(
		"no line parsed in %s: every content line was skipped, so that input contributed nothing to the merge",
		strings.Join(dropped, ", "),
	), droppedInputHint)
	os.Exit(ExitValidation)
	return nil
}
