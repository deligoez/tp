package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginManifestPath is the plugin's identity file. v0.35.0 §6.1 fills the
// plugin out at the root that already carried marketplace.json, so this file
// and marketplace.json are siblings.
const pluginManifestPath = ".claude-plugin/plugin.json"

// pluginMinVersion is the version the SessionStart preflight (§6.1) compares
// `tp --version` against. The spec makes plugin.json's own version the minimum,
// so the manifest may never declare less than the release that introduced the
// preflight — a lower value would let an older tp satisfy a check written for
// this one.
const pluginMinVersion = "0.35.0"

// pluginManifest is the subset of the manifest this guard asserts: identity,
// nothing else. Components (skills/, hooks/, agents/) are discovered by
// convention at the plugin root, so declaring them here would be a second copy
// of the layout that could drift from the directories themselves.
type pluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"author"`
	Homepage   string   `json:"homepage"`
	Repository string   `json:"repository"`
	License    string   `json:"license"`
	Keywords   []string `json:"keywords"`
}

// TestPluginManifestDeclaresIdentity guards v0.35.0 §6.1: .claude-plugin/
// plugin.json must exist beside marketplace.json and carry the plugin's
// identity. The manifest that shipped before this version declared a "1.0.0"
// version and an inline skills array, which `claude plugin validate` rejected
// outright — so an unparseable or identity-less manifest is a regression this
// test catches without needing the claude CLI installed.
func TestPluginManifestDeclaresIdentity(t *testing.T) {
	raw := readRepoDoc(t, pluginManifestPath)

	var manifest pluginManifest
	require.NoError(t, json.Unmarshal([]byte(raw), &manifest), "%s must be valid JSON", pluginManifestPath)

	assert.Equal(t, "tp", manifest.Name)
	assert.NotEmpty(t, manifest.Description, "the manifest must describe the plugin")
	assert.NotEmpty(t, manifest.Author.Name, "the manifest must name an author")
	assert.Equal(t, "https://github.com/deligoez/tp", manifest.Homepage)
	assert.Equal(t, "https://github.com/deligoez/tp", manifest.Repository)
	assert.Equal(t, "MIT", manifest.License)
	assert.NotEmpty(t, manifest.Keywords, "the marketplace lists the plugin by its keywords")

	require.NotEmpty(t, manifest.Version, "the preflight compares tp --version against this field")
	assert.GreaterOrEqual(t, comparePluginVersions(t, manifest.Version, pluginMinVersion), 0,
		"plugin.json's version is the preflight minimum, so it may not fall below %s", pluginMinVersion)

	// The skills the plugin ships live at skills/tp and are discovered by
	// convention. The rejected pre-v0.35.0 manifest instead inlined them as
	// objects with path/triggers fields; assert that shape is gone.
	var shape map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &shape))
	assert.NotContains(t, shape, "skills",
		"skills/ is discovered at the plugin root; an inline skills block fails claude plugin validate")
	assert.FileExists(t, filepath.Join(repoRoot(t), "skills", "tp", "SKILL.md"),
		"the plugin root must still carry the skill the manifest's identity covers")
}

// comparePluginVersions reports -1, 0 or 1 for a dotted numeric version pair.
// The manifest holds a plain three-part version, so a full semver parser (and a
// new dependency) would be more machinery than the one comparison needs.
func comparePluginVersions(t *testing.T, a, b string) int {
	t.Helper()
	partsA, partsB := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(partsA) || i < len(partsB); i++ {
		numA, numB := pluginVersionPart(t, partsA, i), pluginVersionPart(t, partsB, i)
		if numA != numB {
			if numA < numB {
				return -1
			}
			return 1
		}
	}
	return 0
}

func pluginVersionPart(t *testing.T, parts []string, i int) int {
	t.Helper()
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[i])
	require.NoError(t, err, "version component %q must be numeric", parts[i])
	return n
}

// executableMagics are the leading bytes of the binary formats a cross-platform
// release produces. A marketplace is a git repository (§6.1), so none of them
// may appear in a tracked file.
var executableMagics = [][]byte{
	{0x7f, 'E', 'L', 'F'},    // ELF
	{0xcf, 0xfa, 0xed, 0xfe}, // Mach-O 64-bit, little endian
	{0xce, 0xfa, 0xed, 0xfe}, // Mach-O 32-bit, little endian
	{0xfe, 0xed, 0xfa, 0xcf}, // Mach-O 64-bit, big endian
	{0xfe, 0xed, 0xfa, 0xce}, // Mach-O 32-bit, big endian
	{0xca, 0xfe, 0xba, 0xbe}, // Mach-O universal ("fat") binary
	{'M', 'Z', 0x90, 0x00},   // PE/COFF, via its DOS header
}

