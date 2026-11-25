package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// foundAuditRowHint says what an audit row may file, and what takes the
// other paths.
const foundAuditRowHint = "--audit-row names the row (role:item_id) of a Fix-shaped finding on an item this audit round did not ask you about; a finding on an asked item is recorded in your results file and left open for the audit-fix path"

// foundAuditRowFinder names the finder of a request filed from an audit row
// (spec/1.31.0.md §7), refusing with exit 2 what §7 keeps on today's paths: a
// request that is not Fix-shaped (recorded and shipped as today), a row on an
// asked item (the audit-fix path), and a caller with no audit round in flight.
// An audit row works no task, so --task and --blocks are refused beside it.
func foundAuditRowFinder(taskFilePath string, f *foundAddFlags, req *engine.FoundRequest) engine.FoundFinder {
	if f.task != "" || f.blocks {
		_ = foundRefusal("--audit-row files a request from an audit row, which works no task; drop --task and --blocks", foundAuditRowHint)
	}
	if !req.FixShaped() || strings.TrimSpace(req.Question) != "" {
		_ = foundRefusal("an audit finding outside the spec's surface becomes a request only when it is Fix-shaped: --expected and --observed, no --question", foundAuditRowHint)
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		_ = foundRefusal(err.Error(), foundAuditRowHint)
	}
	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	finder, err := engine.FoundAuditFinder(specPath, f.auditRow)
	if err != nil {
		_ = foundRefusal(err.Error(), foundAuditRowHint)
	}
	return finder
}

// fileFoundAuditRows writes, at --record, the disposition of every row of
// audit round round a request was filed from (spec/1.31.0.md §7): wontfix,
// with evidence and a `request` key naming the request, so the row does not
// dirty the round while its defect is tp found apply's to place once the
// round is recorded. tp writes it from the request the store holds, so it is
// no acceptance a unit made, and the accept-finding fence the payload's own
// dispositions pass through has no place here.
//
// rows are the parsed non-blank lines of data, in order; a filed row is
// updated in place, so the clean verdict computed from rows reads it, and its
// line of data is re-encoded. data comes back unchanged when no row was filed.
func fileFoundAuditRows(taskFilePath string, round int, data []byte, rows []map[string]any) []byte {
	if taskFilePath == "" {
		return data
	}
	reqs, err := engine.ReadFoundRequests(engine.CanonicalProjectRoot(filepath.Dir(taskFilePath)), taskFilePath)
	if err != nil || len(reqs) == 0 {
		return data
	}
	lines := strings.Split(string(data), "\n")
	next, filed := 0, 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || next >= len(rows) {
			continue
		}
		row := rows[next]
		next++
		req, ok := engine.FoundAuditRequest(reqs, round, row)
		if !ok {
			continue
		}
		resolved := disposition("wontfix", fmt.Sprintf("outside the spec's surface: filed as found request %s, applied once this round is recorded", req.ID))
		resolved["request"] = req.ID
		row["resolved"] = resolved
		// A row decoded from JSON always re-encodes.
		encoded, _ := json.Marshal(row)
		lines[i] = string(encoded)
		filed++
	}
	if filed == 0 {
		return data
	}
	return []byte(strings.Join(lines, "\n"))
}
