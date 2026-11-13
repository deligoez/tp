package dashboard_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// themeTokens is style.css's colour tokens for one theme: every :root
// block's custom properties, the dark theme's :root[data-theme="dark"]
// blocks over them, each var() reference resolved.
func themeTokens(t *testing.T, css string, dark bool) map[string]string {
	t.Helper()
	tok := map[string]string{}
	block := regexp.MustCompile(`(?s)(:root(?:\[data-theme="dark"\])?) \{([^}]*)\}`)
	prop := regexp.MustCompile(`(--[\w-]+):\s*([^;]+);`)
	for _, pass := range []string{":root", `:root[data-theme="dark"]`} {
		if pass != ":root" && !dark {
			break
		}
		for _, m := range block.FindAllStringSubmatch(css, -1) {
			if m[1] != pass {
				continue
			}
			for _, p := range prop.FindAllStringSubmatch(m[2], -1) {
				tok[p[1]] = strings.TrimSpace(p[2])
			}
		}
	}
	ref := regexp.MustCompile(`^var\((--[\w-]+)\)$`)
	for k, v := range tok {
		for range 5 {
			r := ref.FindStringSubmatch(v)
			if r == nil {
				break
			}
			v = tok[r[1]]
		}
		tok[k] = v
	}
	return tok
}

// TestPage_EveryColourPairReadsAgainstItsBackground is 1.17.1's contrast
// check, run on both themes from style.css's own tokens through the
// palette the graph draws with (tpPure.graphPalette): text on a node's fill
// and the page's small texts read at 4.5:1 or more, and every node border,
// edge colour and control outline at 3:1 or more against what it sits on.
func TestPage_EveryColourPairReadsAgainstItsBackground(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	for _, dark := range []bool{false, true} {
		tok, err := json.Marshal(themeTokens(t, css, dark))
		require.NoError(t, err)
		got := runPure(t, `const tok = `+string(tok)+`;
const hex = (v) => [1, 3, 5].map((i) => parseInt(v.slice(i, i + 2), 16));
const pal = P.graphPalette(tok);
const r = (a, b) => Math.round(P.contrast(a, b) * 100) / 100;
const pairs = [];
Object.keys(pal.fill).forEach((k) => pairs.push(["text", "ink on the " + k + " fill", r(pal.ink, pal.fill[k])]));
Object.keys(pal.mark).forEach((k) => pairs.push(["line", k + " border on the canvas", r(pal.mark[k], pal.page)]));
["muted", "crit", "back", "anc", "desc", "tree"].forEach((k) => pairs.push(["line", k + " edges on the canvas", r(pal[k], pal.page)]));
pairs.push(["text", "muted text on the page (tab counts)", r(hex(tok["--muted"]), hex(tok["--bg"]))]);
pairs.push(["text", "muted text on a card (placeholders, legend items off)", r(hex(tok["--muted"]), hex(tok["--surface"]))]);
pairs.push(["line", "the (i) ring on a card", r(hex(tok["--muted"]), hex(tok["--surface"]))]);
out(pairs);`)
		var pairs [][]any
		require.NoError(t, json.Unmarshal([]byte(got), &pairs))
		require.NotEmpty(t, pairs)
		for _, p := range pairs {
			floor := 3.0
			if p[0] == "text" {
				floor = 4.5
			}
			assert.GreaterOrEqual(t, p[2].(float64), floor, "dark=%v: %s", dark, p[1])
		}
	}
	assert.Contains(t, css, ".tab .c { font-family: var(--mono); font-size: var(--text-xs); color: var(--muted);")
	assert.Contains(t, css, ".gcard #gc-q::placeholder { color: var(--muted); opacity: 1; }")
	assert.Contains(t, css, "border: 1px solid var(--muted); display: inline-grid;")
	assert.Contains(t, css, `.gcard .legend button[aria-pressed="false"] { color: var(--muted); text-decoration: line-through; }`)
}

// TestPure_AHueIsNudgedUntilItReadsAgainstThePage: a status hue below the
// contrast asked of it against the page is moved toward white on a dark
// page and toward black on a light one, step by step, until it reaches it;
// a hue that already reads is left as it is.
func TestPure_AHueIsNudgedUntilItReadsAgainstThePage(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const light = [238, 241, 245], dark = [16, 22, 28];
const a = P.nudgeContrast([184, 193, 203], light, 3), b = P.nudgeContrast([58, 69, 82], dark, 3), c = P.nudgeContrast([20, 20, 20], light, 3);
out([P.contrast(a, light) >= 3, a[0] < 184, P.contrast(b, dark) >= 3, b[0] > 58, c, Math.round(P.contrast([0, 0, 0], [255, 255, 255]))]);`)
	assert.JSONEq(t, `[true, true, true, true, [20, 20, 20], 21]`, got)
}
