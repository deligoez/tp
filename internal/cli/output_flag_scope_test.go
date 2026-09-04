package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOutputFlagRequiresMerge pins that -o/--output is refused outside --merge.
// It is read only by the merge paths, so accepting it elsewhere silently dropped
// the caller's redirect target while the payload still went to stdout — the same
// silently-ignored-flag hazard the --merge rejection lists guard in the opposite
// direction.
func TestOutputFlagRequiresMerge(t *testing.T) {
	t.Parallel()
	writeSpec := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
			[]byte("# S\n## A\n| c |\n|---|\n| x |\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code)
		return dir
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"audit emission", []string{"audit", "spec.md", "-o", "out.json"}},
		{"review emission", []string{"review", "spec.md", "-o", "out.json"}},
		{"review status", []string{"review", "spec.md", "--status", "-o", "out.json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeSpec(t)
			_, stderr, code := runTP(t, dir, tc.args...)
			require.Equal(t, 2, code, "a flag that would be ignored is a usage error")
			require.Contains(t, stderr, "-o/--output requires --merge")
			require.NoFileExists(t, filepath.Join(dir, "out.json"),
				"the flag was refused, so nothing may be written there")
		})
	}

	// The merge paths still honour it — the guard must not break the one mode
	// that reads the flag.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"review merge", []string{"review", "--merge", "in.ndjson", "-o", "merged.ndjson"}},
		{"audit merge", []string{"audit", "--merge", "in.ndjson", "-o", "merged.ndjson"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeSpec(t)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "in.ndjson"),
				[]byte(`{"role":"r","severity":"low","location":"§1","finding":"f","item_id":"i","status":"PASS"}`+"\n"), 0o600))
			_, _, code := runTP(t, dir, tc.args...)
			require.Equal(t, 0, code)
			require.FileExists(t, filepath.Join(dir, "merged.ndjson"))
		})
	}
}
