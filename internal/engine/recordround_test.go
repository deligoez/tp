package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Section 6.3: TP_ROUND identifies the round a record unit re-records, and
// nothing else may be read as a rewrite — hand recording must stay additive.
func TestRecordTargetRound(t *testing.T) {
	for _, tc := range []struct {
		name     string
		env      string
		recorded int
		round    int
		rewrite  bool
	}{
		{"absent, nothing recorded", "", 0, 1, false},
		{"absent, two recorded", "", 2, 3, false},
		{"names the latest recorded round", "2", 2, 2, true},
		{"names an earlier recorded round", "1", 3, 1, true},
		{"names the round about to be created", "3", 2, 3, false},
		{"past every recorded round", "9", 2, 3, false},
		{"zero", "0", 2, 3, false},
		{"negative", "-1", 2, 3, false},
		{"not a number", "r2", 2, 3, false},
		{"padded with a space", " 2", 2, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			round, rewrite := RecordTargetRound(tc.env, tc.recorded)
			assert.Equal(t, tc.round, round)
			assert.Equal(t, tc.rewrite, rewrite)
		})
	}
}

func TestRecordRound_ReadsItsOwnEnvironment(t *testing.T) {
	round, rewrite := RecordRound(2)
	assert.Equal(t, 3, round, "with no TP_ROUND in the environment a record appends")
	assert.False(t, rewrite)

	t.Setenv(EnvRound, "2")
	round, rewrite = RecordRound(2)
	assert.Equal(t, 2, round)
	assert.True(t, rewrite, "TP_ROUND naming a recorded round makes the call a rewrite")
}
