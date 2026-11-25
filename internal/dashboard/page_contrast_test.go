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
const pal = P.graphPalette(tok);
const bg = P.parseColour(tok["--bg"]), card = P.parseColour(tok["--surface"]);
const r = (a, b) => Math.round(P.contrast(a, b) * 100) / 100;
const pairs = [];
Object.keys(pal.fill).forEach((k) => pairs.push(["text", "ink on the " + k + " fill", r(pal.ink, pal.fill[k])]));
Object.keys(pal.mark).forEach((k) => pairs.push(["line", k + " border on the canvas", r(pal.mark[k], pal.page)]));
["muted", "crit", "back", "anc", "desc", "tree"].forEach((k) => pairs.push(["line", k + " edges on the canvas", r(pal[k], pal.page)]));
pairs.push(["text", "muted text on the page (tab counts)", r(P.parseColour(tok["--muted"], bg), bg)]);
pairs.push(["text", "muted text on a card (placeholders, legend items off)", r(P.parseColour(tok["--muted"], card), card)]);
pairs.push(["line", "the (i) icon on a card", r(P.parseColour(tok["--muted"], card), card)]);
pairs.push(["line", "a control's border on a card", r(P.parseColour(tok["--line-strong"], card), card)]);
pairs.push(["text", "a link on a card", r(P.parseColour(tok["--accent"], card), card)]);
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
	assert.Contains(t, css, ".tab .c { font: var(--type-tiny); color: var(--muted);")
	assert.Contains(t, css, ".gcard #gc-q::placeholder, .gcard #tm-q::placeholder { color: var(--muted); opacity: 1; }")
	assert.Contains(t, css, "color: var(--muted); font-size: 0; }", "the (i) icon is drawn in the muted colour")
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
const pal = P.graphPalette(tok);
const raw = P.GRAPH_MARKS.map(([k, name]) => [k, Math.round(P.contrast(P.parseColour(tok[name], pal.page), pal.page) * 100) / 100]);
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
			// moot is DESIGN.md's disabled text, faint by design; the nudge
			// lifts it and the colour difference below keeps it apart.
			if r[0] == "moot" {
				continue
			}
			assert.GreaterOrEqual(t, r[1].(float64), 2.0, "theme %q: the %s token before the nudge", over, r[0])
		}
		assert.GreaterOrEqual(t, res.Near[0].(float64), 12.0, "theme %q: the closest marks %s", over, res.Near[1])
	}
}

// TestPure_ASemiTransparentTokenIsBlendedOntoWhatItSitsOn: DESIGN.md's greys
// are semi-transparent, so parseColour composites a colour's alpha over the
// colour under it (white when none is named) instead of reading it opaque.
func TestPure_ASemiTransparentTokenIsBlendedOntoWhatItSitsOn(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.parseColour("rgba(0, 6, 38, 0.9)"),
  P.parseColour("rgba(255, 255, 255, 0.78)", [18, 19, 26]),
  P.parseColour("#4C64D9", [0, 0, 0]),
  P.parseColour("rgb(1, 2, 3)"),
]);`)
	assert.JSONEq(t, `[[25, 31, 60], [203, 203, 205], [76, 100, 217], [1, 2, 3]]`, got)
}

// TestPure_ColourDifferenceIsCIE76: deltaE is the Euclidean distance of two
// colours in CIELAB, 0 for the same colour and about 100 from black to
// white.
func TestPure_ColourDifferenceIsCIE76(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.deltaE([10, 20, 30], [10, 20, 30]), Math.round(P.deltaE([0, 0, 0], [255, 255, 255]))]);`)
	assert.JSONEq(t, `[0, 100]`, got)
}

