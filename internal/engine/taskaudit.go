package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// TaskAuditBatchFiles is the most files one task-audit prompt reads: the only
// prompt size whose depth was measured (spec/backlog/per-task-code-audit.md §1).
const TaskAuditBatchFiles = 10

// TaskAuditCandidate is a done task that owes a task audit: its id, the close
// it owes it for, the roles it owes, and its auditable files.
type TaskAuditCandidate struct {
	ID       string
	ClosedAt string
	Roles    []string
	Files    []string
}

// TaskAuditMember is one task, or one slice of a task over ten files, in a
// task-audit batch. Piece counts from 0 of Pieces; a task of ten files or
// fewer is piece 0 of 1. ClosedAt names the close the member was batched
// for, so a reopened and re-closed task owes again.
type TaskAuditMember struct {
	ID       string   `json:"id"`
	ClosedAt string   `json:"closed_at"`
	Piece    int      `json:"piece"`
	Pieces   int      `json:"pieces"`
	Roles    []string `json:"roles"`
	Files    []string `json:"files"`
}

// TaskAuditBatch is the members one task-audit emission reads together. Full
// is set when the batch holds ten files or the next member would overflow it;
// only the last batch of a packing can be partial.
type TaskAuditBatch struct {
	Members []TaskAuditMember
	Full    bool
}

// Files is the batch's files, member by member.
func (b TaskAuditBatch) Files() []string {
	var out []string
	for _, m := range b.Members {
		out = append(out, m.Files...)
	}
	return out
}

// Roles is the union of the roles the batch's members owe, sorted.
func (b TaskAuditBatch) Roles() []string {
	var out []string
	for _, m := range b.Members {
		for _, r := range m.Roles {
			if !slices.Contains(out, r) {
				out = append(out, r)
			}
		}
	}
	sort.Strings(out)
	return out
}

// taskAuditPieces cuts each candidate, in close order with the task id
// breaking a tie, into members of at most ten sorted files.
func taskAuditPieces(cands []TaskAuditCandidate) []TaskAuditMember {
	ordered := slices.Clone(cands)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].ClosedAt != ordered[j].ClosedAt {
			return ordered[i].ClosedAt < ordered[j].ClosedAt
		}
		return ordered[i].ID < ordered[j].ID
	})
	var out []TaskAuditMember
	for _, c := range ordered {
		fs := slices.Clone(c.Files)
		sort.Strings(fs)
		pieces := max(1, (len(fs)+TaskAuditBatchFiles-1)/TaskAuditBatchFiles)
		for p := range pieces {
			lo, hi := p*TaskAuditBatchFiles, min(len(fs), (p+1)*TaskAuditBatchFiles)
			out = append(out, TaskAuditMember{
				ID: c.ID, ClosedAt: c.ClosedAt, Piece: p, Pieces: pieces,
				Roles: slices.Clone(c.Roles), Files: fs[lo:hi:hi],
			})
		}
	}
	return out
}

// PackTaskAudits packs the candidates greedily, in close order, into batches
// of at most ten files. The packing is prefix-stable: a candidate closed later
// never changes a batch that was full before it closed.
func PackTaskAudits(cands []TaskAuditCandidate) []TaskAuditBatch {
	return packTaskAuditPieces(taskAuditPieces(cands))
}

// packTaskAuditPieces packs members greedily, in order, into batches of at
// most ten files.
func packTaskAuditPieces(pieces []TaskAuditMember) []TaskAuditBatch {
	var out []TaskAuditBatch
	var cur TaskAuditBatch
	n := 0
	for _, m := range pieces {
		if len(cur.Members) > 0 && n+len(m.Files) > TaskAuditBatchFiles {
			cur.Full = true
			out = append(out, cur)
			cur, n = TaskAuditBatch{}, 0
		}
		cur.Members = append(cur.Members, m)
		n += len(m.Files)
		if n == TaskAuditBatchFiles {
			cur.Full = true
			out = append(out, cur)
			cur, n = TaskAuditBatch{}, 0
		}
	}
	if len(cur.Members) > 0 {
		out = append(out, cur)
	}
	return out
}

