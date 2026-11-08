package engine

import "strings"

// NewEpicErrors returns the error findings of after that before does not
// hold (spec/1.3.0.md §3, decision D3 of spec/1.3.0-corrections.md). An
// error is identified by its rule, its sorted epic ids and the milestone,
// gate or key names it involves, counted as a multiset; a cycle whose
// component is a subset of a before-state component is not new, so a write
// that shrinks a cycle passes.
func NewEpicErrors(before, after []EpicLintFinding) []EpicLintFinding {
	held := map[string]int{}
	var cycles [][]string
	for i := range before {
		f := &before[i]
		if f.Severity != EpicSeverityError {
			continue
		}
		held[epicErrorKey(f)]++
		if f.Rule == EpicRuleCycle {
			cycles = append(cycles, f.Epics)
		}
	}
	var out []EpicLintFinding
	for i := range after {
		f := &after[i]
		if f.Severity != EpicSeverityError || f.Rule == EpicRuleCycle && withinAnyComponent(f.Epics, cycles) {
			continue
		}
		if key := epicErrorKey(f); held[key] > 0 {
			held[key]--
			continue
		}
		out = append(out, *f)
	}
	return out
}

func epicErrorKey(f *EpicLintFinding) string {
	return f.Rule + "\x00" + strings.Join(f.Epics, "\x01") + "\x00" + strings.Join(f.Names, "\x01")
}

// withinAnyComponent reports whether every id of comp is in one of the
// components.
func withinAnyComponent(comp []string, components [][]string) bool {
	for _, c := range components {
		in := make(map[string]bool, len(c))
		for _, id := range c {
			in[id] = true
		}
		all := true
		for _, id := range comp {
			all = all && in[id]
		}
		if all {
			return true
		}
	}
	return false
}