// designToken says whether name is one of DESIGN.md's colour tokens, the
// families style.css's first :root block copies from it.
func designToken(name string) bool {
	for _, p := range []string{"--text-", "--stroke-", "--fill-", "--background-", "--icon-"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// TestPage_TheGraphDrawsOnlyDesignTokens: every colour the graph reads
// (tpPure.GRAPH_TOKENS) is one of DESIGN.md's tokens, or a role of
// style.css's that names one through var(), in both themes, so the map is
// coloured as the rest of the dashboard is; the owner palette, a
// categorical one the switch turns on, is the named deviation. graph.js
// and pure.js write no colour of their own beyond parseColour's fallbacks
// and the nudge's black and white.
func TestPage_TheGraphDrawsOnlyDesignTokens(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	got := runPure(t, `out([P.GRAPH_TOKENS, P.OWNER_TOKENS]);`)
	var lists [][]string
	require.NoError(t, json.Unmarshal([]byte(got), &lists))
	owner := map[string]bool{}
	for _, n := range lists[1] {
		owner[n] = true
	}
	def := regexp.MustCompile(`(--[\w-]+):\s*([^;]+);`)
	ref := regexp.MustCompile(`^var\((--[\w-]+)\)$`)
	raw := map[string][]string{}
	for _, m := range def.FindAllStringSubmatch(css, -1) {
		raw[m[1]] = append(raw[m[1]], strings.TrimSpace(m[2]))
	}
	for _, name := range lists[0] {
		if owner[name] {
			continue
		}
		require.NotEmpty(t, raw[name], "%s is defined in style.css", name)
		for _, v := range raw[name] {
			at, ok := name, true
			for range 5 {
				if designToken(at) {
					break
				}
				r := ref.FindStringSubmatch(v)
				if r == nil {
					ok = false
					break
				}
				at = r[1]
				if len(raw[at]) > 0 {
					v = raw[at][0]
				}
			}
			assert.True(t, ok && designToken(at), "%s = %s comes from a DESIGN.md token", name, v)
		}
	}
	triple := regexp.MustCompile(`\[\d{1,3}, \d{1,3}, \d{1,3}\]`)
	allowed := []string{
		"const to = luminance(bg) < 0.18 ? [255, 255, 255] : [0, 0, 0];",
		"if (!m) return [128, 128, 128];",
		"const bg = under || [255, 255, 255];",
		"const t = vs.target || [0, 0, 0];",
		"return [1, 3, 5].map(",
		"tree: [170, 150, 1.4], treeFar: [130, 120, 1.2]",
	}
	for _, file := range []string{"/graph.js", "/pure.js"} {
		for _, line := range strings.Split(string(served(t, file)), "\n") {
			if !triple.MatchString(line) {
				continue
			}
			ok := false
			for _, a := range allowed {
				ok = ok || strings.Contains(line, a)
			}
			assert.True(t, ok, "%s writes a colour of its own: %s", file, strings.TrimSpace(line))
		}
	}
}

// TestPage_ANodeIsAQuietCardWithItsStatusOnTheBorder: as the Board's
// cards are, a node is a quiet card, its fill no further from the card
// than a weak status tint, and its state is told by its border in the
// status colour; the regions behind the nodes are the weak and weaker
// neutral fills, in both themes.
func TestPage_ANodeIsAQuietCardWithItsStatusOnTheBorder(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	for _, over := range []string{"", darkByControl} {
		tok, err := json.Marshal(themeTokens(t, css, over))
		require.NoError(t, err)
		got := runPure(t, `const tok = `+string(tok)+`;
const pal = P.graphPalette(tok);
const far = {};
Object.keys(pal.fill).forEach((k) => { far[k] = Math.round(P.deltaE(pal.fill[k], pal.surface) * 10) / 10; });
out({ far, band: pal.band.map((b) => Math.round(P.deltaE(b, pal.page) * 10) / 10) });`)
		var res struct {
			Far  map[string]float64 `json:"far"`
			Band []float64          `json:"band"`
		}
		require.NoError(t, json.Unmarshal([]byte(got), &res))
		require.NotEmpty(t, res.Far)
		for k, d := range res.Far {
			assert.LessOrEqual(t, d, 10.0, "theme %q: the %s fill is a quiet tint of the card", over, k)
		}
		require.Len(t, res.Band, 2)
		for _, d := range res.Band {
			assert.LessOrEqual(t, d, 6.0, "theme %q: a region is a weak neutral fill", over)
		}
	}
}
