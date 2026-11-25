package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// InFlightAuditRound is the audit round emitted for specPath and not yet
// recorded, 0 when none is in flight.
func InFlightAuditRound(specPath string) (int, error) {
	st, err := LoadReviewState(specPath)
	if err != nil && !IsRebuildableStateIndex(err) {
		return 0, err
	}
	recorded := 0
	if err == nil && st != nil {
		recorded = len(st.AuditRounds)
	}
	return InFlightRound(specPath, PhaseAudit, recorded), nil
}

// FoundAuditFinder names the finder of a request an audit-role unit files
// from one of its rows (spec/1.31.0.md §7): the audit round in flight, so
// tp found apply holds the request until that round is recorded, and the
// row's role:item_id key. A row on an item the round asked the role keeps the
// audit-fix path and never becomes a request, so such a key is refused, as are
// a key that is not role:item_id, a spec with no audit round in flight, and a
// round whose asked items were not kept (nothing shows the item was not asked).
func FoundAuditFinder(specPath, rowKey string) (FoundFinder, error) {
	role, item, ok := strings.Cut(rowKey, ":")
	role, item = strings.TrimSpace(role), strings.TrimSpace(item)
	if !ok || role == "" || item == "" {
		return FoundFinder{}, fmt.Errorf("--audit-row %q is not a row key; name the row as role:item_id", rowKey)
	}
	round, err := InFlightAuditRound(specPath)
	if err != nil {
		return FoundFinder{}, err
	}
	if round == 0 {
		return FoundFinder{}, errors.New("no audit round is in flight, so no audit row can file a request")
	}
	pools, found, err := ReadAuditPools(specPath, round)
	if err != nil {
		return FoundFinder{}, err
	}
	if !found {
		return FoundFinder{}, fmt.Errorf("audit round %d kept no record of what it asked, so no row can be shown to lie outside it", round)
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
