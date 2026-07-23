package engine

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// Bookkeeping kinds (§5.2): closure is the task file, round is a .tp-review/
// round or snapshot file, config is any other dirty .tp/ state file.
const (
	BookkeepingClosure = "closure"
	BookkeepingRound   = "round"
	BookkeepingConfig  = "config"
)

// roundNumberRe captures the trailing integer in a .tp-review/ round or snapshot
// filename (review-round-N.ndjson, audit-round-N.ndjson, snapshot-round-N.md).
var roundNumberRe = regexp.MustCompile(`(?:review|audit|snapshot)-round-(\d+)`)

// DeriveBookkeeping splits the dirty working-tree paths into tp-owned
// bookkeeping entries (§5.2) and the remaining unexplained-change candidates.
// tp-owned paths are: the task file itself (closure), anything under the spec's
// .tp-review/ directory (round), and anything else under the project .tp/
// directory (config). The closure kind is derived by diffing the task file
// against HEAD, emitting one entry per task whose managed fields changed.
//
// Classification runs on paths already cleared by the keep-list, so a tp-owned
// file the agent explicitly keep-listed stays in kept and is not double-
// reported. remaining preserves input order; the caller sorts. bookkeeping is
// returned sorted by (path, ref) and is never nil.
func DeriveBookkeeping(start, taskFilePath, specPath string, dirty []string, tf *model.TaskFile) (bookkeeping []BookkeepingEntry, remaining []string) {
	taskRel, err := NormalizeKeepPath(start, taskFilePath)
	if err != nil {
		taskRel = taskFilePath
	}
	tpReviewPrefix := tpOwnedPrefix(start, filepath.Dir(ReviewStateDir(specPath)), ".tp-review")
	tpConfigPrefix := tpOwnedPrefix(start, ProjectConfigDir(start), ".tp")

	bookkeeping = make([]BookkeepingEntry, 0)
	remaining = make([]string, 0, len(dirty))
	taskFileDirty := false

	for _, p := range dirty {
		switch {
		case p == taskRel:
			taskFileDirty = true
		case strings.HasPrefix(p, tpReviewPrefix):
			bookkeeping = append(bookkeeping, BookkeepingEntry{
				Path: p,
				Kind: BookkeepingRound,
				Ref:  roundRefFromName(p),
			})
		case strings.HasPrefix(p, tpConfigPrefix):
			bookkeeping = append(bookkeeping, BookkeepingEntry{
				Path: p,
				Kind: BookkeepingConfig,
				Ref:  filepath.Base(p),
			})
		default:
			remaining = append(remaining, p)
		}
	}

	if taskFileDirty {
		bookkeeping = append(bookkeeping, closureEntries(start, taskRel, tf)...)
	}

	sort.Slice(bookkeeping, func(i, j int) bool {
		if bookkeeping[i].Path != bookkeeping[j].Path {
			return bookkeeping[i].Path < bookkeeping[j].Path
		}
		return bookkeeping[i].Ref < bookkeeping[j].Ref
	})
	return bookkeeping, remaining
}

// tpOwnedPrefix returns the repo-root-relative form of path (an absolute or
// cwd-relative directory) with a trailing slash, for HasPrefix classification.
// fallback is used when the path cannot be normalized.
func tpOwnedPrefix(start, path, fallback string) string {
	rel, err := NormalizeKeepPath(start, path)
	if err != nil || rel == "" || rel == "." {
		rel = fallback
	}
	if !strings.HasSuffix(rel, "/") {
		rel += "/"
	}
	return rel
}

// roundRefFromName returns the round number parsed from a .tp-review/ filename
// (e.g. "review-round-1.ndjson" -> "1"), or the file's base name when no round
// number is present (e.g. state.json).
func roundRefFromName(path string) string {
	base := filepath.Base(path)
	if m := roundNumberRe.FindStringSubmatch(base); len(m) == 2 {
		return m[1]
	}
	return base
}

// closureEntries derives one closure bookkeeping entry per task whose managed
// fields (status, closed_at, gate_passed_at, commit_sha) changed between the
// HEAD version of the task file and the current tf. When the HEAD baseline is
// unavailable (untracked file, no commits), unparseable, or no managed field
// differs while the file is still dirty, it falls back to a single entry whose
// ref is the task file's base name — so a dirty task file is always captured as
// closure bookkeeping and never leaks into changes.
func closureEntries(start, taskRel string, tf *model.TaskFile) []BookkeepingEntry {
	fallback := BookkeepingEntry{Path: taskRel, Kind: BookkeepingClosure, Ref: filepath.Base(taskRel)}
	headBytes, ok := FileAtHead(start, taskRel)
	if !ok {
		return []BookkeepingEntry{fallback}
	}
	var headTF model.TaskFile
	if err := json.Unmarshal(headBytes, &headTF); err != nil {
		return []BookkeepingEntry{fallback}
	}
	headByID := make(map[string]model.Task, len(headTF.Tasks))
	for i := range headTF.Tasks {
		headByID[headTF.Tasks[i].ID] = headTF.Tasks[i]
	}
	entries := make([]BookkeepingEntry, 0)
	for i := range tf.Tasks {
		cur := tf.Tasks[i]
		prev, hadPrev := headByID[cur.ID]
		if hadPrev && managedFieldsChanged(&prev, &cur) {
			entries = append(entries, BookkeepingEntry{Path: taskRel, Kind: BookkeepingClosure, Ref: cur.ID})
		}
	}
	if len(entries) == 0 {
		return []BookkeepingEntry{fallback}
	}
	return entries
}

// managedFieldsChanged reports whether any of the close-managed fields differ
// between two snapshots of the same task (§5.2).
func managedFieldsChanged(a, b *model.Task) bool {
	if a.Status != b.Status {
		return true
	}
	if !ptrTimeEqual(a.ClosedAt, b.ClosedAt) {
		return true
	}
	if !ptrTimeEqual(a.GatePassedAt, b.GatePassedAt) {
		return true
	}
	if !ptrStrEqual(a.CommitSHA, b.CommitSHA) {
		return true
	}
	return false
}

func ptrTimeEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func ptrStrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
