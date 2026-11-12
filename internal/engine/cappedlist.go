package engine

import (
	"fmt"
	"strings"
)

// cappedListShown is how many names CappedList shows before it counts.
const cappedListShown = 3

// CappedList renders names joined by ", " while they are few, else their
// count and the first few: "12 (a, b, c, …)". A message naming every unit
// of a large slice would cost more than the decision it asks for.
func CappedList(names []string) string {
	if len(names) <= cappedListShown {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d (%s, …)", len(names), strings.Join(names[:cappedListShown], ", "))
}
