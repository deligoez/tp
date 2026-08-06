package engine

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

const (
	// AuditFileCap bounds every prompt's affected-files list.
	AuditFileCap = 20
	// CodeFileCap further bounds the shared code-file list.
	CodeFileCap = 10
)

// priorityPathSubstrings mark paths that rank ahead of the rest.
var priorityPathSubstrings = []string{"lock", "validate", "auth", "secret", "perm"}

// auditBinaryExtensions mirrors the binary-file check used at audit time.
var auditBinaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".zip": true, ".tar": true, ".gz": true, ".pdf": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true,
}

// AuditFileEntry is one selected file: repo-relative path, the ids of tasks
// whose recorded commit touched it (empty for the file-checklist roles), and
// the "+N/-M" diff summary.
type AuditFileEntry struct {
	Path        string   `json:"path"`
	Tasks       []string `json:"tasks"`
	DiffSummary string   `json:"diff_summary"`
}

// AuditFileSelection holds the per-role affected-files lists.
type AuditFileSelection struct {
	SpecCoverage []AuditFileEntry
	CodeFiles    []AuditFileEntry
	// SpecCoverageTotal and CodeFilesTotal carry each list's pre-cap size, so a
	// prompt can report how much of the pool the cap left out.
	SpecCoverageTotal int
	CodeFilesTotal    int
}

// AuditFileInputs carries the pre-collected facts the selection operates on,
// so the selection itself stays deterministic and git-free.
type AuditFileInputs struct {
	Universe  []string            // git diff base..HEAD, or the --affected-files list
	DiffStats map[string][2]int   // path -> {added, deleted}; absent entries render +0/-0
	Deleted   map[string]bool     // files deleted in the diff
	TaskFiles map[string][]string // path -> task ids whose commit changed it
	// DiffUnmeasured says no comparison covers Universe, so an absent DiffStats
	// entry means "not measured" rather than "unchanged". Set when the caller
	// replaced the universe (--affected-files, --affected-from-tasks): the diff
	// ranges the audit compares are derived from auto-detection and need not
	// contain a path the caller named, so rendering the fallback +0/-0 would
	// hand every role an unmeasured zero as measured fact.
	DiffUnmeasured bool
}

// SelectAuditFiles applies the drop rules to the universe FIRST, then every
// per-role selection rule, ranking, cap, and fallback — so caps backfill with
// the next eligible files.
func SelectAuditFiles(in *AuditFileInputs) AuditFileSelection {
	universe := filterAuditUniverse(in)
	specCoverage, specCoverageTotal := selectSpecCoverage(in, universe)

	return AuditFileSelection{
		SpecCoverage:      specCoverage,
		CodeFiles:         selectCodeFiles(in, universe),
		SpecCoverageTotal: specCoverageTotal,
		CodeFilesTotal:    len(universe),
	}
}

