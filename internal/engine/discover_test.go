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
