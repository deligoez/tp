package engine

import (
	"sort"
	"strings"
)

// LocationCluster is one location that more than one reviewer role reported on
// (§8a.1). --merge deduplicates on (location, class) and roles compose their
// class slugs independently, so two roles naming one defect almost never
// collide; this is the other cut of the same rows — grouped by location alone —
// and it exists so a reader can tell "one defect, four lenses" from "four
// defects". It is reporting only: nothing here feeds convergence arithmetic, the
// stored clean flag, or the --status --check exit code.
//
// Count is the number of merged records at the location, not the number of
// pre-merge rows: merge has already collapsed the exact (location, class)
// duplicates, and each surviving record is what a resolver still has to answer.
type LocationCluster struct {
	Location   string   `json:"location"`
	Roles      []string `json:"roles"`
	Severities []string `json:"severities"`
	Count      int      `json:"count"`
}

// LocationClusterRecord is one merged finding projected onto the fields §8a.1
// groups by. Roles is the record's contributing roles — found_by_roles for a
// merged record, its own role otherwise — so a cluster that already collapsed
// two roles into one record still reads as a multi-role location.
type LocationClusterRecord struct {
	Location string
	Roles    []string
	Severity string
}

// LocationClusters groups records by location key (§8.2) and returns one entry
// per location whose contributing roles number two or more, sorted by location.
// Within an entry, Roles is the distinct role set sorted lexicographically and
// Severities is the distinct severity set ordered most severe first (ties, and
// unrecognised severities, break lexicographically) — both deduplicated, so
// neither array is parallel to the records.
//
// The built-in regression role and blank roles are excluded, matching
// OverlapReport and FindingCluster.Attribution: regression re-reports findings
// a diversity role already raised, so counting it would report every re-raised
// location as multi-role. A record with an empty location key is skipped
// entirely rather than grouped under "", which would collapse unrelated
// findings the way §8.3 forbids for the merge key.
//
// The result is never nil: no clustered location yields an empty slice.
func LocationClusters(records []LocationClusterRecord) []LocationCluster {
	type acc struct {
		roles      map[string]struct{}
		severities map[string]struct{}
		count      int
	}
	order := make([]string, 0, len(records))
	byKey := make(map[string]*acc)

	for _, rec := range records {
		key := LocationKey(rec.Location)
		if key == "" {
			continue
		}
		a := byKey[key]
		if a == nil {
			a = &acc{roles: map[string]struct{}{}, severities: map[string]struct{}{}}
			byKey[key] = a
			order = append(order, key)
		}
		a.count++
		for _, role := range rec.Roles {
			role = strings.TrimSpace(role)
			if role == "" || role == RegressionRoleID {
				continue
			}
			a.roles[role] = struct{}{}
		}
		if sev := strings.TrimSpace(rec.Severity); sev != "" {
			a.severities[sev] = struct{}{}
		}
	}

	out := make([]LocationCluster, 0, len(order))
	for _, key := range order {
		a := byKey[key]
		if len(a.roles) < 2 {
			continue
		}
		out = append(out, LocationCluster{
			Location:   key,
			Roles:      sortedKeys(a.roles),
			Severities: sortedBySeverity(a.severities),
			Count:      a.count,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Location < out[j].Location })
	return out
}

// sortedKeys returns a set's members in lexicographic order.
func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedBySeverity returns a severity set ordered most severe first, breaking
// ties (and ordering unrecognised severities, which share one rank)
// lexicographically so the result is deterministic.
func sortedBySeverity(set map[string]struct{}) []string {
	out := sortedKeys(set)
	sort.SliceStable(out, func(i, j int) bool {
		return SeverityRank(out[i]) < SeverityRank(out[j])
	})
	return out
}
