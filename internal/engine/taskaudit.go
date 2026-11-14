package engine

import (
	"slices"
	"sort"
	"strconv"
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

// TaskAuditsOwed reports whether any candidate still owes a piece: the
// presence rule's condition (spec/backlog/per-task-code-audit.md §2.7).
func TaskAuditsOwed(cands []TaskAuditCandidate, recorded []TaskAuditRecord) bool {
	return len(owedTaskAuditPieces(cands, recorded)) > 0
}
