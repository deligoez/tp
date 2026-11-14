package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// Notice kinds an audit emission carries in its payload
// (spec/backlog/checklist-covers-what-changed.md, decisions 3 and 5).
const (
	noticeAutoDetectCap        = "auto-detect-cap"
	noticeCodeFilesCut         = "code-files-cut"
	noticeSpecCoverageFilesCut = "spec-coverage-files-cut"
	noticeNamedListOnePrompt   = "named-list-one-prompt"
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
	// Untasked and Outside size the two sides of a tasks-differ notice (Outside
	// only when a committed range exists), and MergeOnly a merge-only-files
	// notice; their files list is capped at universeNoticeFiles per side.
	Untasked  *int `json:"untasked,omitempty"`
	Outside   *int `json:"outside,omitempty"`
	MergeOnly int  `json:"merge_only,omitempty"`
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
func (n *auditNotice) emit() auditNotice {
	output.Notice(n.Message)
	return *n
}

// cutNotices names every cut of an emitted prompt's file list below the
// auto-detect cap: the code roles' shared list once, and spec-coverage's own.
// A prompt's file_pool above its file_count is the cut; a named list has the
// two equal, so it is never cut, and is named instead when it is longer than
// the code roles' cap (namedListNotice).
func cutNotices(prompts []auditPrompt) []auditNotice {
	var notices []auditNotice
	codeSeen := false
	for i := range prompts {
		p := &prompts[i]
		if p.Role == roleSpecCoverage {
			if p.FilePool > p.FileCount {
				notices = append(notices, (&auditNotice{
					Kind:    noticeSpecCoverageFilesCut,
					Message: fmt.Sprintf("spec-coverage lists %d of %d files", p.FileCount, p.FilePool),
				}).emit())
			}
			continue
		}
		if codeSeen {
			continue
		}
		switch {
		case p.FilePool > p.FileCount:
			codeSeen = true
			notices = append(notices, (&auditNotice{
				Kind: noticeCodeFilesCut,
				Message: fmt.Sprintf("code roles audit %d of %d files, the most-changed first; name the rest with --affected-files, which no cap cuts but one prompt reads shallowly when long",
					p.FileCount, p.FilePool),
			}).emit())
		case p.FileCount > engine.CodeFileCap:
			codeSeen = true
			notices = append(notices, namedListNotice(p.FileCount).emit())
		}
	}
	return notices
}

// namedListNotice names a named list longer than the code roles' cap, which
// goes whole into each code role's one prompt. In a measured trial a role given
// 121 files in one prompt graded all of them by pattern-scanning, and found a
// fraction of what ten-file prompts found on the same files, while its `graded
// N of M` read complete (spec/1.21.1-measurements.md). Where between ten and
// 121 the depth falls is not measured, so the notice states the fact rather
// than a threshold.
func namedListNotice(files int) *auditNotice {
	return &auditNotice{
		Kind: noticeNamedListOnePrompt,
		Message: fmt.Sprintf("code roles grade the named list's %d files in one prompt each; above %d files a role was measured skimming rather than reading (spec/1.21.1-measurements.md)",
			files, engine.CodeFileCap),
	}
}
