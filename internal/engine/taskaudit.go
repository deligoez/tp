package engine

import (
	"fmt"
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
	Seq             int               `json:"seq"`
	Members         []TaskAuditMember `json:"members,omitempty"`
	Files           []string          `json:"files"`
	Roles           []string          `json:"roles"`
	Head            string            `json:"head"`
	Regrade         bool              `json:"regrade,omitempty"`
	EmittedAt       string            `json:"emitted_at"`
	AfterAuditRound int               `json:"after_audit_round,omitempty"`
	RecordedAt      string            `json:"recorded_at,omitempty"`
	File            string            `json:"file,omitempty"`
	Findings        int               `json:"findings,omitempty"`
	Clean           bool              `json:"clean,omitempty"`
}

// TaskAuditFileName is task audit seq's round file in the spec's state
// directory.
func TaskAuditFileName(seq int) string {
	return "task-audit-" + strconv.Itoa(seq) + ".ndjson"
}

// taskAuditMemberKey names one member's close and slice.
type taskAuditMemberKey struct {
	id, closedAt string
	piece        int
}

// owedTaskAuditPieces is every piece of a candidate that owes a role and that
// no recorded task audit holds for the same close.
func owedTaskAuditPieces(cands []TaskAuditCandidate, recorded []TaskAuditRecord) []TaskAuditMember {
	done := make(map[taskAuditMemberKey]bool)
	for i := range recorded {
		for _, m := range recorded[i].Members {
			done[taskAuditMemberKey{m.ID, m.ClosedAt, m.Piece}] = true
		}
	}
	owing := make([]TaskAuditCandidate, 0, len(cands))
	for _, c := range cands {
		if len(c.Roles) > 0 {
			owing = append(owing, c)
		}
	}
	var out []TaskAuditMember
	for _, m := range taskAuditPieces(owing) {
		if !done[taskAuditMemberKey{m.ID, m.ClosedAt, m.Piece}] {
			out = append(out, m)
		}
	}
	return out
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

// TaskAuditClosedAt is the close a task owes its task audit for, in the form
// a batch member records it.
func TaskAuditClosedAt(t *model.Task) string {
	if t.ClosedAt == nil {
		return ""
	}
	return t.ClosedAt.UTC().Format(time.RFC3339Nano)
}

// TaskAuditOwedTasks is the presence rule (spec/backlog/per-task-code-
// audit.md §2.7): the ids of the spec's done tasks whose stamp, met with the
// roles task_audit_roles lists now, is not empty, and for whose close no
// recorded task audit holds every slice. A task closed before the field was
// set carries no stamp and owes nothing.
func TaskAuditOwedTasks(specPath string, st *ReviewState) []string {
	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	tf, err := model.ReadTaskFile(taskPath)
	if err != nil {
		return nil
	}
	roles := EffectiveWorkflowForTaskFile(taskPath).TaskAuditRoles
	recorded := make(map[taskAuditMemberKey]int)
	if st != nil {
		for i := range st.TaskAudits {
			for _, m := range st.TaskAudits[i].Members {
				recorded[taskAuditMemberKey{m.ID, m.ClosedAt, m.Piece}] = m.Pieces
			}
		}
	}
	var owed []string
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusDone || t.ClosedAt == nil || !slices.ContainsFunc(t.TaskAuditOwed, func(r string) bool { return slices.Contains(roles, r) }) {
			continue
		}
		if !allSlicesRecorded(recorded, t.ID, TaskAuditClosedAt(t)) {
			owed = append(owed, t.ID)
		}
	}
	return owed
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
func allSlicesRecorded(recorded map[taskAuditMemberKey]int, id, closedAt string) bool {
	pieces, ok := recorded[taskAuditMemberKey{id, closedAt, 0}]
	if !ok {
		return false
	}
	for p := 1; p < pieces; p++ {
		if _, ok := recorded[taskAuditMemberKey{id, closedAt, p}]; !ok {
			return false
		}
	}
	return true
}

// TaskAuditsOwed reports whether any candidate still owes a piece: the
// presence rule's condition (spec/backlog/per-task-code-audit.md §2.7).
func TaskAuditsOwed(cands []TaskAuditCandidate, recorded []TaskAuditRecord) bool {
	return len(owedTaskAuditPieces(cands, recorded)) > 0
}
