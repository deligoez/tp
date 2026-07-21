package engine

import (
	"sort"
	"strings"
)

// RegressionRoleID is the reserved id of the built-in regression reviewer (§5.2).
// It is never a corpus file, never part of the overlap report, and never a trim
// target.
const RegressionRoleID = "regression"

// ClusterFinding is the minimal view of a review finding needed to cluster and
// attribute it (§8). Callers populate it from an NDJSON row; the full row is kept
// separately so the representative can be emitted verbatim.
type ClusterFinding struct {
	Location string
	Class    string
	Role     string
	Severity string
	Finding  string
}

// FindingCluster is a group of review findings that share a (location key, class)
// per §8.3. Members holds indices into the slice passed to ClusterFindings, in
// input order.
type FindingCluster struct {
	Members []int
}

// severityRanks orders severities from most to least severe; lower is higher.
var severityRanks = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

// SeverityRank ranks a severity for ordering: critical(0) < high(1) < medium(2) <
// low(3) < unknown(4). Lower rank means higher severity.
func SeverityRank(severity string) int {
	if r, ok := severityRanks[strings.ToLower(strings.TrimSpace(severity))]; ok {
		return r
	}
	return 4
}

// ClusterFindings groups review findings by (location key, class) (§8.3). Two
// findings share a cluster only when their location keys (§8.2) and their trimmed
// class are equal and the class is non-empty. A finding whose class is absent
// (empty after trim) forms its own singleton cluster and never merges, so an
// empty location key cannot collapse unrelated findings. Clusters are returned in
// order of first appearance.
func ClusterFindings(findings []ClusterFinding) []FindingCluster {
	clusters := make([]FindingCluster, 0)
	byKey := make(map[string]int) // composite cluster key -> index into clusters
	for i, f := range findings {
		class := strings.TrimSpace(f.Class)
		if class == "" {
			clusters = append(clusters, FindingCluster{Members: []int{i}})
			continue
		}
		key := LocationKey(f.Location) + "\x00" + class
		if ci, ok := byKey[key]; ok {
			clusters[ci].Members = append(clusters[ci].Members, i)
			continue
		}
		byKey[key] = len(clusters)
		clusters = append(clusters, FindingCluster{Members: []int{i}})
	}
	return clusters
}

// Representative returns the index (into findings) of the cluster member that
// supplies the cluster's displayed fields (§8.4). The total order is: highest
// severity, then lexicographic role, then finding text — deterministic even when
// two same-role members tie on severity. All members share the location key by
// construction, so it is never a tiebreaker. Returns -1 for an empty cluster.
func (c FindingCluster) Representative(findings []ClusterFinding) int {
	best := -1
	for _, idx := range c.Members {
		if best == -1 {
			best = idx
			continue
		}
		if lessRepresentative(&findings[idx], &findings[best]) {
			best = idx
		}
	}
	return best
}

// lessRepresentative reports whether a should win over b as representative.
func lessRepresentative(a, b *ClusterFinding) bool {
	ra, rb := SeverityRank(a.Severity), SeverityRank(b.Severity)
	if ra != rb {
		return ra < rb
	}
	if a.Role != b.Role {
		return a.Role < b.Role
	}
	return a.Finding < b.Finding
}

// Attribution returns the cluster's found_by_roles and found_by (§8.4):
// found_by_roles is the sorted set of distinct contributing diversity reviewer
// roles, excluding the built-in regression role and absent/blank roles; found_by
// is its cardinality. A cluster produced only by regression and/or absent-role
// findings therefore has found_by 0 and an empty found_by_roles.
func (c FindingCluster) Attribution(findings []ClusterFinding) (roles []string, count int) {
	set := make(map[string]struct{})
	for _, idx := range c.Members {
		role := strings.TrimSpace(findings[idx].Role)
		if role == "" || role == RegressionRoleID {
			continue
		}
		set[role] = struct{}{}
	}
	roles = make([]string, 0, len(set))
	for r := range set {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	return roles, len(roles)
}
