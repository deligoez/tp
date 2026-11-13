package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/output"
)

// Notice kinds an audit emission carries in its payload
// (spec/backlog/checklist-covers-what-changed.md, decisions 3 and 5).
const (
	noticeAutoDetectCap        = "auto-detect-cap"
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
	// TotalChanged is the auto-detect-cap notice's pre-cap count, the same
	// number file_summary.total_changed carries where --compact keeps none.
	TotalChanged int `json:"total_changed,omitempty"`
}

// autoDetectCapMessage is the auto-detect cut's one sentence, on stderr and
// in the notice alike.
func autoDetectCapMessage(totalChanged int) string {
	return fmt.Sprintf("%d files changed, auditing first %d — name the rest with --affected-files", totalChanged, maxAutoDetectFiles)
}

// autoDetectCapNotice puts the auto-detect cut in the payload. detectChangedFiles
// already printed it, so it is not printed again. It is the one cut a prompt's
// file_pool does not show: the pool is what the role's list was cut from,
// after this cap.
func autoDetectCapNotice(totalChanged, audited int) []auditNotice {
	if totalChanged <= audited {
		return nil
	}
	return []auditNotice{{Kind: noticeAutoDetectCap, Message: autoDetectCapMessage(totalChanged), TotalChanged: totalChanged}}
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
