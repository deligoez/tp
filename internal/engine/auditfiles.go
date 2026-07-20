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
	// MaintainabilityFileCap further bounds the maintainability-conventions list.
	MaintainabilityFileCap = 10
	// securityHeadLineCap bounds the content heuristic read at HEAD.
	securityHeadLineCap = 200
)

// securitySubstrings mark security-relevant paths and file heads.
var securitySubstrings = []string{"lock", "validate", "auth", "secret", "perm"}

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
	SpecCoverage    []AuditFileEntry
	Security        []AuditFileEntry
	Maintainability []AuditFileEntry
}

// AuditFileInputs carries the pre-collected facts the selection operates on,
// so the selection itself stays deterministic and git-free.
type AuditFileInputs struct {
	Universe   []string            // git diff base..HEAD, or the --affected-files list
	DiffStats  map[string][2]int   // path -> {added, deleted}; absent entries render +0/-0
	Deleted    map[string]bool     // files deleted in the diff
	TaskFiles  map[string][]string // path -> task ids whose commit changed it
	HeadReader func(path string) ([]byte, bool)
}

// SelectAuditFiles applies the drop rules to the universe FIRST, then every
// per-role selection rule, ranking, cap, and fallback — so caps backfill with
// the next eligible files.
func SelectAuditFiles(in *AuditFileInputs) AuditFileSelection {
	universe := filterAuditUniverse(in)

	return AuditFileSelection{
		SpecCoverage:    selectSpecCoverage(in, universe),
		Security:        selectSecurity(in, universe),
		Maintainability: selectMaintainability(in, universe),
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
	return "+0/-0"
}

// selectSpecCoverage returns the union of task-mapped files ranked by task
// count descending (tie-break alphabetical), capped at 20. When no task has a
// usable commit_sha, it falls back to the first 20 universe files with empty
// task lists.
func selectSpecCoverage(in *AuditFileInputs, universe []string) []AuditFileEntry {
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
		return entries
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
	return entries
}

// selectSecurity keeps universe files whose path contains a security
// substring, or whose first 200 lines at HEAD do; files absent at HEAD are
// judged by the path heuristic alone. Alphabetical, capped at 20.
func selectSecurity(in *AuditFileInputs, universe []string) []AuditFileEntry {
	entries := make([]AuditFileEntry, 0, AuditFileCap)
	for _, p := range universe {
		if len(entries) >= AuditFileCap {
			break
		}
		match := containsSecuritySubstring(strings.ToLower(p))
		if !match && in.HeadReader != nil {
			if content, ok := in.HeadReader(p); ok {
				match = containsSecuritySubstring(strings.ToLower(headLines(content, securityHeadLineCap)))
			}
		}
		if match {
			entries = append(entries, AuditFileEntry{Path: p, Tasks: []string{}, DiffSummary: in.diffSummaryOf(p)})
		}
	}
	return entries
}

// selectMaintainability returns the first 10 universe files, regardless of content.
func selectMaintainability(in *AuditFileInputs, universe []string) []AuditFileEntry {
	entries := make([]AuditFileEntry, 0, MaintainabilityFileCap)
	for _, p := range universe {
		if len(entries) >= MaintainabilityFileCap {
			break
		}
		entries = append(entries, AuditFileEntry{Path: p, Tasks: []string{}, DiffSummary: in.diffSummaryOf(p)})
	}
	return entries
}

func containsSecuritySubstring(s string) bool {
	for _, sub := range securitySubstrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// headLines returns the first n lines of content.
func headLines(content []byte, n int) string {
	lines := strings.SplitN(string(content), "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
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

// GitHeadReader reads a file's content at the HEAD revision; ok is false for
// files absent at HEAD (new or untracked).
func GitHeadReader() func(path string) ([]byte, bool) {
	return func(path string) ([]byte, bool) {
		out, err := exec.Command("git", "show", "HEAD:"+filepath.ToSlash(path)).Output()
		if err != nil {
			return nil, false
		}
		return out, true
	}
}
