package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// groundFixtureSpec is the spec every emit test starts from. Its three units
// exercise the two halves of §2.2's index in one document: two floor units —
// one reached through the digit arm, one through the identifier arm — and, sat
// BETWEEN them, a unit all three arms cut, which is the only arrangement in
// which a cut row can be seen to occupy an id rather than to be appended.
const groundFixtureSpec = `# Fixture spec

## 1. Numbers

The gate runs 4 steps.

Prose with no signal at all.

## 2. Names

` + "`internal/cli/ground.go`" + ` holds the command.
`

// groundFixtureEdit is appended to the fixture by the tests that need the
// spec's floor to move after an emission.
const groundFixtureEdit = `
## 3. Later

A fourth unit measured 9 things.
`

// writeGroundFixture puts the fixture spec in a fresh directory and returns the
// directory, so a test names the spec by the relative path tp is run with.
func writeGroundFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(groundFixtureSpec), 0o600))
	return dir
}

// expectedFloorIndex renders the index tp must have written for text: the
// derivation is the engine's own, so the assertion pins that the command emits
// THAT floor rather than restating one here in a literal that could drift from
// it. The commit is empty because a t.TempDir() is not a git work tree, so the
// index's first line must read "# commit unknown" — a mutant discovering the
// commit from the process's own repository fails on that line.
func expectedFloorIndex(text string) string {
	return engine.FormatFloorIndex("", engine.FloorIndexRows(text, engine.FloorAnchorOf(text)))
}

// stateDirNames lists the state directory's entries, sorted.
func stateDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".tp-review", "spec"))
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// groundEmit runs one emission and returns the decoded payload.
func groundEmit(t *testing.T, dir string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out
}

// TestGroundEmitCreatesTheStateDirectoryAndWritesTheSnapshotAndTheFloor is §11
// row 12's emit half: an emission on a spec with no spec/.tp-review/<base>/
// creates it, and what it leaves there is the snapshot and the floor derived
// from it — and nothing else.
//
// The directory listing is asserted as a SET rather than by stating each file
// exists, because row 12's other clause is negative: no state.json. A pair of
// os.Stat calls cannot see a third file appear, and the file this release must
// not write is exactly the one a future edit would add without noticing.
func TestGroundEmitCreatesTheStateDirectoryAndWritesTheSnapshotAndTheFloor(t *testing.T) {
	dir := writeGroundFixture(t)
	_, err := os.Stat(filepath.Join(dir, ".tp-review"))
	require.True(t, os.IsNotExist(err), "the fixture must start with no state directory")

	out := groundEmit(t, dir)

	assert.Equal(t, []string{"floor-ground-round-1.txt", "snapshot-ground-round-1.md"},
		stateDirNames(t, dir), "an emission writes those two files and creates no state.json")

	snapshot, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "snapshot-ground-round-1.md"))
	require.NoError(t, err)
	assert.Equal(t, groundFixtureSpec, string(snapshot), "the snapshot is the spec's bytes as the round read them")

	floor, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "floor-ground-round-1.txt"))
	require.NoError(t, err)
	assert.Equal(t, expectedFloorIndex(groundFixtureSpec), string(floor),
		"the floor is §2.2's index over the snapshot's text")
	assert.Contains(t, string(floor), " (cut)", "the fixture must reach the cut branch")

	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, filepath.Join(".tp-review", "spec", "snapshot-ground-round-1.md"), out["snapshot"])
	assert.Equal(t, filepath.Join(".tp-review", "spec", "floor-ground-round-1.txt"), out["floor"])
}