// TaskAuditRecord is one recorded task audit in state.json's task_audits: a
// batch's members (none on a re-grade), the files and roles it read, the HEAD
// it read them at, where it sits among the audit rounds, and its round file.
// The same shape, without the recorded fields, is the frozen emission.
type TaskAuditRecord struct {
	Seq     int               `json:"seq"`
	Members []TaskAuditMember `json:"members,omitempty"`
	Files   []string          `json:"files"`
	Roles   []string          `json:"roles"`
	Head    string            `json:"head"`
	Regrade bool              `json:"regrade,omitempty"`
	// FixPass marks a re-grade a fix claim made due: at most one such pass
	// runs between two audit rounds, so fix and re-grade cannot cycle past
	// the round cap.
	FixPass         bool   `json:"fix_pass,omitempty"`
	EmittedAt       string `json:"emitted_at"`
	AfterAuditRound int    `json:"after_audit_round,omitempty"`
	RecordedAt      string `json:"recorded_at,omitempty"`
	File            string `json:"file,omitempty"`
	Findings        int    `json:"findings,omitempty"`
	Clean           bool   `json:"clean,omitempty"`
}

// TaskAuditFileName is task audit seq's round file in the spec's state
// directory.
func TaskAuditFileName(seq int) string {
	return "task-audit-" + strconv.Itoa(seq) + ".ndjson"
}

// owedTaskAuditPieces is the slices of what each candidate still owes: its
// files no recorded member of the same close read, cut into slices of ten
// that number after the close's recorded ones. A candidate with no file left,
// or owing no role, owes nothing.
func owedTaskAuditPieces(cands []TaskAuditCandidate, recorded []TaskAuditRecord) []TaskAuditMember {
	cands = subtractReadFiles(cands, recorded)
	doneSlices := recordedSlices(recorded)
	owing := make([]TaskAuditCandidate, 0, len(cands))
	for _, c := range cands {
		if len(c.Roles) > 0 && len(c.Files) > 0 {
			owing = append(owing, c)
		}
	}
	out := taskAuditPieces(owing)
	for i := range out {
		done := doneSlices[[2]string{out[i].ID, out[i].ClosedAt}]
		out[i].Piece += done
		out[i].Pieces += done
	}
	return out
}

// subtractReadFiles takes from each candidate the files a recorded member of
// the same close already read. Membership is by file, not by slice number,
// so a file set that moves after a slice is recorded (a file deleted, one a
// grading already covers) never leaves a slice owed that no batch can fill.
func subtractReadFiles(cands []TaskAuditCandidate, recorded []TaskAuditRecord) []TaskAuditCandidate {
	read := make(map[[2]string]map[string]bool)
	for i := range recorded {
		for _, m := range recorded[i].Members {
			key := [2]string{m.ID, m.ClosedAt}
			if read[key] == nil {
				read[key] = make(map[string]bool)
			}
			for _, f := range m.Files {
				read[key][f] = true
			}
		}
	}
	out := make([]TaskAuditCandidate, 0, len(cands))
	for _, c := range cands {
		if seen := read[[2]string{c.ID, c.ClosedAt}]; len(seen) > 0 {
			left := make([]string, 0, len(c.Files))
			for _, f := range c.Files {
				if !seen[f] {
					left = append(left, f)
				}
			}
			c.Files = left
		}
		out = append(out, c)
	}
	return out
}

// recordedSlices counts the recorded members of each close, so a later slice
// of it numbers after them.
func recordedSlices(recorded []TaskAuditRecord) map[[2]string]int {
	n := make(map[[2]string]int)
	for i := range recorded {
		for _, m := range recorded[i].Members {
			n[[2]string{m.ID, m.ClosedAt}]++
		}
	}
	return n
}

