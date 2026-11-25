package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// FoundAuditFinder names the finder of a request an audit-role unit files
// from one of its rows (spec/1.31.0.md §7): the audit round in flight, so
// tp found apply holds the request until that round is recorded, and the
// row's role:item_id key. A row on an item the round asked the role keeps the
// audit-fix path and never becomes a request, so such a key is refused, as are
// a key that is not role:item_id and a spec with no audit round in flight.
func FoundAuditFinder(specPath, rowKey string) (FoundFinder, error) {
	role, item, ok := strings.Cut(rowKey, ":")
	role, item = strings.TrimSpace(role), strings.TrimSpace(item)
	if !ok || role == "" || item == "" {
		return FoundFinder{}, fmt.Errorf("--audit-row %q is not a row key; name the row as role:item_id", rowKey)
	}
	recorded := 0
	st, err := LoadReviewState(specPath)
	switch {
	case err != nil && !IsRebuildableStateIndex(err):
		return FoundFinder{}, err
	case st != nil:
		recorded = len(st.AuditRounds)
	}
	round := InFlightRound(specPath, PhaseAudit, recorded)
	if round == 0 {
		return FoundFinder{}, errors.New("no audit round is in flight, so no audit row can file a request")
	}
	pools, _, err := ReadAuditPools(specPath, round)
	if err != nil {
		return FoundFinder{}, err
	}
	for _, p := range pools {
		if p.Role == role && slices.Contains(p.Items, item) {
			return FoundFinder{}, fmt.Errorf("audit round %d asked %s about %s, so its row takes the audit-fix path and files no request", round, role, item)
		}
	}
	return FoundFinder{AuditRound: round, RowKey: role + ":" + item}, nil
}

// FoundAuditRequest returns the request filed from row in audit round round
// (spec/1.31.0.md §7): a Fix-shaped request whose finder names that round and
// the row's role:item_id key. It is false for a PASS row, a row that already
// carries a disposition, and a row no such request names.
func FoundAuditRequest(reqs []FoundRequest, round int, row map[string]any) (FoundRequest, bool) {
	if AuditRowIsPass(row) {
		return FoundRequest{}, false
	}
	if _, own := row["resolved"]; own {
		return FoundRequest{}, false
	}
	key := auditRowKey(row)
	for i := range reqs {
		r := &reqs[i]
		if r.Finder.AuditRound == round && r.Finder.RowKey == key && r.FixShaped() && strings.TrimSpace(r.Question) == "" {
			return *r, true
		}
	}
	return FoundRequest{}, false
}
