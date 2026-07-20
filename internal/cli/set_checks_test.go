package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetWorkflowChecks_ReplaceAndValidate(t *testing.T) {
	t.Run("valid list replaces whole checks array", func(t *testing.T) {
		dir := setupProject(t)
		_, stderr, code := runTP(t, dir, "set", "--workflow",
			`checks=[{"class":"nil-slice","cmd":"grep -rn foo ."},{"class":"todo-scan","cmd":"grep -rn TODO ."}]`)
		require.Equal(t, 0, code, "set failed: %s", stderr)

		data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
		require.NoError(t, err)
		var tf struct {
			Workflow struct {
				Checks []map[string]string `json:"checks"`
			} `json:"workflow"`
		}
		require.NoError(t, json.Unmarshal(data, &tf))
		require.Len(t, tf.Workflow.Checks, 2)
		assert.Equal(t, "nil-slice", tf.Workflow.Checks[0]["class"])

		// Replace semantics: a new list fully overwrites the old one
		_, _, code = runTP(t, dir, "set", "--workflow", `checks=[{"class":"only-one","cmd":"true"}]`)
		require.Equal(t, 0, code)
		data, err = os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &tf))
		require.Len(t, tf.Workflow.Checks, 1)
		assert.Equal(t, "only-one", tf.Workflow.Checks[0]["class"])
	})

	t.Run("invalid entries reject whole write naming index", func(t *testing.T) {
		dir := setupProject(t)

		cases := []struct {
			value    string
			expected string
		}{
			{`checks=[{"class":"Bad_Slug","cmd":"true"}]`, "checks[0]"},
			{`checks=[{"class":"ok","cmd":"true"},{"class":"ok","cmd":"true"}]`, "checks[1]"},
			{`checks=[{"class":"ok","cmd":"true"},{"class":"empty-cmd","cmd":"  "}]`, "checks[1]"},
		}
		for _, tc := range cases {
			_, stderr, code := runTP(t, dir, "set", "--workflow", tc.value)
			assert.Equal(t, 1, code, "value %s must exit 1", tc.value)
			assert.Contains(t, stderr, tc.expected, "error names the offending index")
		}
	})
}
