package engine

import (
	"sort"
	"strings"
)

// RoleOverlap is one diversity reviewer role's line in the per-role overlap
// report (§8.5). Unique counts clusters where the role is the sole contributor;
// Shared counts clusters it is in with two or more contributing roles. A role
// with Unique 0 and Shared >= 1 — it found only findings others also found — is a
// TrimCandidate.
type RoleOverlap struct {
	Role          string `json:"role"`
	Unique        int    `json:"unique"`
	Shared        int    `json:"shared"`
	TrimCandidate bool   `json:"trim_candidate"`
}

// OverlapReport computes the per-role overlap report (§8.5) from the merged
// findings' found_by_roles sets: each element of clustersFoundByRoles is one
// cluster's contributing diversity roles. A role is credited unique for a cluster
// it solely contributed and shared for a cluster with >= 2 contributing roles. A
// role with unique 0 and shared >= 1 is flagged a trim candidate. The built-in
// regression role and blank roles are excluded (defensively re-checked here), so
// they are never trim targets; a role that found nothing never appears. There is
// no corpus-global ratio — the report needs only the merged findings. The result
// is sorted by role id.
func OverlapReport(clustersFoundByRoles [][]string) []RoleOverlap {
	type acc struct{ unique, shared int }
	stats := make(map[string]*acc)
	at := func(role string) *acc {
		a := stats[role]
		if a == nil {
			a = &acc{}
			stats[role] = a
		}
		return a
	}

	for _, roles := range clustersFoundByRoles {
		// Defensively drop regression/blank and de-duplicate within the cluster.
		seen := make(map[string]struct{})
		clean := make([]string, 0, len(roles))
		for _, r := range roles {
			r = strings.TrimSpace(r)
			if r == "" || r == RegressionRoleID {
				continue
			}
			if _, dup := seen[r]; dup {
				continue
			}
			seen[r] = struct{}{}
			clean = append(clean, r)
		}
		switch {
		case len(clean) == 1:
			at(clean[0]).unique++
		case len(clean) >= 2:
			for _, r := range clean {
				at(r).shared++
			}
		}
	}

	out := make([]RoleOverlap, 0, len(stats))
	for role, a := range stats {
		out = append(out, RoleOverlap{
			Role:          role,
			Unique:        a.unique,
			Shared:        a.shared,
			TrimCandidate: a.unique == 0 && a.shared >= 1,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out
}
