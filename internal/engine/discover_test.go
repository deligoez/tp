package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverTaskFile(t *testing.T) {
	tests := []struct {
		name      string
		files     []string // files to create in temp dir
		explicit  string   // explicit --file flag value (relative to temp dir if non-empty and not absolute)
		wantFile  string   // expected filename (basename only)
		wantError string   // substring of expected error
	}{
		{
			name:     "single task file found",
			files:    []string{"project.tasks.json"},
			wantFile: "project.tasks.json",
		},
		{
			name:      "multiple task files",
			files:     []string{"a.tasks.json", "b.tasks.json"},
			wantError: "multiple task files",
		},
		{
			name:      "no task file",
			files:     []string{"readme.md", "main.go"},
			wantError: "no task file",
		},
		{
			name:  "explicit file flag overrides discovery",
			files: []string{"other.tasks.json"},
			// explicit is set per-test below
			wantFile: "explicit.tasks.json",
		},
		{
			name:      "explicit file that does not exist",
			files:     []string{},
			explicit:  "/nonexistent/explicit.tasks.json",
			wantError: "task file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			for _, f := range tt.files {
				err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0o600)
				require.NoError(t, err)
			}

			explicit := tt.explicit
			// For the "explicit file flag" test, create the file and set the path
			if tt.name == "explicit file flag overrides discovery" {
				explicitPath := filepath.Join(dir, "explicit.tasks.json")
				err := os.WriteFile(explicitPath, []byte("{}"), 0o600)
				require.NoError(t, err)
				explicit = explicitPath
			}

			got, err := DiscoverTaskFile(dir, explicit)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantFile, filepath.Base(got))
			}
		})
	}
}

func TestResolveSpecPath(t *testing.T) {
	tests := []struct {
		name       string
		spec       string
		createSpec bool
		wantExists bool
	}{
		{
			name:       "spec file exists",
			spec:       "spec.md",
			createSpec: true,
			wantExists: true,
		},
		{
			name:       "spec file missing",
			spec:       "missing-spec.md",
			createSpec: false,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			taskFilePath := filepath.Join(dir, "project.tasks.json")

			if tt.createSpec {
				err := os.WriteFile(filepath.Join(dir, tt.spec), []byte("# Spec"), 0o600)
				require.NoError(t, err)
			}

			resolvedPath, exists := ResolveSpecPath(taskFilePath, tt.spec)

			assert.Equal(t, tt.wantExists, exists)
			assert.Equal(t, filepath.Join(dir, tt.spec), resolvedPath)
		})
	}
}

// TestDiscoverTaskFile_PrecedenceAndTPActiveIgnored confirms the v0.25.0
// discovery precedence (--file > TP_FILE > .tp/local.json active > auto-detect)
// and that the removed .tp-active marker is never consulted (§11.1, §11.3).
func TestDiscoverTaskFile_PrecedenceAndTPActiveIgnored(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	write := func(name string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("{}"), 0o600))
	}
	write("pointed.tasks.json")
	write("env.tasks.json")
	write("explicit.tasks.json")

	// A .tp-active pointing at a real file must never be consulted.
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp-active"), []byte("env.tasks.json\n"), 0o600))

	tpDir := filepath.Join(root, ".tp")
	require.NoError(t, os.Mkdir(tpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "local.json"), []byte(`{"active":"pointed.tasks.json"}`), 0o600))
	t.Chdir(root)

	t.Run("local.json active wins over auto-detect; .tp-active ignored", func(t *testing.T) {
		got, err := DiscoverTaskFile(root, "")
		require.NoError(t, err)
		assert.Equal(t, "pointed.tasks.json", filepath.Base(got))
	})

	t.Run("TP_FILE wins over local.json", func(t *testing.T) {
		t.Setenv("TP_FILE", filepath.Join(root, "env.tasks.json"))
		got, err := DiscoverTaskFile(root, "")
		require.NoError(t, err)
		assert.Equal(t, "env.tasks.json", filepath.Base(got))
	})

	t.Run("--file wins over TP_FILE", func(t *testing.T) {
		t.Setenv("TP_FILE", filepath.Join(root, "env.tasks.json"))
		got, err := DiscoverTaskFile(root, filepath.Join(root, "explicit.tasks.json"))
		require.NoError(t, err)
		assert.Equal(t, "explicit.tasks.json", filepath.Base(got))
	})

	t.Run(".tp-active is not consulted when local.json is absent", func(t *testing.T) {
		require.NoError(t, os.Remove(filepath.Join(tpDir, "local.json")))
		// Multiple .tasks.json files -> auto-detect is ambiguous; if .tp-active
		// were still read it would resolve env.tasks.json instead of erroring.
		_, err := DiscoverTaskFile(root, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple task files")
	})
}

