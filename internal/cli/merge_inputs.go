package cli

import (
	"fmt"
	"os"
	"path/filepath"
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

// writeMergeOutput writes `tp review --merge`'s NDJSON to `-o`, or refuses to
// touch that path at all when the merge is already going to exit non-zero (§5
// row 10). It reports whether it wrote, so the caller names `output_path` in
// the summary only when a file is actually there. `tp audit --merge` does not
// call it: §4 fences this release out of the audit phase, so that merge still
// writes before it refuses.
//
// The refusal is keyed on `dropped`, never on how many rows survived: a
// converged round's inputs hold no content line, drop nothing, and still get
// the zero-byte `-o` the review loop reads as "nothing found".
//
// The write itself goes to a temporary file in the destination's own directory
// and is renamed on success, so a failed write leaves neither a truncated `-o`
// nor the temporary beside it.
func writeMergeOutput(outputPath, ndjson string, dropped []string) bool {
	if len(dropped) > 0 {
		return false
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".tp-merge-*")
	if err != nil {
		failMergeOutput(err)
		return false
	}
	// One failure path, so the temporary file is removed however the write
	// ends: a partial temporary left beside `-o` is the thing this shape is
	// here to prevent.
	name := tmp.Name()
	_, err = tmp.WriteString(ndjson)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, outputPath)
	}
	if err != nil {
		_ = os.Remove(name)
		failMergeOutput(err)
		return false
	}
	return true
}

// failMergeOutput reports an unwritable `-o` at exit 3, the code and hint the
// single os.WriteFile call this replaced already used.
func failMergeOutput(err error) {
	output.Error(ExitFile, fmt.Sprintf("cannot write output file: %s", err), outputFileHint)
	os.Exit(ExitFile)
}

// finishMerge applies §8a.4's exit rule once the merge has emitted its payload,
// and both merges still reach it the same way. What changed under it is what a
// caller has already written: for `tp review --merge`, §5 row 10 makes the
// payload the summary alone, because `-o` is the file the next command in a
// review loop reads — so a refused review merge leaves that path exactly as it
// found it, writeMergeOutput having declined to write it.
//
// That supersedes §8a.4's stated rationale for the review side, which kept the
// write because "the accounting an operator reads is in that payload": the
// accounting reaches stdout either way, and it was the file, not the
// accounting, that let a refused merge be chained into `--record` as a clean
// round. The rationale still stands for `tp audit --merge`, which §4 fences
// out of this release and which therefore still writes `-o` before refusing.
//
// encodeErr is the payload's own write error and is reported first: a merge
// that could not emit has nothing to say about its inputs.
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
