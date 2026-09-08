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

// TestReviewMerge_OutputPathThatIsADirectoryNamesTheDirectory restores the
// diagnosis the temp-file-and-rename write lost.
//
// Measured, same tree, same invocation, two binaries: at bcf5bf3b^ (the single
// os.WriteFile this replaced) `-o` naming an existing directory reported
// "cannot write output file: open outdirA: is a directory". At HEAD it reported
// "cannot write output file: rename ./outdirA.tp-merge-1877366751 outdirA: file
// exists" — with the standing hint "check -o/--output: its directory must exist
// and be writable", every word of which is true of that path, so the pair
// pointed at nothing wrong. Both exit 3; only the diagnosis changed.
//
// The exit code is unchanged and therefore proves nothing on its own, so the
// message is what this test is for. The second read-back is what makes it more
// than a string check: no temporary is left in the directory the merge wrote
// into, because with the check in front of it none is ever created.
func TestReviewMerge_OutputPathThatIsADirectoryNamesTheDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "outdirA"), 0o755))
	role := writeFindingsFile(t, dir, "role-a.ndjson", []string{
		`{"role":"implementer","severity":"high","class":"gap","location":"§1",` +
			`"finding":"the bound is unstated","evidence":"read §1"}`,
	})

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", "outdirA", role)
	require.Equal(t, 3, code, "an unwritable -o is a file error: %s", stderr)

	var payload struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload),
		"the refusal must be a JSON envelope: %s", stderr)
	assert.Contains(t, payload.Error, "is a directory",
		"the message names what is actually wrong with the path")
	assert.NotContains(t, payload.Error, "rename",
		"a rename that failed because the destination is a directory reads as a tp bug, not a bad -o")

	// The check runs before the temporary is created, so nothing is left
	// behind in the directory the merge would have written into.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tp-merge-",
			"no temporary is created when the path is refused up front")
	}
}
