package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LandRequest is a worktree agent's request to close a task a hand-driven
// coordinator claimed (spec/backlog/worktree-land.md): the agent's commits
// and evidence, waiting for tp land in the coordinator's tree to pick the
// commits onto its branch and close the task there.
type LandRequest struct {
	ID       string   `json:"id"`
	Reason   string   `json:"reason"`
	Commits  []string `json:"commits"`
	Worktree string   `json:"worktree"`
	At       string   `json:"at"`
}

// LandDir is where the land requests of the task file at taskFile live: in
// the git common directory, so every worktree reads the same requests and
// removing the agent's worktree removes none, keyed by the task file's path
// relative to its worktree's top level. It is "" outside a git repository.
func LandDir(taskFile string) string {
	abs, err := filepath.Abs(taskFile)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	top, common := gitToplevel(dir), gitCommonDir(dir)
	if top == "" || common == "" {
		return ""
	}
	return LandDirFor(common, repoRelativeTo(top, abs))
}

// LandDirFor is LandDir from a git common directory and a task file's path
// relative to its worktree's top level.
func LandDirFor(common, rel string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(rel)))
	return filepath.Join(common, "tp", "land", hex.EncodeToString(sum[:8]))
}

// WriteLandRequest records r for the task file at taskFile, replacing an
// earlier request for the same task.
func WriteLandRequest(taskFile string, r *LandRequest) error {
	dir := LandDir(taskFile)
	if dir == "" {
		return fmt.Errorf("a land request needs a git repository")
	}
	record := *r
	if record.At == "" {
		record.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, record.ID+".json"), append(data, '\n'))
}

// ReadLandRequests returns the land requests in dir in id order; none is an
// empty slice. A file that cannot be parsed is an error: a request tp land
// cannot read is finished work it must not silently drop.
func ReadLandRequests(dir string) ([]LandRequest, error) {
	out := []LandRequest{}
	if dir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var r LandRequest
		if jsonErr := json.Unmarshal(data, &r); jsonErr != nil {
			return nil, fmt.Errorf("land request %s: %w", e.Name(), jsonErr)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// HasLandRequest reports whether dir holds a land request for id.
func HasLandRequest(dir, id string) bool {
	return dir != "" && fileExists(filepath.Join(dir, id+".json"))
}

// LandBlockedError is a pick the coordinator's own uncommitted changes stand
// in the way of: they touch Paths, which the commits change too.
type LandBlockedError struct{ Paths []string }

func (e *LandBlockedError) Error() string {
	return "uncommitted changes in " + strings.Join(e.Paths, ", ") + " stand in the way of the pick"
}

// LandPick puts a land request's commits on the branch checked out at root
// and returns the HEAD it started from and the commits now carrying the work.
// Commits already on HEAD (a hand merge of the same commits, or a pick a
// crash left behind) are not picked again. A commit that no longer exists, or
// a pick that conflicts, is an error with HEAD restored; uncommitted changes
// in the paths the commits change are a *LandBlockedError, before anything is
// picked.
func LandPick(root string, commits []string) (pre string, landed []string, err error) {
	pre, err = gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		return "", nil, err
	}
	onHead := true
	for _, sha := range commits {
		if !SafeGitRev(sha) {
			return pre, nil, fmt.Errorf("commit %q is not a revision", sha)
		}
		if _, verErr := gitOut(root, "rev-parse", "--verify", "--quiet", sha+"^{commit}"); verErr != nil {
			return pre, nil, fmt.Errorf("commit %s no longer exists", sha)
		}
		if _, ancErr := gitOut(root, "merge-base", "--is-ancestor", sha, "HEAD"); ancErr != nil {
			onHead = false
		}
	}
	if onHead {
		return pre, commits, nil
	}
	if blocked := landBlockingPaths(root, commits); len(blocked) > 0 {
		return pre, nil, &LandBlockedError{Paths: blocked}
	}
	picked, err := cherryPick(root, commits)
	if err != nil {
		_, _ = gitOut(root, "reset", "-q", "--keep", pre)
		return pre, nil, err
	}
	return pre, picked, nil
}

// LandUndo takes a landing's pick back to pre, keeping uncommitted changes.
func LandUndo(root, pre string) error {
	if !SafeGitRev(pre) {
		return fmt.Errorf("%q is not a revision", pre)
	}
	_, err := gitOut(root, "reset", "-q", "--keep", pre)
	return err
}

// landBlockingPaths lists the uncommitted paths at root that commits change.
func landBlockingPaths(root string, commits []string) []string {
	// Not gitOut: its trim would eat the status column of the first line.
	cmd := exec.Command("git", "status", "--porcelain", "-z", "--untracked-files=all")
	cmd.Dir = root
	status, err := cmd.Output()
	if err != nil || len(status) == 0 {
		return nil
	}
	dirty := map[string]bool{}
	entries := strings.Split(string(status), "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		dirty[entry[3:]] = true
		// -z writes a rename's source as the next entry, with no status.
		if entry[0] == 'R' || entry[0] == 'C' {
			i++
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, sha := range commits {
		files, diffErr := gitOut(root, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
		if diffErr != nil {
			continue
		}
		for f := range strings.SplitSeq(files, "\n") {
			if f != "" && dirty[f] && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// RemoveLandRequest deletes the request for id in dir; an absent one is not
// an error.
func RemoveLandRequest(dir, id string) error {
	if err := os.Remove(filepath.Join(dir, id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
