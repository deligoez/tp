package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/output"
)

// Notice kinds an audit emission carries in its payload
// (spec/backlog/checklist-covers-what-changed.md, decisions 3 and 5).
const (
	noticeCodeFilesCut         = "code-files-cut"
	noticeSpecCoverageFilesCut = "spec-coverage-files-cut"
)

// auditNotice is one notice an emission printed on stderr, kept in the payload
// as well: --quiet erases stderr and --compact drops file_summary, and an agent
// reading stdout must still learn that its list was cut or its universe looks
// wrong. Files names the paths the notice is about, when it is about some.
type auditNotice struct {
	Kind    string   `json:"kind"`
	Message string   `json:"message"`
	Files   []string `json:"files,omitempty"`
}

// emit prints the notice on stderr and returns it for the payload.
func (n auditNotice) emit() auditNotice {
	output.Notice(n.Message)
	return n
}

// cutNotices names every cut of an emitted prompt's file list below the
// auto-detect cap: the code roles' shared list once, and spec-coverage's own.
// A prompt's file_pool above its file_count is the cut; a named list has the
// two equal, so it is never reported.
func cutNotices(prompts []auditPrompt) []auditNotice {
	var notices []auditNotice
	codeSeen := false
	for i := range prompts {
		p := &prompts[i]
		if p.FilePool <= p.FileCount {
			continue
		}
		if p.Role == roleSpecCoverage {
			notices = append(notices, auditNotice{
				Kind:    noticeSpecCoverageFilesCut,
				Message: fmt.Sprintf("spec-coverage lists %d of %d files", p.FileCount, p.FilePool),
			}.emit())
			continue
		}
		if codeSeen {
			continue
		}
		codeSeen = true
		notices = append(notices, auditNotice{
			Kind: noticeCodeFilesCut,
			Message: fmt.Sprintf("code roles audit %d of %d files, the most-changed first; name the rest with --affected-files, which no cap cuts",
				p.FileCount, p.FilePool),
		}.emit())
	}
	return notices
}
