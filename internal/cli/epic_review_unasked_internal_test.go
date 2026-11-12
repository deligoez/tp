package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestEpicUnaskedError_RefusesAUnitNoPromptAsks is the H1 guard: a round that
// lists a unit no prompt asks is refused at exit 4, naming the unit, and one
// whose every unit has a reader is not.
func TestEpicUnaskedError_RefusesAUnitNoPromptAsks(t *testing.T) {
	units := []engine.EpicReviewUnit{{Key: "epic:a"}, {Key: "scope:A"}}
	err := epicUnaskedError(units, map[string]bool{"epic:a": true})
	require.NotNil(t, err)
	assert.Equal(t, ExitState, err.code)
	assert.Contains(t, err.msg, "scope:A")
	assert.NotContains(t, err.msg, "epic:a")
	assert.Nil(t, epicUnaskedError(units, map[string]bool{"epic:a": true, "scope:A": true}))
}
