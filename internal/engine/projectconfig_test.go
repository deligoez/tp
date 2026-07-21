package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func evalLink(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestDiscoverTPDir_FindsFromSubdir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	sub := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	got := DiscoverTPDir(sub)
	require.NotEmpty(t, got)
	assert.Equal(t, evalLink(t, filepath.Join(root, ".tp")), evalLink(t, got))
}

func TestDiscoverTPDir_TestsAnchorItself(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	got := DiscoverTPDir(root)
	assert.Equal(t, evalLink(t, filepath.Join(root, ".tp")), evalLink(t, got))
}

func TestDiscoverTPDir_HaltsAtGitBoundary(t *testing.T) {
	// .tp/ sits ABOVE the git boundary; the walk must stop at the boundary
	// and never read it.
	outer := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outer, ".tp"), 0o755))
	repo := filepath.Join(outer, "repo")
	require.NoError(t, os.Mkdir(repo, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(repo, ".git"), 0o755))

	assert.Empty(t, DiscoverTPDir(repo), "must not read a .tp/ above the git boundary")
}

func TestDiscoverTPDir_GitAsFileIsBoundary(t *testing.T) {
	outer := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outer, ".tp"), 0o755))
	repo := filepath.Join(outer, "wt")
	require.NoError(t, os.Mkdir(repo, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /x\n"), 0o600))

	assert.Empty(t, DiscoverTPDir(repo), "a .git file (worktree) is a boundary too")
}

func TestDiscoverTPDir_NoneReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	assert.Empty(t, DiscoverTPDir(root))
}

func TestProjectRoot_DirContainingTP(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	sub := filepath.Join(root, "spec")
	require.NoError(t, os.Mkdir(sub, 0o755))
	assert.Equal(t, evalLink(t, root), evalLink(t, ProjectRoot(sub)))
}

func TestProjectRoot_FallsBackToGitBoundary(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	sub := filepath.Join(root, "a")
	require.NoError(t, os.Mkdir(sub, 0o755))
	assert.Equal(t, evalLink(t, root), evalLink(t, ProjectRoot(sub)),
		"with no .tp/, the git boundary is the project root")
}

func TestProjectRoot_FallsBackToStartOutsideGitRepo(t *testing.T) {
	// A dir with no .tp/ and no .git ancestor resolves to itself.
	root := t.TempDir()
	got := ProjectRoot(root)
	if FindGitBoundary(root) == "" {
		assert.Equal(t, evalLink(t, root), evalLink(t, got))
	}
}

func TestEnsureTPGitignore_CreatesWhenAbsent(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, EnsureTPGitignore(tp))
	data, err := os.ReadFile(filepath.Join(tp, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "local.json\n", string(data))
}

func TestEnsureTPGitignore_IdempotentAppend(t *testing.T) {
	tp := t.TempDir()
	// Pre-existing .gitignore with unrelated content, no trailing newline.
	require.NoError(t, os.WriteFile(filepath.Join(tp, ".gitignore"), []byte("other"), 0o600))
	require.NoError(t, EnsureTPGitignore(tp))
	require.NoError(t, EnsureTPGitignore(tp)) // second call must not duplicate
	data, err := os.ReadFile(filepath.Join(tp, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "other\nlocal.json\n", string(data))
	assert.Equal(t, 1, strings.Count(string(data), "local.json"))
}

func TestLoadProjectConfig_ParsesWorkflowWithPresence(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":0,"gate_timeout_seconds":900}}`), 0o600))
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	assert.Empty(t, warns)
	require.NotNil(t, pc.Workflow.ReviewMaxRounds)
	assert.Equal(t, 0, *pc.Workflow.ReviewMaxRounds) // explicit zero, present
	require.NotNil(t, pc.Workflow.GateTimeoutSeconds)
	assert.Equal(t, 900, *pc.Workflow.GateTimeoutSeconds)
	assert.Nil(t, pc.Workflow.ReviewCleanRounds) // absent key stays nil
}

func TestLoadProjectConfig_MissingFileIsEmpty(t *testing.T) {
	pc, warns, err := LoadProjectConfig(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.True(t, pc.Workflow.IsEmpty())
}

func TestLoadLocalConfig_ParsesActiveAndDefaults(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "local.json"),
		[]byte(`{"active":"a/a.tasks.json","defaults":{"compact":true}}`), 0o600))
	lc, warns, err := LoadLocalConfig(tp)
	require.NoError(t, err)
	assert.Empty(t, warns)
	require.NotNil(t, lc.Active)
	assert.Equal(t, "a/a.tasks.json", *lc.Active)
	assert.True(t, lc.Defaults["compact"])

	empty, _, err := LoadLocalConfig(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, empty.Active)
}

