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
//
// It tracks plugin.json rather than lagging it: because the manifest's version
// IS the minimum, bumping the plugin raises the bar for the binary too, and the
// release that carries the bump ships that binary.
//
// It is READ from the manifest rather than restated beside it. Restating it cost
// a gate run during v0.37.0's review: correcting the workflow-field precedence in
// REFERENCE.md — plugin content — reddened
// TestPluginVersionIsBumpedWhenPluginContentChanges, and bumping the manifest to
// satisfy that reddened four subtests of TestSessionStartHookFailsWhenTpTooOld
// against a literal that had to be edited to match. A guard whose expected value
// is a copy of the thing it guards fails twice for one change, and the second
// failure teaches nothing.
func pluginMinVersion(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), pluginManifestPath)) //nolint:gosec // a fixed path inside the repo under test
	require.NoError(t, err)
	var m pluginManifest
	require.NoError(t, json.Unmarshal(raw, &m))
	require.NotEmpty(t, m.Version, "plugin.json must declare a version; it is the preflight minimum")
	return m.Version
}

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
	assert.GreaterOrEqual(t, comparePluginVersions(t, manifest.Version, pluginMinVersion(t)), 0,
		"plugin.json's version is the preflight minimum, so it may not fall below itself")

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

// TestPluginShipsNoBinary guards the other half of §6.1: the Go binary is not
// shipped inside the plugin. Installation stays Homebrew and `go install`, and
// the SessionStart preflight is what covers a missing tp — so a committed
// binary is not merely redundant, it is the failure mode the preflight exists
// to replace. The scan reads leading bytes rather than trusting file names,
// because the accident this catches is a stray `go build -o` output, which
// carries no telling extension.
func TestPluginShipsNoBinary(t *testing.T) {
	root := repoRoot(t)
	skipDirs := map[string]bool{".git": true, "dist": true, "node_modules": true}

	var found []string
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skipDirs[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		head, readErr := readFileHead(path, 4)
		if readErr != nil {
			return readErr
		}
		for _, magic := range executableMagics {
			if bytes.HasPrefix(head, magic) {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				found = append(found, rel)
				break
			}
		}
		return nil
	}))

	assert.Empty(t, found, "the plugin ships no binary: install tp with Homebrew or go install")

	// The locally built binary lands at the repo root under the plugin's own
	// name, so the ignore entry is what keeps the scan above from ever having
	// something to find.
	assert.Contains(t, strings.Split(readRepoDoc(t, ".gitignore"), "\n"), "/tp",
		".gitignore must keep a locally built tp out of the plugin")
}

func readFileHead(path string, n int) ([]byte, error) {
	file, err := os.Open(path) //nolint:gosec // walking the repo the test runs in
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	head := make([]byte, n)
	read, err := file.Read(head)
	if err != nil && read == 0 {
		return nil, nil // empty file, or one that cannot be read past its header
	}
	return head[:read], nil
}

// TestPluginValidatesWithClaudeCLI runs the check test 24 names verbatim. It is
// skipped when the claude CLI is absent, so the two tests above stay the
// durable guard and this one is the confirmation on a machine that has the
// tool.
func TestPluginValidatesWithClaudeCLI(t *testing.T) {
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude CLI not on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, claude, "plugin", "validate", repoRoot(t), "--strict")
	out, runErr := cmd.CombinedOutput()
	require.NoError(t, runErr, "claude plugin validate --strict failed:\n%s", out)
}

// TestPluginVersionIsNotBehindTheLatestTag catches the failure mode that makes
// a published plugin update invisible.
//
// Claude Code resolves a plugin's version from plugin.json's `version` field
// first, and a set field PINS it: users are offered an update only when that
// field changes, no matter how many commits or tags ship. The documented remedy
// is to omit the field and let git tags drive versioning — which tp cannot do,
// because `hooks/session-start.sh` reads that same field as the minimum tp
// version the plugin will run against (§6.1). The field is load-bearing twice,
// so the bump is mandatory rather than optional, and this is the check that
// makes it mechanical instead of a line in a checklist.
//
// The comparison is >= rather than ==, which is what makes it usable inside the
// release itself: bumping plugin.json and then tagging is the correct order, so
// the manifest legitimately runs ahead of the newest tag for the length of that
// window. It falls behind only when a release was tagged without the bump.
func TestPluginVersionIsNotBehindTheLatestTag(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, git, "-C", repoRoot(t), "describe", "--tags", "--abbrev=0")
	out, describeErr := cmd.Output()
	if describeErr != nil {
		t.Skip("no reachable tag to compare against (shallow clone or fresh repo)")
	}
	tag := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "v"))

	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".claude-plugin", "plugin.json"))
	require.NoError(t, err)
	var manifest struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.NotEmpty(t, manifest.Version,
		"plugin.json must carry a version: the SessionStart preflight reads it as the minimum tp version")

	assert.GreaterOrEqual(t, comparePluginVersions(t, manifest.Version, tag), 0,
		"plugin.json says %s but the newest tag is v%s — a release was tagged without bumping the "+
			"manifest, so Claude Code still reports the old version and no user is offered the update",
		manifest.Version, tag)
}