// TestDiscoverTaskFileVia_ReportsOnlyThePointer pins the flag write commands
// key their notice on: true when .tp/local.json's active pointer chose the
// file, false for --file and auto-detect, which resolve the same paths.
func TestDiscoverTaskFileVia_ReportsOnlyThePointer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	spec := filepath.Join(root, "spec")
	require.NoError(t, os.Mkdir(spec, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(spec, "a.tasks.json"), []byte("{}"), 0o600))

	got, via, err := DiscoverTaskFileVia(root, "")
	require.NoError(t, err)
	assert.Equal(t, "a.tasks.json", filepath.Base(got))
	assert.False(t, via, "auto-detect found the file, not the pointer")

	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"), []byte(`{"active":"spec/a.tasks.json"}`), 0o600))
	got, via, err = DiscoverTaskFileVia(root, "")
	require.NoError(t, err)
	assert.Equal(t, "a.tasks.json", filepath.Base(got))
	assert.True(t, via, "the pointer chose the file")

	_, via, err = DiscoverTaskFileVia(root, filepath.Join(spec, "a.tasks.json"))
	require.NoError(t, err)
	assert.False(t, via, "--file outranks the pointer")
}

// TestOtherTaskFiles_SeesSiblingsRootAndSubdirs covers the three places the
// scan looks — the pointer file's own directory, the directory the command
// runs from, and that directory's non-hidden subdirectories — and that it
// never counts the file itself or a hidden directory.
func TestOtherTaskFiles_SeesSiblingsRootAndSubdirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	for _, d := range []string{"spec", filepath.Join("spec", "backlog"), "other", ".hidden"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	nested := filepath.Join(root, "spec", "backlog", "a.tasks.json")
	for _, f := range []string{nested, filepath.Join(root, "spec", "backlog", "b.tasks.json"), filepath.Join(root, "root.tasks.json"), filepath.Join(root, "other", "c.tasks.json"), filepath.Join(root, ".hidden", "h.tasks.json")} {
		require.NoError(t, os.WriteFile(f, []byte("{}"), 0o600))
	}

	names := make([]string, 0)
	for _, p := range OtherTaskFiles(root, nested) {
		names = append(names, filepath.Base(p))
	}
	assert.ElementsMatch(t, []string{"b.tasks.json", "root.tasks.json", "c.tasks.json"}, names)
}

// TestOtherTaskFiles_ARelativePathIsNotItsOwnSibling: the file named by path is
// never among the others, however path is spelled. Every other file is
// compared by its absolute path, so path must be made absolute too; kept
// relative, the file came back as its own sibling.
func TestOtherTaskFiles_ARelativePathIsNotItsOwnSibling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	self := filepath.Join(root, "a.tasks.json")
	require.NoError(t, os.WriteFile(self, []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.tasks.json"), []byte("{}"), 0o600))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	rel, err := filepath.Rel(cwd, self)
	require.NoError(t, err)
	require.False(t, filepath.IsAbs(rel), "the fixture must hand in a relative path")

	names := make([]string, 0)
	for _, p := range OtherTaskFiles(root, rel) {
		names = append(names, filepath.Base(p))
	}
	assert.Equal(t, []string{"b.tasks.json"}, names)
}
