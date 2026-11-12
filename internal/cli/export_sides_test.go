package cli

import "sort"

// TreeModes exposes the tree-side modes of commandSides to the external test
// package, so the test that drives each one with a foreign task file every
// way it can be given one enumerates the same set the code refuses from.
func TreeModes() []string {
	modes := make([]string, 0)
	for mode, side := range commandSides {
		if side == sideTree {
			modes = append(modes, mode)
		}
	}
	sort.Strings(modes)
	return modes
}