// TestPluginVersionIsBumpedWhenPluginContentChanges closes the half of the
// version trap the test above cannot see.
//
// The marketplace entry's source is `./`, so the plugin IS this repository at
// whatever commit the user's marketplace clone sits on — and a marketplace
// source tracks the default branch, not a tag. Plugin content therefore follows
// main immediately, while `claude plugin update` decides purely on plugin.json's
// version string. The two move independently, and that gap is the trap: push a
// change to skills/, hooks/, agents/ or .claude-plugin/ without bumping the
// version and an existing user is told "already at the latest version" forever,
// while a fresh installer silently receives different bytes under the same
// version number.
//
// So the rule is content-addressed rather than calendar-based: if anything the
// plugin carries differs from the newest tag, the manifest must already be
// AHEAD of that tag. Between releases nothing differs and the test is quiet;
// the moment a plugin-carried file changes it demands the bump that makes the
// change reachable.
func TestPluginVersionIsBumpedWhenPluginContentChanges(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	root := repoRoot(t)

	run := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, runErr := exec.CommandContext(ctx, git, append([]string{"-C", root}, args...)...).Output()
		return strings.TrimSpace(string(out)), runErr
	}

	tag, err := run("describe", "--tags", "--abbrev=0")
	if err != nil {
		t.Skip("no reachable tag to compare against")
	}

	// Everything the plugin ships. The Go binary is deliberately absent: it is
	// installed separately, so a code change alone does not require a bump.
	carried := []string{".claude-plugin", "skills", "hooks", "agents"}
	changed, err := run(append([]string{"diff", "--name-only", tag + "..HEAD", "--"}, carried...)...)
	if err != nil {
		t.Skip("cannot diff against " + tag)
	}
	if changed == "" {
		return // nothing the plugin carries has moved since the tag
	}

	raw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	require.NoError(t, err)
	var manifest struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))

	assert.Positive(t, comparePluginVersions(t, manifest.Version, strings.TrimPrefix(tag, "v")),
		"these plugin files changed since %s:\n%s\n\nbut plugin.json still says %s. Claude Code "+
			"decides updates on that string alone, so every existing user would be told they are "+
			"already current while receiving none of this. Bump plugin.json before tagging.",
		tag, changed, manifest.Version)
}

// TestMarketplaceEntryDoesNotDoubleDeclareComponents guards the one manifest
// error `claude plugin validate` cannot see.
//
// With `strict: false` the runtime discovers components by convention from the
// directories beside plugin.json — skills/, hooks/, agents/. A marketplace
// entry that ALSO lists any of them declares the same component twice, and the
// runtime refuses to load the plugin at install time with "conflicting
// manifests: both plugin.json and marketplace entry specify components".
//
// v0.35.0 shipped exactly that: the entry carried `"skills": ["./skills/tp"]`
// beside `"strict": false`. Validation passed, the install reported success,
// and the plugin then failed to load — so none of the hooks or agents this
// version exists to ship were ever active. The lesson is the one this repo
// keeps relearning: `claude plugin validate` checks the manifest's shape, not
// whether the plugin loads, so a green validate is not evidence of a working
// plugin. `claude plugin install` is the experiment that can fail.
func TestMarketplaceEntryDoesNotDoubleDeclareComponents(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".claude-plugin", "marketplace.json"))
	require.NoError(t, err)

	var manifest struct {
		Plugins []map[string]any `json:"plugins"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.NotEmpty(t, manifest.Plugins, "the marketplace lists at least one plugin")

	// The keys the runtime treats as component declarations.
	component := []string{"skills", "hooks", "agents", "commands", "mcpServers"}

	for _, entry := range manifest.Plugins {
		strict, _ := entry["strict"].(bool)
		if strict {
			// Under strict the entry is the only source, so listing is required
			// rather than duplicated — but then it must list everything shipped.
			continue
		}
		for _, key := range component {
			assert.NotContains(t, entry, key,
				"marketplace entry %q declares %q while strict is false, so the runtime "+
					"sees it declared twice (convention discovery + this entry) and refuses "+
					"to load the plugin; drop the key and let the directory be discovered",
				entry["name"], key)
		}
	}
}
