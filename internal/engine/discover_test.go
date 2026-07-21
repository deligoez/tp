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
