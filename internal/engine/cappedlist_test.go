package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCappedList_CountsPastThree: three names read whole, a fourth turns the
// list into its count and the first three.
func TestCappedList_CountsPastThree(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a, b, c", CappedList([]string{"a", "b", "c"}, "roles"))
	assert.Equal(t, "4 roles (a, b, c, …)", CappedList([]string{"a", "b", "c", "d"}, "roles"))
}
