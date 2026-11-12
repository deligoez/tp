package engine

import (
	"fmt"
	"strings"
)

// cappedListShown is how many names CappedList shows before it counts.
const cappedListShown = 3

// CappedList renders names joined by ", " while they are few, else their
// count, the noun, and the first few: "12 units (a, b, c, …)". A message
// naming every unit of a large slice would cost more than the decision it
// asks for. Never cap a list a hint tells the agent to act on, such as the
// roles whose files to write: the agent needs every name there.
func CappedList(names []string, noun string) string {
	if len(names) <= cappedListShown {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d %s (%s, …)", len(names), noun, strings.Join(names[:cappedListShown], ", "))
}
