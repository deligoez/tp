package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocationKey_TokenExtraction(t *testing.T) {
	cases := map[string]string{
		"§8":                       "§8",
		"§8.2":                     "§8.2",
		"§8.2.1":                   "§8.2.1",
		"§8 first sentence":        "§8",
		"§8.2 then §9 later":       "§8.2", // trailing text and later § tokens ignored
		"see §12.4 in the spec":    "§12.4",
		"  §8.2  ":                 "§8.2", // surrounding whitespace ignored
		"prose about §8.2 and §10": "§8.2",
	}
	for in, want := range cases {
		assert.Equal(t, want, LocationKey(in), "LocationKey(%q)", in)
	}
}

func TestLocationKey_ExactMatchDistinction(t *testing.T) {
	// §8 and §8.2 must be different keys and never cluster together.
	assert.NotEqual(t, LocationKey("§8"), LocationKey("§8.2"))
	assert.Equal(t, "§8", LocationKey("§8 overview"))
	assert.Equal(t, "§8.2", LocationKey("§8.2 detail"))
}

func TestLocationKey_NoTokenVerbatim(t *testing.T) {
	// A location without a § token uses its trimmed string verbatim.
	assert.Equal(t, "section 8", LocationKey("section 8"))
	assert.Equal(t, "README.md:42", LocationKey("  README.md:42  "))
	assert.Equal(t, "", LocationKey("   "))
	// A bare § with no digits is not a token; the trimmed string is verbatim.
	assert.Equal(t, "§ general", LocationKey("§ general"))
}
