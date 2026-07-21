package engine

import "strings"

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
