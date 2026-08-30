package engine

import "testing"

// TestPlaceholderShaped_CharacterClassBoundaries pins the exact character class
// section 3.2.1's placeholder syntax documents: an ASCII letter, then letters,
// digits, underscore or hyphen.
//
// A mutation run over the new engine surface left nine survivors here and
// nowhere else in it — every one a boundary on the comparisons below
// (`c >= '0'` to `c > '0'`, `c <= 'z'` to `c < 'z'`, and so on). The prompts
// this predicate actually sees carry names like log_path and unit_kind, so no
// existing test ever presented '0', '9', 'a', 'z', 'A' or 'Z' in the position
// where the bound decides. The code is right; the boundary was simply never
// asked about, which is why every one of those mutants lived.
//
// The distinction matters at the sink: a name this predicate rejects is left in
// the argv as a literal, and one it accepts must resolve or the run is a usage
// error before any child is spawned. Misplacing a bound therefore either
// substitutes into a JSON literal or lets an unresolvable token reach a spawn.
func TestPlaceholderShaped_CharacterClassBoundaries(t *testing.T) {
	shaped := []struct{ name, why string }{
		{"a", "lowest lowercase letter, alone"},
		{"z", "highest lowercase letter, alone"},
		{"A", "lowest uppercase letter, alone"},
		{"Z", "highest uppercase letter, alone"},
		{"a0", "digit 0 is the low bound of the digit class"},
		{"a9", "digit 9 is the high bound of the digit class"},
		{"z0", "high letter bound beside the low digit bound"},
		{"Z9", "high uppercase bound beside the high digit bound"},
		{"a_", "underscore is in the class"},
		{"a-", "hyphen is in the class"},
		{"log_path", "a real placeholder tp emits"},
		{"unit_kind", "a real placeholder tp emits"},
		{"aA0z9Z_-", "every accepted class in one name"},
	}
	for _, c := range shaped {
		if !placeholderShaped(c.name) {
			t.Errorf("placeholderShaped(%q) = false, want true: %s", c.name, c.why)
		}
	}

	unshaped := []struct{ name, why string }{
		{"", "empty is not a placeholder"},
		{"0", "a digit cannot open a placeholder"},
		{"9", "a digit cannot open a placeholder"},
		{"_x", "underscore cannot open a placeholder"},
		{"-x", "hyphen cannot open a placeholder"},
		{"`x", "backtick is one below 'a' and must stay out"},
		{"{x", "brace is one above 'z' and must stay out"},
		{"@x", "at-sign is one below 'A' and must stay out"},
		{"[x", "bracket is one above 'Z' and must stay out"},
		{"a/", "slash is one below '0' and must stay out"},
		{"a:", "colon is one above '9' and must stay out — this is the JSON case"},
		{"a b", "a space means a JSON fragment, not a placeholder"},
		{`a"`, "a quote means a JSON fragment, not a placeholder"},
	}
	for _, c := range unshaped {
		if placeholderShaped(c.name) {
			t.Errorf("placeholderShaped(%q) = true, want false: %s", c.name, c.why)
		}
	}
}
