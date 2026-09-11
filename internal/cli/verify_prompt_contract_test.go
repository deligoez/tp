package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifyPromptOutputContractNamesRequiredKeys: §5 row 8 for the one listed
// command that carried no JSON object line at all. A verifier's findings file
// reaches tp review --merge and tp review <spec> --record like any other, so a
// row copied out of this prompt has to survive both gates — which, since the
// required set gained evidence, it could not, because there was nothing to copy.
//
// The verdict rests on the PARSED key set of the emitted object line, never on a
// substring search of the prompt: the fixture below pins why. Its one fixed
// finding carries resolution evidence, which buildVerifyPrompt renders into the
// "Fixed findings to verify" section, so strings.Contains(prompt, "evidence") is
// true with no output contract present at all.
func TestVerifyPromptOutputContractNamesRequiredKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	specPath := filepath.Join(dir, "spec.md")
	specText := "# Spec\n\n## Section\n\nContent with no JSON in it.\n"
	require.NoError(t, os.WriteFile(specPath, []byte(specText), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(
		`{"severity":"high","finding":"missing bound","location":"## Section","evidence":"read the cited section","category":"completeness","resolved":{"status":"fixed","evidence":"bound stated in section 2"}}`+"\n",
	), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--verify", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)

	// Every line of the prompt that is itself a JSON object. The fixture holds
	// none — neither the spec nor any finding text is JSON — so whatever this
	// collects came from the output contract.
	var objects []map[string]any
	var objectLines []string
	for line := range strings.SplitSeq(prompt, "\n") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &obj); err == nil && obj != nil {
			objects = append(objects, obj)
			objectLines = append(objectLines, line)
		}
	}
	require.Len(t, objects, 1, "the verify prompt carries exactly one JSON object line — the one a verifier copies")

	keys := make([]string, 0, len(objects[0]))
	for k := range objects[0] {
		keys = append(keys, k)
	}
	for _, required := range []string{"severity", "finding", "location", "evidence"} {
		assert.Contains(t, keys, required, "the output format names every key both gates require")
	}
	assert.NotContains(t, keys, "role",
		`role: "verifier" is a separate stamp no gate requires; inside the key list it would read as a fifth required key`)

	// Why the assertion above is made on the parsed keys: with the object line
	// removed the word still occurs in the prompt, so a substring search passes
	// against a prompt that carries no contract at all.
	rest := strings.Replace(prompt, objectLines[0], "", 1)
	require.Contains(t, rest, "evidence",
		"the fixture must keep the word reachable outside the object line, or this test proves nothing about substring searches")
}
