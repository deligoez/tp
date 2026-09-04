package cli

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// quotedName matches the single-quoted perspective names in the refusal text.
var quotedName = regexp.MustCompile(`'([^']+)'`)

// TestInvalidPerspectiveMessageNamesExactlyTheAcceptedSet is the link that lets
// an external test derive the mode list from the code (§6.2 property 2) by
// reading a live refusal instead of restating the list.
//
// Without it, deriving from the message would be one indirection away from the
// truth: a perspective added to the validator but forgotten in the message
// would leave the clause-absence property silently unmeasured for that mode.
// Here the message is rendered FROM the slice, and this asserts the rendering
// round-trips.
func TestInvalidPerspectiveMessageNamesExactlyTheAcceptedSet(t *testing.T) {
	t.Parallel()
	msg := invalidPerspectiveMessage("zzz")

	named := make([]string, 0, len(reviewPerspectives))
	for _, m := range quotedName.FindAllStringSubmatch(msg, -1) {
		named = append(named, m[1])
	}

	assert.Equal(t, reviewPerspectives, named,
		"the refusal names every accepted perspective, in order")
	assert.Contains(t, msg, `"zzz"`, "the refusal repeats what the caller typed")
}