// TestGroundEmitNamesTheScratchFileAndNotTheRecordedRound pins §7.3's two
// filenames apart: the prompt's output_path is ground-r<N>.ndjson, the scratch
// file a unit writes, while ground-round-<N>.ndjson is what --record writes
// into the state directory. A reader deriving the emitted name from §7.1's
// table alone gets it wrong, so the recorded name is the mutant this fails on.
//
// The second round is what makes the assertion say anything: r1 and round-1
// differ by four characters that a fixture at round 1 could still confuse with
// a hard-coded string, and the round file planted here is also the input that
// makes N advance.
func TestGroundEmitNamesTheScratchFileAndNotTheRecordedRound(t *testing.T) {
	dir := writeGroundFixture(t)

	out := groundEmit(t, dir)
	assert.Equal(t, "ground-r1.ndjson", out["output_path"])
	prompt, ok := out["prompt"].(string)
	require.True(t, ok, "one prompt, as a string, not a panel: %T", out["prompt"])
	assert.NotContains(t, out, "prompts", "grounding emits one prompt, never an array of role prompts")
	assert.Contains(t, prompt, "ground-r1.ndjson", "the prompt body names the file it writes")

	// A recorded round 1 — the only artifact that numbers a round.
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".tp-review", "spec", "ground-round-1.ndjson"), []byte("{}\n"), 0o600))

	out = groundEmit(t, dir)
	assert.Equal(t, float64(2), out["round"])
	assert.Equal(t, "ground-r2.ndjson", out["output_path"])
	assert.Equal(t, []string{
		"floor-ground-round-1.txt", "floor-ground-round-2.txt",
		"ground-round-1.ndjson",
		"snapshot-ground-round-1.md", "snapshot-ground-round-2.md",
	}, stateDirNames(t, dir), "round 2's emission writes beside round 1's artifacts, not over them")
}

// TestTheFloorOnDiskIsFrozenUntilTheNextEmission is §11 row 19: the snapshot
// and the floor are written at EMIT, so editing the spec afterwards cannot
// change what a later --record validates against. Row 19's named mutant —
// write the snapshot at record — leaves both files absent here.
//
// The require in the middle is what makes the assertion discriminating: an
// edit whose floor is identical to the original's would let a re-derivation
// pass this test. The fixture asserts that its own edit moves the floor rather
// than assuming it.
func TestTheFloorOnDiskIsFrozenUntilTheNextEmission(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)

	snapshotPath := filepath.Join(dir, ".tp-review", "spec", "snapshot-ground-round-1.md")
	floorPath := filepath.Join(dir, ".tp-review", "spec", "floor-ground-round-1.txt")
	emitted, err := os.ReadFile(floorPath)
	require.NoError(t, err)

	edited := groundFixtureSpec + groundFixtureEdit
	require.NotEqual(t, expectedFloorIndex(edited), string(emitted),
		"the edit must move the floor, or this test cannot tell the two derivations apart")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(edited), 0o600))

	frozen, err := os.ReadFile(floorPath)
	require.NoError(t, err)
	assert.Equal(t, string(emitted), string(frozen), "editing the spec does not re-floor the emitted round")
	snapshot, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)
	assert.Equal(t, groundFixtureSpec, string(snapshot), "the snapshot still holds the text the round read")

	// The one thing that does move it: emitting again. Nothing has been
	// recorded, so this is round 1 a second time, and both artifacts are
	// rewritten from the spec as it now stands.
	groundEmit(t, dir)
	reEmitted, err := os.ReadFile(floorPath)
	require.NoError(t, err)
	assert.Equal(t, expectedFloorIndex(edited), string(reEmitted), "a re-emission of the same round re-floors it")
	snapshot, err = os.ReadFile(snapshotPath)
	require.NoError(t, err)
	assert.Equal(t, edited, string(snapshot), "and rewrites the snapshot beside it")
}

// TestTheEmittedPromptCarriesTheFloorIndexItWrote pins §7.1's "one prompt
// carrying the floor's index": the index in the prompt is the index on disk,
// byte for byte, so a unit grading from the prompt and a --record validating
// against the file cannot be reading two different floors.
func TestTheEmittedPromptCarriesTheFloorIndexItWrote(t *testing.T) {
	dir := writeGroundFixture(t)
	out := groundEmit(t, dir)

	floor, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "floor-ground-round-1.txt"))
	require.NoError(t, err)
	prompt := out["prompt"].(string)
	assert.Contains(t, prompt, string(floor), "the prompt carries the emitted index whole, not a summary of it")
	assert.False(t, strings.Contains(prompt, groundFixtureSpec),
		"the prompt carries the index, not the spec's text: §2.2 measured inlining at 5.7x the index's cost")
}