// filterAuditUniverse drops binaries, test fixtures, and deleted files, then
// sorts alphabetically.
func filterAuditUniverse(in *AuditFileInputs) []string {
	out := make([]string, 0, len(in.Universe))
	for _, p := range in.Universe {
		if in.Deleted[p] || IsBinaryPath(p) || isTestFixture(p) {
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// IsBinaryPath reports whether the path has a binary file extension.
func IsBinaryPath(p string) bool {
	return auditBinaryExtensions[strings.ToLower(filepath.Ext(p))]
}

// isTestFixture matches testdata/** and *.golden paths.
func isTestFixture(p string) bool {
	if strings.HasSuffix(p, ".golden") {
		return true
	}
	clean := filepath.ToSlash(p)
	return strings.HasPrefix(clean, "testdata/") || strings.Contains(clean, "/testdata/")
}

func (in *AuditFileInputs) diffSummaryOf(p string) string {
	if s, ok := in.DiffStats[p]; ok {
		return fmt.Sprintf("+%d/-%d", s[0], s[1])
	}
	if in.DiffUnmeasured {
		// Empty, not "+0/-0": the caller renders no diff annotation at all
		// rather than stating a churn nothing measured.
		return ""
	}
	return "+0/-0"
}

// selectSpecCoverage returns the union of task-mapped files ranked by task
// count descending (tie-break alphabetical), capped at 20. When no task has a
// usable commit_sha, it falls back to the first 20 universe files with empty
// task lists. The second return value is the pre-cap size of whichever pool
// the branch capped.
func selectSpecCoverage(in *AuditFileInputs, universe []string) (selected []AuditFileEntry, total int) {
	type fileCount struct {
		path  string
		count int
	}
	mapped := make([]fileCount, 0)
	for _, p := range universe {
		if ids := in.TaskFiles[p]; len(ids) > 0 {
			mapped = append(mapped, fileCount{path: p, count: len(ids)})
		}
	}

	if len(mapped) == 0 {
		entries := make([]AuditFileEntry, 0, AuditFileCap)
		for _, p := range universe {
			if len(entries) >= AuditFileCap {
				break
			}
			entries = append(entries, AuditFileEntry{Path: p, Tasks: []string{}, DiffSummary: in.diffSummaryOf(p)})
		}
		return entries, len(universe)
	}

	sort.Slice(mapped, func(i, j int) bool {
		if mapped[i].count != mapped[j].count {
			return mapped[i].count > mapped[j].count
		}
		return mapped[i].path < mapped[j].path
	})

	entries := make([]AuditFileEntry, 0, AuditFileCap)
	for _, fc := range mapped {
		if len(entries) >= AuditFileCap {
			break
		}
		ids := append([]string(nil), in.TaskFiles[fc.path]...)
		sort.Strings(ids)
		entries = append(entries, AuditFileEntry{Path: fc.path, Tasks: ids, DiffSummary: in.diffSummaryOf(fc.path)})
	}
	return entries, len(mapped)
}

// selectCodeFiles orders the filtered universe with path-keyword-matching files
// first and every other file after, each group alphabetical, capped at
// CodeFileCap. A non-empty universe therefore never yields an empty list.
func selectCodeFiles(in *AuditFileInputs, universe []string) []AuditFileEntry {
	priority := make([]string, 0, len(universe))
	rest := make([]string, 0, len(universe))
	for _, p := range universe {
		if matchesPriorityPath(strings.ToLower(p)) {
			priority = append(priority, p)
			continue
		}
		rest = append(rest, p)
	}

	entries := make([]AuditFileEntry, 0, CodeFileCap)
	for _, group := range [][]string{priority, rest} {
		for _, p := range group {
			if len(entries) >= CodeFileCap {
				return entries
			}
			entries = append(entries, AuditFileEntry{Path: p, Tasks: []string{}, DiffSummary: in.diffSummaryOf(p)})
		}
	}
	return entries
}

func matchesPriorityPath(s string) bool {
	for _, sub := range priorityPathSubstrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// GitTaskFileMapping maps each universe file to the sorted ids of tasks whose
// recorded commit_sha changed it. Tasks without a commit_sha, or whose sha is
// unknown to git, map to zero files.
func GitTaskFileMapping(tasks []model.Task, universe []string) map[string][]string {
	inUniverse := make(map[string]bool, len(universe))
	for _, p := range universe {
		inUniverse[p] = true
	}

	byFile := make(map[string]map[string]bool)
	for i := range tasks {
		if tasks[i].CommitSHA == nil || *tasks[i].CommitSHA == "" {
			continue
		}
		// The task file is written by import and add as well as by done, so a
		// stored sha is not guaranteed to have passed an entry-point check.
		if !SafeGitRev(*tasks[i].CommitSHA) {
			continue
		}
		out, err := exec.Command("git", "show", "--name-only", "--pretty=format:", *tasks[i].CommitSHA).Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(bytes.TrimSpace(out)), "\n") {
			f := strings.TrimSpace(line)
			if f == "" || !inUniverse[f] {
				continue
			}
			if byFile[f] == nil {
				byFile[f] = make(map[string]bool)
			}
			byFile[f][tasks[i].ID] = true
		}
	}

	result := make(map[string][]string, len(byFile))
	for f, ids := range byFile {
		sorted := make([]string, 0, len(ids))
		for id := range ids {
			sorted = append(sorted, id)
		}
		sort.Strings(sorted)
		result[f] = sorted
	}
	return result
}
