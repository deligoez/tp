package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRole_Fields(t *testing.T) {
	r := Role{
		ID:           "security",
		Title:        "Security reviewer",
		Instructions: "Examine the spec for security gaps.",
		Focus:        []string{"Any injection?", "Any missing authz?"},
		Domains:      []string{"software"},
	}

	assert.Equal(t, "security", r.ID)
	assert.Equal(t, "Security reviewer", r.Title)
	assert.Equal(t, "Examine the spec for security gaps.", r.Instructions)
	assert.Equal(t, []string{"Any injection?", "Any missing authz?"}, r.Focus)
	assert.Equal(t, []string{"software"}, r.Domains)
}

func TestRole_JSONKeyIsFocusNotLens(t *testing.T) {
	r := Role{
		ID:           "coherence",
		Title:        "Coherence reviewer",
		Instructions: "Check internal consistency.",
		Focus:        []string{"Does it contradict itself?"},
	}

	data, err := json.Marshal(r)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, `"focus"`, "the focus questions must serialize under the focus key")
	assert.NotContains(t, s, `"lens"`, "the schema field must never serialize as lens")
}

func TestRole_OptionalFieldsOmitted(t *testing.T) {
	r := Role{ID: "tester", Title: "Tester", Instructions: "Test it."}

	data, err := json.Marshal(r)
	require.NoError(t, err)
	s := string(data)

	// focus and domains are optional; absent when empty.
	assert.NotContains(t, s, "focus")
	assert.NotContains(t, s, "domains")
}

func TestRole_RoundTrip(t *testing.T) {
	in := `{"id":"architect","title":"Architect","instructions":"Judge the design.","focus":["Over-engineered?"],"domains":["software","prose"]}`

	var r Role
	require.NoError(t, json.Unmarshal([]byte(in), &r))
	assert.Equal(t, "architect", r.ID)
	assert.Len(t, r.Focus, 1)
	assert.Equal(t, []string{"software", "prose"}, r.Domains)

	out, err := json.Marshal(r)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(out), `"id":"architect"`))
}
