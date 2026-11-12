package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

// epicScopeFields are the scope block's field lines a change lists old → new,
// in block order; the closure line is listed as ids added and removed.
var epicScopeFields = []string{"members", "deferrable", "threshold", "purpose"}

// epicScopeChangeText renders, for each asked scope unit whose block changed
// since the round before, the change itself: each differing field old → new
// and the closure as +added and −removed ids. The block now is already shown
// as the asked unit, so neither copy is repeated.
func epicScopeChangeText(run *epicReviewRun, units []engine.EpicReviewUnit) string {
	var b strings.Builder
	for _, u := range units {
		before, ok := run.carry.Before[u.Key]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "\nChanged since round %d — %s, field by field:\n", run.round-1, u.Key)
		was, now := epicScopeBlockFields(before), epicScopeBlockFields(u.Block)
		for _, f := range epicScopeFields {
			if was[f] != now[f] {
				fmt.Fprintf(&b, "- %s: %s → %s\n", f, epicScopeValue(was[f]), epicScopeValue(now[f]))
			}
		}
		if diff := epicClosureDiff(was["closure"], now["closure"]); diff != "" {
			b.WriteString("- closure: " + diff + "\n")
		}
	}
	return b.String()
}

// epicScopeBlockFields reads a scope block's "key: value" lines; the closure
// line is keyed "closure" under its status-blind label and the older bare one.
func epicScopeBlockFields(block string) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(block, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		if strings.HasPrefix(key, "closure") {
			key = "closure"
		}
		out[key] = value
	}
	return out
}

// epicScopeValue renders an absent field as (none).
func epicScopeValue(v string) string {
	if v == "" {
		return "(none)"
	}
	return v
}

// epicClosureDiff renders two comma-separated id lists' difference as the
// ids added, each +id, then the ids removed, each −id; "" when equal.
func epicClosureDiff(was, now string) string {
	before, after := strings.Split(was, ", "), strings.Split(now, ", ")
	var parts []string
	for _, id := range after {
		if id != "" && !slices.Contains(before, id) {
			parts = append(parts, "+"+id)
		}
	}
	for _, id := range before {
		if id != "" && !slices.Contains(after, id) {
			parts = append(parts, "−"+id)
		}
	}
	return strings.Join(parts, ", ")
}
