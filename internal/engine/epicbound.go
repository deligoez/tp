package engine

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicBoundFiles indexes the task files bound to each epic (spec/1.5.0.md §2):
// every tracked or untracked *.tasks.json under the repository root discovered
// from start, ignore rules respected, keyed by the epic its `epic` field names.
// Paths are absolute and sorted within each epic. A file with no epic is in no
// list, and one that cannot be read or parsed binds nothing. When start is not
// inside a git repository, or git cannot be run, the index is empty.
func EpicBoundFiles(start string) map[string][]string {
	index := map[string][]string{}
	root := gitToplevel(start)
	if root == "" {
		return index
	}
	for _, path := range repoTaskFiles(root) {
		tf, err := model.ReadTaskFile(path)
		if err != nil || tf.Epic == "" {
			continue
		}
		index[tf.Epic] = append(index[tf.Epic], path)
	}
	for _, paths := range index {
		sort.Strings(paths)
	}
	return index
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
