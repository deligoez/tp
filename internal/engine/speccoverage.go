package engine

// SpecCoverageCleanRounds reports §2.3's spec_coverage_clean_rounds over the
// role_streaks of one audit history: the spec-coverage role's consecutive_clean,
// or nil when the latest recorded audit round contributes no row attributed to
// that id.
//
// It reads the streaks ComputeAuditRoleStreaks already returned rather than
// walking the rounds again. The entries of that array are exactly the roles of
// the latest round in §2.1's sense of "contributes": a round that contributes no
// rows — including one whose rows are readable but whose stored roles_hash is
// empty — yields no entries at all, so every path §2.3 lists reaches nil through
// the same absence, and a second read of the same files (with the advisory it
// would re-fire) is avoided.
//
// The result is a *int rather than an int precisely so nil and 0 stay distinct
// at the JSON boundary: 0 means the role was measured in the latest round and at
// least one of its rows is not PASS, nil means the latest round did not measure
// conformance at all. A caller emits the field without omitempty, so nil is an
// explicit JSON null and the key is never absent.
//
// The returned pointer addresses a copy, never an element of streaks, so a
// caller cannot mutate the array through it.
func SpecCoverageCleanRounds(streaks []RoleStreak) *int {
	for _, s := range streaks {
		if s.Role == RoleSpecCoverage {
			clean := s.ConsecutiveClean
			return &clean
		}
	}
	return nil
}
