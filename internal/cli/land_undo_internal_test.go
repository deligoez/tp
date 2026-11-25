package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A land whose take-back failed says so, naming the reset that finishes it,
// rather than reporting the task reopened over a branch that still carries
// the picks.
func TestWithUndoFailure_NamesTheResetThatFinishesIt(t *testing.T) {
	assert.Equal(t, "the gate failed", withUndoFailure("the gate failed", "abc123", nil))
	got := withUndoFailure("the gate failed", "abc123", errors.New("not uptodate"))
	assert.Equal(t, "the gate failed; taking the pick back failed (not uptodate): this branch still carries it, run git reset --keep abc123", got)
}
