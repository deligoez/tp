package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestGolangciConfigEnablesGofmt guards the v0.31.1 gate fix: the repo
// .golangci.yml must keep a formatters section that enables gofmt, so
// `golangci-lint run` (the project quality gate) reports gofmt-dirty files and
// exits non-zero. Silently dropping that section re-opens the exact blind spot
// the release closed — this test fails instead of passing unnoticed.
func TestGolangciConfigEnablesGofmt(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(golangciRepoRoot(t), ".golangci.yml"))
	require.NoError(t, err, ".golangci.yml must exist at the repo root")

	var cfg struct {
		Formatters struct {
			Enable []string `yaml:"enable"`
		} `yaml:"formatters"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Contains(t, cfg.Formatters.Enable, "gofmt",
		"formatters.enable must include gofmt so `golangci-lint run` catches formatting")
}

// golangciRepoRoot walks up from the test's working directory to the module
// root (the directory holding go.mod).
func golangciRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "reached the filesystem root without finding go.mod")
		dir = parent
	}
}
