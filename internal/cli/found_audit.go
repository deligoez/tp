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
	finder, err := engine.FoundAuditFinder(foundSpecPath(taskFilePath), f.auditRow)
	if err != nil {
		_ = foundRefusal(err.Error(), foundAuditRowHint)
	}
	return finder
}

// foundAuditUnitFinder is the finder of a request an audit-role unit files
// without --audit-row, a finding that is not a row (spec/1.31.0.md §7): the
// audit round in flight, so tp found apply holds the request until that round
// is recorded. No round in flight leaves the finder empty.
func foundAuditUnitFinder(taskFilePath string) engine.FoundFinder {
	round, err := engine.InFlightAuditRound(foundSpecPath(taskFilePath))
	if err != nil {
		_ = foundRefusal(err.Error(), foundAuditRowHint)
	}
	return engine.FoundFinder{AuditRound: round}
}

// foundSpecPath is the spec of the task file a request is filed against. A
// spec that does not resolve is refused: its path would name no audit round,
// so the request would carry none for tp found apply to hold it on.
func foundSpecPath(taskFilePath string) string {
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		_ = foundRefusal(err.Error(), foundAuditRowHint)
	}
	specPath, found := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	if !found {
		_ = foundRefusal(fmt.Sprintf("the spec %s of task file %s does not resolve", tf.Spec, taskFilePath), "fix the task file's spec field or restore the spec")
	}
	return specPath
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
// A store that cannot be read or parsed is the error: the rows filed from it
// would otherwise be recorded open with nothing saying why.
func fileFoundAuditRows(taskFilePath string, round int, data []byte, rows []map[string]any) ([]byte, error) {
	if taskFilePath == "" {
		return data, nil
	}
	reqs, err := engine.ReadFoundRequests(engine.CanonicalProjectRoot(filepath.Dir(taskFilePath)), taskFilePath)
	if err != nil {
		return nil, err
	}
	if len(reqs) == 0 {
		return data, nil
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
		return data, nil
	}
	return []byte(strings.Join(lines, "\n")), nil
}
