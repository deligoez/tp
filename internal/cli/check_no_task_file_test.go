package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configCheckClass is the class the project-level check below registers.
const configCheckClass = "my-class"

// configCheckMarker is the file the registered check creates when it runs, so
// a test can tell a check that ran from one that never started.
const configCheckMarker = "ran-marker"

// configCheckFixture writes a spec, the standalone regression inputs, and a
// `.tp/config.json` registering one check — and, unlike exclusionFixture, no
// task file unless withTaskFile is set. Without a task file the runner starts
// no check at all (runMechanicalChecks), which is the condition under test.
func configCheckFixture(t *testing.T, withTaskFile bool) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## One\ncontent\n"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
		[]byte(`{"workflow":{"checks":[{"class":"`+configCheckClass+`","cmd":"touch `+configCheckMarker+`; exit 0"}]}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "baseline.md"), []byte("# Spec\n\n## One\nolder content\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "regression-findings.ndjson"),
		[]byte(`{"evidence":"read the cited section","severity":"low","category":"consistency","location":"L1","finding":"f","suggestion":"fix","resolved":{"status":"fixed","evidence":"e"}}`+"\n"), 0o600))
	if withTaskFile {
		_, stderr, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code, "init failed: %s", stderr)
	}
	return dir
}

// suppressesConfigClass reports whether text tells the reviewer not to report
// configCheckClass.
func suppressesConfigClass(t *testing.T, text string) bool {
	t.Helper()
	got, ok := exclusionClasses(t, text)
	return ok && slices.Contains(got, configCheckClass)
}

func markerExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, configCheckMarker))
	return err == nil
}

// TestReviewEmission_ACheckWithNoTaskFileSuppressesNothing closes the no-task-
// file half of the not-run hole. A check registered in `.tp/config.json` with
// no task file resolving never starts — the runner returns no entry for it at
// all, so there is no `ran: false` row for the exclusion to act on — yet every
// prompt still told the reviewers not to report its class. A class is
// suppressed only by a check that ran in this emission.
//
// The marker file is what proves the check did not run, so the absent
// sentence is attributable to that and not to a check that ran and passed.
// Both emission sites are driven: the panel and the standalone regression
// perspective each append the sentence on their own path.
func TestReviewEmission_ACheckWithNoTaskFileSuppressesNothing(t *testing.T) {
	t.Parallel()

	t.Run("panel", func(t *testing.T) {
		t.Parallel()
		dir := configCheckFixture(t, false)
		texts, _ := panelPrompts(t, dir)
		require.False(t, markerExists(dir), "no task file resolves, so the check must not have run")
		for i, text := range texts {
			assert.False(t, suppressesConfigClass(t, text),
				"panel prompt %d: a check that never ran must not suppress %s", i, configCheckClass)
		}
	})

	t.Run("regression", func(t *testing.T) {
		t.Parallel()
		dir := configCheckFixture(t, false)
		text, _ := regressionPrompt(t, dir)
		require.False(t, markerExists(dir), "no task file resolves, so the check must not have run")
		assert.False(t, suppressesConfigClass(t, text),
			"regression prompt: a check that never ran must not suppress %s", configCheckClass)
	})
}

// TestReviewEmission_ACheckThatRanFromConfigStillSuppresses is the control:
// the same `.tp/config.json` check with a task file resolving runs, and a check
// that ran keeps suppressing its class at both sites. Without it, a fix that
// dropped the sentence wholesale would pass the test above.
func TestReviewEmission_ACheckThatRanFromConfigStillSuppresses(t *testing.T) {
	t.Parallel()

	t.Run("panel", func(t *testing.T) {
		t.Parallel()
		dir := configCheckFixture(t, true)
		texts, _ := panelPrompts(t, dir)
		require.True(t, markerExists(dir), "with a task file the check runs")
		for i, text := range texts {
			assert.True(t, suppressesConfigClass(t, text),
				"panel prompt %d: a check that ran suppresses %s", i, configCheckClass)
		}
	})

	t.Run("regression", func(t *testing.T) {
		t.Parallel()
		dir := configCheckFixture(t, true)
		text, _ := regressionPrompt(t, dir)
		require.True(t, markerExists(dir), "with a task file the check runs")
		assert.True(t, suppressesConfigClass(t, text),
			"regression prompt: a check that ran suppresses %s", configCheckClass)
	})
}
