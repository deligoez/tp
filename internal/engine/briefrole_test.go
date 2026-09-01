package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoleUnitBriefNamesItsOwnRole pins v0.36.0 §4.2.3: a role unit's brief
// asks for that unit's own prompt rather than the whole panel.
//
// Without it the flag buys nothing on the driver path, which is the path §4
// measures: every unit runs `tp review <spec>`, receives every role's prompt
// and reads one. The saving is unreachable until the brief carries the name.
func TestRoleUnitBriefNamesItsOwnRole(t *testing.T) {
	target := UnitTarget{Spec: "spec/0.36.0.md", ID: "architect"}

	assert.Equal(t, "tp review spec/0.36.0.md --role architect",
		UnitReviewRole.BriefCommand(target),
		"a review-role unit asks for its own role")
	assert.Equal(t, "tp audit spec/0.36.0.md --role architect",
		UnitAuditRole.BriefCommand(target),
		"an audit-role unit asks for its own role")
}

// TestRoleUnitBriefWithoutAnIDStaysWhole guards the direction that would break
// every other caller: the two role kinds are the only ones that gain the flag,
// and a target with no ID must not emit a dangling `--role`.
func TestRoleUnitBriefWithoutAnIDStaysWhole(t *testing.T) {
	target := UnitTarget{Spec: "spec/0.36.0.md"}

	assert.Equal(t, "tp review spec/0.36.0.md", UnitReviewRole.BriefCommand(target))
	assert.Equal(t, "tp audit spec/0.36.0.md", UnitAuditRole.BriefCommand(target))
}

// TestBuildNextUnitsBriefsCarryTheirOwnRole is the end-to-end half: the oracle
// is what fills the target, and a brief that named the right role only in a
// unit test would still hand every driver-spawned unit the whole panel.
func TestBuildNextUnitsBriefsCarryTheirOwnRole(t *testing.T) {
	units := []NextUnit{}
	for _, id := range []string{"architect", "tester"} {
		units = append(units, newNextUnit(UnitReviewRole, id, UnitTarget{Spec: "spec/x.md"}))
	}

	require.Len(t, units, 2)
	for _, u := range units {
		assert.Contains(t, u.BriefCommand, "--role "+u.ID,
			"newNextUnit must put the unit's id where BriefCommand can read it")
	}
}
