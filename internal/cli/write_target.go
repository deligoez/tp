package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// noticeCandidateCap bounds how many other task files the pointer notice
// names. A repository that keeps one task file per shipped spec holds dozens,
// and the notice fires on every pointer write there; the count of the rest
// still says the choice was not the only one.
const noticeCandidateCap = 3

// discoverWriteTarget resolves the task file a write command modifies, on the
// chain every command uses (--file > TP_FILE > .tp/local.json active >
// auto-detect). A pointer set by `tp use` outlives the spec it was set for:
// after `tp init` of another spec, a write resolved through it rewrote the old
// task file with exit 0 and nothing in the output naming the file. So when the
// pointer is what chose the file and another task file is in reach, one line
// on stderr names the file, the pointer and the other candidates. The
// payload's "file" key (taskFileLabel) names the file on every write, whoever
// chose it.
func discoverWriteTarget() (string, error) {
	path, viaPointer, err := engine.DiscoverTaskFileVia(".", flagFile)
	if err != nil {
		return "", err
	}
	if viaPointer {
		if others := engine.OtherTaskFiles(".", path); len(others) > 0 {
			output.Notice(fmt.Sprintf("notice: writing %s because the .tp/local.json active pointer names it; other task files: %s — pass --file to write another",
				taskFileLabel(path), candidateList(others)))
		}
	}
	return path, nil
}

// candidateList names up to noticeCandidateCap paths and counts the rest.
func candidateList(paths []string) string {
	shown := make([]string, 0, noticeCandidateCap)
	for _, p := range paths[:min(len(paths), noticeCandidateCap)] {
		shown = append(shown, taskFileLabel(p))
	}
	list := strings.Join(shown, ", ")
	if rest := len(paths) - len(shown); rest > 0 {
		list += fmt.Sprintf(" and %d more", rest)
	}
	return list
}

// warnPointerNamesAnother is for the two writes that pick their task file from
// a spec path rather than through discovery, tp init and tp import. The pointer
// does not choose their file, but it chooses the file of every write after
// them, so a pointer naming a different existing task file is said once, here,
// where the mismatch is created. The pointer is left where it is: it is
// per-checkout state another agent in the same checkout may rely on.
func warnPointerNamesAnother(written string) {
	active := engine.ResolveLocalActive(".")
	if active == "" {
		return
	}
	if _, err := os.Stat(active); err != nil {
		return // dangling: discovery falls through to auto-detect, and says so
	}
	if absPath(active) == absPath(written) {
		return
	}
	output.Notice(fmt.Sprintf("warning: the .tp/local.json active pointer names %s, not %s; later writes without --file go to %s (tp use %s to switch)",
		taskFileLabel(active), taskFileLabel(written), taskFileLabel(active), taskFileLabel(written)))
}

// taskFileLabel is the form a write payload's "file" key carries: the path
// relative to the directory the command ran in, so the value works as
// --file's argument from there. The pointer's stored value is relative to the
// project root instead, and misses when the command runs in a subdirectory.
func taskFileLabel(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(wd, path)
	if err != nil {
		return path
	}
	return rel
}

func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// taskWritten is a task echoed by a write command (tp set, tp claim, tp close)
// together with the task file it was written to. The embedded pointer's fields
// are promoted, so the payload keeps the task's own shape plus "file".
type taskWritten struct {
	*model.Task
	File string `json:"file"`
}
