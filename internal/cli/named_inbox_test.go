package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// The tests below are spec/backlog/audit-round-ergonomics.md §2.2: --inbox
// <dir> names the directory a round's role files are written to and read
// from, in place of the spec's state directory, so a role told the checkout is
// read-only has somewhere to write. The default stays where it is.

// realPath resolves a directory's symlinks, so a temp dir under /var and its
// /private/var spelling compare equal.
func realPath(t *testing.T, dir string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return p
}

// TestNamedInbox_EmissionNamesTheInbox: with --inbox, each emitted prompt's
// output_path is <dir>/<phase>-inbox/<spec base>/round-<N>-<role>.ndjson, absolute even
// when the flag was relative, and the prompt's own line names the same file.
func TestNamedInbox_EmissionNamesTheInbox(t *testing.T) {
	t.Parallel()
	dir, specPath, codePath := emissionFixture(t)
	for _, tc := range []struct {
		phase string
		args  []string
	}{
		{"review", []string{"review", specPath, "--no-state", "--inbox", "outside"}},
		{"audit", []string{"audit", specPath, "--affected-files", codePath, "--inbox", "outside"}},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			prompts := emittedPrompts(t, dir, noRoundDir(), tc.args...)
			require.NotEmpty(t, prompts)
			inbox := filepath.Join(dir, "outside", tc.phase+"-inbox", engine.SpecBaseName(specPath))
			require.DirExists(t, inbox, "the emission makes the inbox a role can write into")
			for _, pm := range prompts {
				got := pm["output_path"].(string)
				assert.True(t, filepath.IsAbs(got), "the path is absolute: %s", got)
				assert.Equal(t, "round-1-"+pm["role"].(string)+".ndjson", filepath.Base(got))
				assert.Equal(t, realPath(t, inbox), realPath(t, filepath.Dir(got)), "the role file is in the named inbox")
				assert.Contains(t, pm["prompt"].(string), "Write this round's findings to: "+got)
			}
		})
	}
}

// TestNamedInbox_TheRunsRoundDirStillWins: under tp run the driver's round
// directory is the one path a role may write, so it outranks --inbox.
func TestNamedInbox_TheRunsRoundDirStillWins(t *testing.T) {
	t.Parallel()
	dir, specPath, _ := emissionFixture(t)
	roundDir := filepath.Join(dir, ".tp", "rounds", "spec", "review-r1")
	prompts := emittedPrompts(t, dir, []string{engine.EnvRoundDir + "=" + roundDir}, "review", specPath, "--no-state", "--inbox", "outside")
	for _, pm := range prompts {
		assert.Equal(t, engine.RoleFindingsPartPath(roundDir, pm["role"].(string)), pm["output_path"].(string))
	}
}

// TestNamedInbox_RecordInboxReadsTheNamedInbox: a review round emitted with
// --inbox records from that inbox with --record-inbox --inbox.
func TestNamedInbox_RecordInboxReadsTheNamedInbox(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md", "--inbox", "outside")
	for _, pm := range prompts {
		require.NoError(t, os.WriteFile(pm["output_path"].(string), nil, 0o600))
	}

	_, stderr, code := runTPEnv(t, dir, noRoundDir(), "review", "spec.md", "--record-inbox", "--inbox", "outside")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.FileExists(t, filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
}

// TestNamedInbox_ARecordThatForgetsTheInboxNamesTheDirectoryRead: the files
// are in the named inbox, and a --record-inbox without --inbox reads the
// default one; it refuses, recording nothing, and names the directory it read
// so the missing flag is visible.
func TestNamedInbox_ARecordThatForgetsTheInboxNamesTheDirectoryRead(t *testing.T) {
	t.Parallel()
	dir, spec := inboxSpec(t)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md", "--inbox", "outside")
	for _, pm := range prompts {
		require.NoError(t, os.WriteFile(pm["output_path"].(string), nil, 0o600))
	}

	_, stderr, code := runTPEnv(t, dir, noRoundDir(), "review", "spec.md", "--record-inbox")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, realPath(t, filepath.Dir(engine.RoleInboxPath(spec, engine.PhaseReview, 1, "x"))), "the refusal names the inbox it read, absolute")
	assert.Contains(t, stderr, "--inbox", "and says how to read another")
	assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
}

// TestNamedInbox_ARecordedRoundLeavesNoFileForTheNext: the files a record
// read leave the named inbox with it; another spec's round 1
// emitted into the same inbox finds none of them and is refused until its own
// roles write theirs.
func TestNamedInbox_ARecordedRoundLeavesNoFileForTheNext(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other\n## 1. B\ncontent\n"), 0o600))
	for _, pm := range emittedPrompts(t, dir, noRoundDir(), "review", "spec.md", "--inbox", "outside") {
		require.NoError(t, os.WriteFile(pm["output_path"].(string), nil, 0o600))
	}
	_, stderr, code := runTPEnv(t, dir, noRoundDir(), "review", "spec.md", "--record-inbox", "--inbox", "outside")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	emittedPrompts(t, dir, noRoundDir(), "review", "other.md", "--inbox", "outside")
	_, stderr, code = runTPEnv(t, dir, noRoundDir(), "review", "other.md", "--record-inbox", "--inbox", "outside")
	assert.Equal(t, 3, code, "spec.md's recorded files are not other.md's: %s", stderr)
}

// TestNamedInbox_TwoSpecsInFlightShareOneInbox: two specs' rounds emitted into
// one named inbox before either is recorded write apart, and each record reads
// and removes only its own spec's role files.
func TestNamedInbox_TwoSpecsInFlightShareOneInbox(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other\n## 1. B\ncontent\n"), 0o600))
	paths := map[string][]string{}
	for _, spec := range []string{"spec.md", "other.md"} {
		for _, pm := range emittedPrompts(t, dir, noRoundDir(), "review", spec, "--inbox", "outside") {
			paths[spec] = append(paths[spec], pm["output_path"].(string))
		}
		require.NotEmpty(t, paths[spec], spec)
	}
	for _, p := range paths["spec.md"] {
		assert.NotContains(t, paths["other.md"], p, "two specs' role files never share a path")
	}
	for spec, ps := range paths {
		for i, p := range ps {
			body := ""
			if i == 0 {
				body = `{"severity":"low","finding":"only in ` + spec + `","location":"§1","evidence":"e"}` + "\n"
			}
			require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
		}
	}

	for _, tc := range []struct{ spec, other, base string }{{"spec.md", "other.md", "spec"}, {"other.md", "spec.md", "other"}} {
		_, stderr, code := runTPEnv(t, dir, noRoundDir(), "review", tc.spec, "--record-inbox", "--inbox", "outside")
		require.Equal(t, 0, code, "%s: %s", tc.spec, stderr)
		round, err := os.ReadFile(filepath.Join(dir, ".tp-review", tc.base, "review-round-1.ndjson"))
		require.NoError(t, err)
		assert.Contains(t, string(round), "only in "+tc.spec, "%s records its own row", tc.spec)
		assert.NotContains(t, string(round), "only in "+tc.other, "%s records none of %s's rows", tc.spec, tc.other)
		for _, p := range paths[tc.spec] {
			assert.NoFileExists(t, p, "%s's record removes its own files", tc.spec)
		}
		assert.NoDirExists(t, filepath.Dir(paths[tc.spec][0]), "and its emptied folder")
	}
}
