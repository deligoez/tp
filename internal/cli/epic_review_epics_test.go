package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicChainGatedOnD is the chain with a gate on A whose member is d.
var epicChainGatedOnD = strings.Replace(epicChain, `"epics": [`,
	`"gates": {"A": {"members": ["d"]}},
  "epics": [`, 1)

// TestEpicReview_ASetSliceAsksItsEpicsAndAFullyListedGate: --epics b,d asks
// exactly b and d and leaves A's gate out, since a and c of A are not listed;
// listing every epic of A brings the gate in. A gate included when one epic
// of its milestone is listed fails the first half.
func TestEpicReview_ASetSliceAsksItsEpicsAndAFullyListedGate(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainGatedOnD)

	out := runEpicReview(t, dir, "--epics", "b,d")
	assert.Equal(t, []string{"epic:b", "epic:d"}, out.Units)

	all := runEpicReview(t, dir, "--epics", "a,b,c,d")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A"}, all.Units)
	assert.NotEqual(t, out.StateDir, all.StateDir, "an unnamed set is keyed by its ids")
}

// TestEpicReview_ANamedSetKeepsItsStateWhenItGrows: round 1 over --epics b
// --name topic is recorded clean; round 2 over --epics b,c --name topic is the
// same slice's round 2, carries the unchanged b and asks only c. Keying a named
// set by its ids would restart at round 1 and ask both.
func TestEpicReview_ANamedSetKeepsItsStateWhenItGrows(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	r1 := runEpicReview(t, dir, "--epics", "b", "--name", "topic")
	require.Equal(t, []string{"epic:b"}, r1.Units)
	writeEpicRound(t, dir, &r1, "")

	r2 := runEpicReview(t, dir, "--epics", "b,c", "--name", "topic")
	assert.Equal(t, r1.StateDir, r2.StateDir)
	assert.Equal(t, 2, r2.Round)
	assert.Equal(t, []string{"epic:c"}, r2.Units, "c is new to the slice")
	assert.Equal(t, []string{"epic:b"}, r2.Carried, "b is unchanged")
}

// TestEpicReview_SetSliceRefusals: an id holding a comma is quoted CSV and
// the flag may repeat; --epics x and --epic x are different slices; an empty
// list or --name without --epics is a usage error (2) and an absent id 4.
func TestEpicReview_SetSliceRefusals(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, strings.Replace(epicChain,
		`{"id": "d", "title": "D", "milestone": "A"}`,
		`{"id": "d", "title": "D", "milestone": "A"}, {"id": "p,q", "title": "PQ", "milestone": "A"}`, 1))

	quoted := runEpicReview(t, dir, "--epics", `"p,q"`, "--epics", "d")
	assert.Equal(t, []string{"epic:d", "epic:p,q"}, quoted.Units)

	set := runEpicReview(t, dir, "--epics", "b")
	one := runEpicReview(t, dir, "--epic", "b")
	assert.NotEqual(t, set.StateDir, one.StateDir)

	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"epic", "review", "--epics", ""}, 2},
		{[]string{"epic", "review", "--epic", "b", "--name", "t"}, 2},
		{[]string{"epic", "review", "--epics", "b", "--epic", "b"}, 2},
		{[]string{"epic", "review", "--epics", "b,zz"}, 4},
	} {
		_, stderr, code := runTP(t, dir, tc.args...)
		assert.Equal(t, tc.code, code, "%v: stderr: %s", tc.args, stderr)
	}
}
