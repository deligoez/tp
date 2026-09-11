package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// acceptanceStatus reports whether a disposition accepts a finding as it
// stands — wontfix or duplicate — rather than saying the text or code changed.
func acceptanceStatus(status string) bool {
	return status == "wontfix" || status == "duplicate"
}

// requireAcceptanceEvidence refuses a wontfix or duplicate whose evidence is
// blank, before anything is written. Such a disposition reads as resolved and
// clears nothing — convergence reads acceptance only with evidence — so writing
// it would report success for an acceptance that never took effect.
func requireAcceptanceEvidence(status, evidence string) {
	if !acceptanceStatus(status) || strings.TrimSpace(evidence) != "" {
		return
	}
	output.Error(ExitUsage,
		fmt.Sprintf("a %s disposition needs evidence: without it the finding stays open", status),
		fmt.Sprintf("say why the finding stays as written, e.g. --resolve <selector> %s \"<what you ran or read>\"", status))
	os.Exit(ExitUsage)
}

// fenceReviewAcceptance refuses, under TP_UNATTENDED, accepting a critical or
// high review finding without a spec change: that is the operator's decision,
// so a unit escalates instead. rows are the rows the command would write; a
// medium or low finding, and any fixed disposition, stay a unit's to write.
func fenceReviewAcceptance(status string, rows []map[string]any) {
	if !engine.Unattended() || !acceptanceStatus(status) {
		return
	}
	for _, row := range rows {
		if engine.ReviewRowBlocking(row) {
			refuseUnattended("accepting a critical or high review finding without a spec change", engine.EscalateAcceptFinding)
			return
		}
	}
}

// fenceAuditAcceptance refuses, under TP_UNATTENDED, accepting an audit
// finding without a code change. fixed stays open: it is the audit-fix unit's
// own write.
func fenceAuditAcceptance(status string) {
	if engine.Unattended() && acceptanceStatus(status) {
		refuseUnattended("accepting an audit finding without a code change", engine.EscalateAcceptFinding)
	}
}
