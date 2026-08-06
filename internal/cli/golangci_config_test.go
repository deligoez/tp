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
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".golangci.yml"))
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
