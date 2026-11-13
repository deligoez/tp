package dashboard_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The theme blocks style.css can hold over its :root: the dark theme as
// the header's control sets it, and the dark theme the system's setting
// asks for under Auto.
const (
	darkByControl = `:root[data-theme="dark"]`
	darkBySystem  = `:root:not([data-theme="light"])`
)

// themeTokens is style.css's colour tokens for one theme: every :root
// block's custom properties, the blocks named over (darkByControl,
// darkBySystem or none for the light theme) over them, each var()
// reference resolved.
func themeTokens(t *testing.T, css, over string) map[string]string {
	t.Helper()
	tok := map[string]string{}
	block := regexp.MustCompile(`(?s)(:root(?:\[data-theme="dark"\]|:not\(\[data-theme="light"\]\))?) \{([^}]*)\}`)
	prop := regexp.MustCompile(`(--[\w-]+):\s*([^;]+);`)
	for _, pass := range []string{":root", over} {
		if pass == "" {
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

// TestPage_BothDarkThemesHoldTheSameTokens: the dark theme the control
// sets and the one the system's setting brings under Auto are written in
// two places of style.css; they hold the same tokens, so the contrast
// check of one covers the other.
func TestPage_BothDarkThemesHoldTheSameTokens(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	light := themeTokens(t, css, "")
	byControl := themeTokens(t, css, darkByControl)
	bySystem := themeTokens(t, css, darkBySystem)
	assert.Equal(t, byControl, bySystem)
	assert.NotEqual(t, light["--sunk"], bySystem["--sunk"], "the system's dark block is read")
}

// TestPage_EveryColourPairReadsAgainstItsBackground is 1.17.1's contrast
// check, run on the light theme and both dark ones from style.css's own
// tokens through the palette the graph draws with (tpPure.graphPalette):
// text on a node's fill and the page's small texts read at 4.5:1 or more,
// and every node border, edge colour and control outline at 3:1 or more
// against what it sits on.
func TestPage_EveryColourPairReadsAgainstItsBackground(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	for _, over := range []string{"", darkByControl, darkBySystem} {
		tok, err := json.Marshal(themeTokens(t, css, over))
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
			assert.GreaterOrEqual(t, p[2].(float64), floor, "theme %q: %s", over, p[1])
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

// TestPage_TheStatusHuesReadBeforeTheNudgeAndStayApart: the nudge lifts
// every hue to 3:1, so the contrast check alone would pass any token. Each
// status hue reads at 2:1 or more against the canvas before it, and after
// it no two marks come closer than a colour difference (CIE76 ΔE) of 12,
// so the graph's marks stay told apart on both themes.
func TestPage_TheStatusHuesReadBeforeTheNudgeAndStayApart(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	for _, over := range []string{"", darkByControl} {
		tok, err := json.Marshal(themeTokens(t, css, over))
		require.NoError(t, err)
		got := runPure(t, `const tok = `+string(tok)+`;
const hex = (v) => [1, 3, 5].map((i) => parseInt(v.slice(i, i + 2), 16));
const pal = P.graphPalette(tok);
const raw = P.GRAPH_MARKS.map(([k, name]) => [k, Math.round(P.contrast(hex(tok[name]), pal.page) * 100) / 100]);
const keys = Object.keys(pal.mark);
let near = [99, ""];
keys.forEach((a, i) => keys.slice(i + 1).forEach((b) => { const d = P.deltaE(pal.mark[a], pal.mark[b]); if (d < near[0]) near = [Math.round(d * 10) / 10, a + "/" + b]; }));
out({ raw, near });`)
		var res struct {
			Raw  [][]any `json:"raw"`
			Near []any   `json:"near"`
		}
		require.NoError(t, json.Unmarshal([]byte(got), &res))
		for _, r := range res.Raw {
			assert.GreaterOrEqual(t, r[1].(float64), 2.0, "theme %q: the %s token before the nudge", over, r[0])
		}
		assert.GreaterOrEqual(t, res.Near[0].(float64), 12.0, "theme %q: the closest marks %s", over, res.Near[1])
	}
}

// TestPure_ColourDifferenceIsCIE76: deltaE is the Euclidean distance of two
// colours in CIELAB, 0 for the same colour and about 100 from black to
// white.
func TestPure_ColourDifferenceIsCIE76(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.deltaE([10, 20, 30], [10, 20, 30]), Math.round(P.deltaE([0, 0, 0], [255, 255, 255]))]);`)
	assert.JSONEq(t, `[0, 100]`, got)
}
