package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheGroundRowRefusalsCiteNoSection holds every refusal a unit meets at
// --record to plain words. A unit reads these to repair its row, and "§7.2's
// table" or "§8 carries" names a section of tp's own design document it has
// never been shown, so the rule has to be in the sentence itself.
//
// Each case is produced the way a unit produces it — a row through
// ParseGroundRow, or a row against an emitted floor's join key — so the
// assertion is over the text that actually reaches the operator.
func TestTheGroundRowRefusalsCiteNoSection(t *testing.T) {
	t.Parallel()
	parse := func(t *testing.T, set map[string]any) error {
		t.Helper()
		_, err := ParseGroundRow(groundWireRow(t, groundClaimRow(), set, nil))
		return err
	}
	unitID := "u7"
	cases := []struct {
		name string
		err  func(t *testing.T) error
	}{
		{"an unknown key", func(t *testing.T) error { return parse(t, map[string]any{"carried_frm": 1}) }},
		{"a unit_id that is not u<N>", func(t *testing.T) error { return parse(t, map[string]any{"unit_id": "3"}) }},
		{"a tier the kind refuses", func(t *testing.T) error {
			return parse(t, map[string]any{"kind": "corpus", "tier": "read"})
		}},
		{"an ordinal the floor does not give the unit", func(t *testing.T) error {
			row := &GroundRow{UnitID: &unitID, TextSHA: "0123456789ab", Ordinal: 2}
			return groundRowMatchesFloor(row, map[string]groundJoinKey{unitID: {textSHA: "0123456789ab", ordinal: 1}})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.err(t)
			require.Error(t, err, "the case must be a refused row, or it tests nothing")
			assert.NotContains(t, err.Error(), "§", "the refusal says its rule in plain words")
		})
	}
}

// TestNoGroundRefusalTextCitesASection is the guard over the text itself, for
// the one input the cases above cannot reach: a format string whose citation
// sits in a branch no fixture takes. The list is the refusals that once cited
// a section, named constant by constant — a bounded set, not a scan of every
// string in the package, whose prose legitimately names sections in comments.
func TestNoGroundRefusalTextCitesASection(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"groundTierRefusalFmt":     groundTierRefusalFmt,
		"groundUnknownKeyMsg":      groundUnknownKeyMsg,
		"groundUnitIDShapeMsg":     groundUnitIDShapeMsg,
		"groundOrdinalMismatchFmt": groundOrdinalMismatchFmt,
	} {
		assert.NotContains(t, text, "§", "%s cites no section of tp's design", name)
	}
}