// DueTaskAudits is the batches the owed pieces pack into that are due now:
// the full ones, and at the end of the implement phase the partial last one.
func DueTaskAudits(cands []TaskAuditCandidate, recorded []TaskAuditRecord, endOfPhase bool) []TaskAuditBatch {
	var out []TaskAuditBatch
	for _, b := range packTaskAuditPieces(owedTaskAuditPieces(cands, recorded)) {
		if b.Full || endOfPhase {
			out = append(out, b)
		}
	}
	return out
}

// unreadableTaskFile prefixes the one entry TaskAuditOwedTasks returns for a
// task file it cannot read: presence fails closed, and the clause names the
// file to repair instead of a task audit no emission could run.
const unreadableTaskFile = "unreadable "

// TaskAuditClosedAt is the close a task owes its task audit for, in the form
// a batch member records it.
func TaskAuditClosedAt(t *model.Task) string {
	if t.ClosedAt == nil {
		return ""
	}
	return t.ClosedAt.UTC().Format(time.RFC3339Nano)
}

// TaskAuditOwedTasks is the presence rule (spec/backlog/per-task-code-
// audit.md §2.7): the ids of the spec's done tasks that still owe a file to a
// role their stamp, task_audit_roles and the audit panel all name, by the
// same computation the emission and the oracle use. A task closed before the
// field was set carries no stamp and owes nothing.
func TaskAuditOwedTasks(specPath string, st *ReviewState) []string {
	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	tf, err := model.ReadTaskFile(taskPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		// A task file nobody can read is no proof that nothing is owed.
		return []string{unreadableTaskFile + taskPath}
	}
	var recorded []TaskAuditRecord
	if st != nil {
		recorded = st.TaskAudits
	}
	cands := subtractReadFiles(TaskAuditCandidates(specPath, tf.Tasks, TaskAuditRolesFor(specPath)), recorded)
	var owed []string
	for _, m := range owedTaskAuditPieces(dropGradedFiles(specPath, st, cands), recorded) {
		if !slices.Contains(owed, m.ID) {
			owed = append(owed, m.ID)
		}
	}
	return owed
}

// owedTaskAuditsNow is TaskAuditOwedTasks over the spec's state as it is on
// disk, for a reader handed only the loop's verdict.
func owedTaskAuditsNow(specPath string) []string {
	st, err := LoadReviewState(specPath)
	if err != nil {
		st = nil // no recorded task audit can be read: everything stamped is owed
	}
	return TaskAuditOwedTasks(specPath, st)
}

// taskAuditOwedClauseNow is the clause the presence rule holds the audit open
// with, read from disk, or "" when nothing is owed.
func taskAuditOwedClauseNow(specPath string) string {
	owed := owedTaskAuditsNow(specPath)
	if len(owed) == 0 {
		return ""
	}
	if path, ok := strings.CutPrefix(owed[0], unreadableTaskFile); ok {
		return "the task file " + path + " cannot be read, so whether a closed task owes a task audit is unknown: repair it, then tp audit " + specPath + " --status"
	}
	return TaskAuditOwedClause(owed, specPath)
}

// TaskAuditOwedClause words what the presence rule holds open and the step
// that clears it, so a loop held by it names something to do.
func TaskAuditOwedClause(owed []string, specPath string) string {
	shown := owed
	if len(shown) > 5 {
		shown = shown[:5]
	}
	return fmt.Sprintf("%d closed task(s) owe a task audit (%s): emit it — tp audit %s --task-audit — run its roles, then tp audit %s --task-audit --record-inbox",
		len(owed), strings.Join(shown, ", "), specPath, specPath)
}

// allSlicesRecorded reports whether every slice of the close is recorded:
// slice 0 names how many there are.

// TaskAuditsOwed reports whether any candidate still owes a piece: the
// presence rule's condition (spec/backlog/per-task-code-audit.md §2.7).
func TaskAuditsOwed(cands []TaskAuditCandidate, recorded []TaskAuditRecord) bool {
	return len(owedTaskAuditPieces(cands, recorded)) > 0
}
