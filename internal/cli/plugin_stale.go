package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// releaseVersion matches a released version and nothing else: a dev build,
// a pseudo-version or a +dirty build reports a number that is not the
// release it was built near, so it never proves a plugin stale.
var releaseVersion = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// pluginStaleNotice names a plugin older than the binary calling it, with the
// remedy. A Claude Code session keeps the skill text it loaded at start, so a
// session started before a plugin update works from text that predates the
// binary. The plugin's SessionStart hook runs tp resume with
// CLAUDE_PLUGIN_ROOT set; an agent's own shell does not, so the notice is
// seen at session start and at compaction. Every case it cannot prove
// returns "", and nothing here can fail tp resume.
func pluginStaleNotice(binary, pluginRoot string) string {
	if pluginRoot == "" {
		return ""
	}
	bin, ok := parseRelease(binary)
	if !ok {
		return ""
	}
	name, version, ok := readPluginManifest(pluginRoot)
	if !ok || name != "tp" {
		return ""
	}
	plugin, ok := parseRelease(version)
	if !ok || !releaseBelow(plugin, bin) {
		return ""
	}
	return fmt.Sprintf("the tp plugin this session loaded is %s, older than tp %s: its skill text predates this binary; run `claude plugin update tp@tp`, then start a new session", version, binary)
}

// parseRelease returns a release version's three numbers.
func parseRelease(v string) ([3]int, bool) {
	var out [3]int
	m := releaseVersion.FindStringSubmatch(v)
	if m == nil {
		return out, false
	}
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// releaseBelow reports whether a is strictly below b.
func releaseBelow(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// readPluginManifest returns the plugin's name and version.
func readPluginManifest(pluginRoot string) (name, version string, ok bool) {
	data, err := os.ReadFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"))
	if err != nil {
		return "", "", false
	}
	var m struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &m) != nil {
		return "", "", false
	}
	return m.Name, m.Version, true
}
