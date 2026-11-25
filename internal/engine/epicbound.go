package engine

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// UnreadableTaskFileError is a task file that cannot be read or parsed. Under
// the bound-file walk it may be bound to any epic, so no epic's bound files,
// and no status derived from them, can be known.
type UnreadableTaskFileError struct {
	Path string
	Err  error
}

func (e *UnreadableTaskFileError) Error() string {
	return fmt.Sprintf("task file %s cannot be read, so the epic it binds is unknown: %v", e.Path, e.Err)
}

func (e *UnreadableTaskFileError) Unwrap() error { return e.Err }

// EpicBoundFiles indexes the task files bound to each epic (spec/1.5.0.md §2):
// every tracked or untracked *.tasks.json under the repository root discovered
// from start, ignore rules respected, keyed by the epic its `epic` field names.
// Paths are absolute and sorted within each epic. A file with no epic is in no
// list. A file that cannot be read or parsed may bind any epic, so it is an
// *UnreadableTaskFileError naming the first such path, with no index. When
// start is not inside a git repository, or git cannot be run, the index is
// empty.
func EpicBoundFiles(start string) (map[string][]string, error) {
	index, _, skipped := EpicBoundTaskFiles(start)
	if len(skipped) > 0 {
		return nil, skipped[0]
	}
	return index, nil
}

// EpicBoundTaskFiles is EpicBoundFiles for a reader that would rather report
// what it can than nothing: it skips each task file that cannot be read or
// parsed, naming it in skipped, and returns the index of the readable ones
// with each bound file's parsed contents, keyed by path, so the reader need
// not read them again.
func EpicBoundTaskFiles(start string) (index map[string][]string, files map[string]*model.TaskFile, skipped []*UnreadableTaskFileError) {
	index, files = map[string][]string{}, map[string]*model.TaskFile{}
	root := gitToplevel(start)
	if root == "" {
		return index, files, nil
	}
	for _, path := range repoTaskFiles(root) {
		tf, err := model.ReadTaskFile(path)
		if err != nil {
			skipped = append(skipped, &UnreadableTaskFileError{Path: path, Err: err})
			continue
		}
		if tf.Epic != "" {
			index[tf.Epic] = append(index[tf.Epic], path)
			files[path] = tf
		}
	}
	for _, paths := range index {
		sort.Strings(paths)
	}
	return index, files, skipped
}

// repoTaskFiles returns the absolute paths of the *.tasks.json files git sees
// under root: tracked ones and untracked ones its ignore rules do not exclude.
// A path git lists twice (an unmerged entry's stages) is returned once.
func repoTaskFiles(root string) []string {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.tasks.json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	for rel := range strings.SplitSeq(string(out), "\x00") {
		if rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		paths = append(paths, filepath.Join(root, filepath.FromSlash(rel)))
	}
	return paths
}