func TestLoadProjectConfig_UnknownKeysWarn(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":8,"bogus":1},"extra":true}`), 0o600))
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	require.NotNil(t, pc.Workflow.ReviewMaxRounds) // known key still parsed
	assert.Contains(t, warns, "unknown top-level key: extra")
	assert.Contains(t, warns, "unknown workflow key: bogus")
}

func TestLoadProjectConfig_TypeMismatchFallsBack(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":"eight","checks":"nope"}}`), 0o600))
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	assert.Nil(t, pc.Workflow.ReviewMaxRounds, "wrong-typed field is left unset")
	assert.Nil(t, pc.Workflow.Checks)
	assert.Contains(t, warns, "workflow.review_max_rounds: expected a number, ignored")
	assert.Contains(t, warns, "workflow.checks: expected an array, ignored")
}

func TestLoadLocalConfig_TypeMismatchFallsBack(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "local.json"),
		[]byte(`{"active":123,"defaults":{"compact":"yes"}}`), 0o600))
	lc, warns, err := LoadLocalConfig(tp)
	require.NoError(t, err)
	assert.Nil(t, lc.Active, "non-string active is treated as unset")
	assert.NotContains(t, lc.Defaults, "compact", "non-boolean default is skipped")
	assert.Contains(t, warns, "active: expected a string, ignored")
	assert.Contains(t, warns, "defaults.compact: expected a boolean, ignored")
}

func TestLoadLocalConfig_UnknownDefaultsKeyWarns(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "local.json"),
		[]byte(`{"defaults":{"compact":true,"bogus":false}}`), 0o600))
	lc, warns, err := LoadLocalConfig(tp)
	require.NoError(t, err)
	assert.True(t, lc.Defaults["compact"], "a known flag is kept")
	assert.NotContains(t, lc.Defaults, "bogus", "an unknown flag is ignored, not accepted")
	assert.Contains(t, warns, "unknown defaults key: bogus")
}

func TestLoadProjectConfig_OutOfRangeFallsBack(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"gate_timeout_seconds":5,"review_clean_rounds":99,"review_max_rounds":0}}`), 0o600))
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	assert.Nil(t, pc.Workflow.GateTimeoutSeconds, "out-of-range gate_timeout_seconds is unset (falls back to 600)")
	assert.Nil(t, pc.Workflow.ReviewCleanRounds, "out-of-range clean_rounds is unset (falls back to 2)")
	require.NotNil(t, pc.Workflow.ReviewMaxRounds, "in-range 0 (no cap) is kept")
	assert.Equal(t, 0, *pc.Workflow.ReviewMaxRounds)
	assert.Contains(t, warns, "workflow.gate_timeout_seconds: 5 is out of range [30,3600], using the built-in default")
	assert.Contains(t, warns, "workflow.review_clean_rounds: 99 is out of range [1,10], using the built-in default")
}

func TestLoadProjectConfig_MalformedAborts(t *testing.T) {
	for name, body := range map[string]string{
		"empty":            "",
		"whitespace":       "   \n",
		"top-level array":  "[]",
		"top-level number": "42",
		"invalid json":     "{not json",
	} {
		t.Run(name, func(t *testing.T) {
			tp := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"), []byte(body), 0o600))
			_, _, err := LoadProjectConfig(tp)
			var mce *MalformedConfigError
			require.ErrorAs(t, err, &mce, "a malformed config must be a MalformedConfigError")
			assert.Contains(t, mce.Hint(), "repair or delete")
		})
	}
}

func TestLoadProjectConfig_EmptyObjectIsValid(t *testing.T) {
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"), []byte("{}"), 0o600))
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.True(t, pc.Workflow.IsEmpty())
}
