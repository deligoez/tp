package dashboard_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/dashboard"
)

const testPort = 7377

// serve sends one request through the handler bound to testPort.
func serve(t *testing.T, method, host, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, "http://"+host+path, http.NoBody)
	req.Host = host
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	dashboard.Handler(testPort, nil).ServeHTTP(rec, req)
	return rec.Result()
}

// assertNoCORS fails when a response carries any CORS header.
func assertNoCORS(t *testing.T, resp *http.Response) {
	t.Helper()
	for name := range resp.Header {
		assert.False(t, strings.HasPrefix(strings.ToLower(name), "access-control-"), "response carries CORS header %s", name)
	}
}

// TestHandler_RefusesEveryMethodButGET is §1's method rule: GET serves, and
// every other method is refused before anything is served.
func TestHandler_RefusesEveryMethodButGET(t *testing.T) {
	t.Parallel()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assertNoCORS(t, resp)
	for _, m := range []string{http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		resp := serve(t, m, "127.0.0.1:7377", "/")
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, m)
		assertNoCORS(t, resp)
	}
}

// TestHandler_RefusesAForeignHost is §1's Host rule: the bound address and
// port, or localhost with that port, serve; any other Host is refused, which
// is what closes DNS rebinding.
func TestHandler_RefusesAForeignHost(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"127.0.0.1:7377", "localhost:7377", "LOCALHOST:7377"} {
		assert.Equal(t, http.StatusOK, serve(t, http.MethodGet, host, "/").StatusCode, host)
	}
	for _, host := range []string{"evil.example:7377", "127.0.0.1:7378", "localhost:8080", "127.0.0.1", "localhost", "0.0.0.0:7377", "[::1]:7377"} {
		resp := serve(t, http.MethodGet, host, "/")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, host)
		assertNoCORS(t, resp)
	}
}

// TestHandler_ServesThePageScriptAndStylesFromTheEmbed is §1's embed rule:
// the page links its script and styles, and each is served from the binary.
func TestHandler_ServesThePageScriptAndStylesFromTheEmbed(t *testing.T) {
	t.Parallel()
	page := serve(t, http.MethodGet, "127.0.0.1:7377", "/")
	body, err := io.ReadAll(page.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `src="app.js"`)
	assert.Contains(t, string(body), `href="style.css"`)

	for path, kind := range map[string]string{"/app.js": "javascript", "/style.css": "text/css"} {
		resp := serve(t, http.MethodGet, "127.0.0.1:7377", path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		assert.Contains(t, resp.Header.Get("Content-Type"), kind, path)
	}
}

// pageText is the page, its script and its styles as the handler serves them.
func pageText(t *testing.T) (page, script string) {
	t.Helper()
	var all strings.Builder
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		resp := serve(t, http.MethodGet, "127.0.0.1:7377", path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		all.Write(body)
		if path == "/app.js" {
			script = string(body)
		}
	}
	return all.String(), script
}

// TestPage_ListensToTheEventStreamAndLoadsNothingRemote is §2 and §3 at the
// page: its script subscribes to the event stream and to each of the three
// payloads by its event name, and neither the page, the script nor the
// styles reference a resource over http or https, so the page needs no
// network beside the dashboard it came from.
func TestPage_ListensToTheEventStreamAndLoadsNothingRemote(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, name := range []string{`"/api/events"`, `"epic-export"`, `"export-list"`, `"summary"`} {
		assert.Contains(t, script, name)
	}
	assert.NotContains(t, page, "http://")
	assert.NotContains(t, page, "https://")
}

// TestPage_ReadsTheKPIsFromThePayloads is §4's KPI strip at the page: the
// lint KPI reads the epic export's lint errors and warnings, the counts tp
// epic lint prints, and every other figure and now list is read from the
// summary's plan, so the page counts nothing itself.
func TestPage_ReadsTheKPIsFromThePayloads(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, field := range []string{
		"lint.errors", "lint.warnings",
		"plan.figures", ".share_done", ".ready", ".critical_path", ".blocked", ".held", ".operator_queue", ".open_loops", ".throughput",
		"plan.ready", "plan.operator_conditions", "plan.waiting", "plan.most_waited_on",
	} {
		assert.Contains(t, script, field)
	}
}

// TestPage_DrawsTheOperatorRowsFromThePayloads is §7's list at the page: it
// reads each slice's grantable and offers --grant-round on it, the
// placeholder thresholds to rewrite, the any-of groups to choose in, and the
// task file list's escalations, so it judges no row itself.
func TestPage_DrawsTheOperatorRowsFromThePayloads(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, field := range []string{".grantable", "--grant-round", ".threshold_rewrites", "plan.operator_groups", ".escalations"} {
		assert.Contains(t, script, field)
	}
}

// TestPage_KeepsTheSinceLastLookedStateInTheBrowserAlone is §17's strip at
// the page: it counts tasks closed, rounds recorded and epics whose status
// changed, from the stored and the current payloads, and every access to the
// browser's storage sits inside a try, so a browser that refuses storage
// loses only the strip.
func TestPage_KeepsTheSinceLastLookedStateInTheBrowserAlone(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, field := range []string{"tasks closed", "rounds recorded", "epics changed stage", ".tasks", ".review", ".audit", ".status", ".slices"} {
		assert.Contains(t, script, field)
	}
	uses := regexp.MustCompile(`localStorage\.`).FindAllStringIndex(script, -1)
	require.NotEmpty(t, uses, "the strip keeps its state in localStorage")
	for _, u := range uses {
		assert.Contains(t, script[max(0, u[0]-40):u[0]], "try {", "localStorage at byte %d is outside a try", u[0])
	}
}

// TestPage_DrawsASpecsHistoryFromItsExportedRounds is §13's History at the
// page: the rounds on a time axis by their recorded time, the round table
// from each loop's rounds with their finding or FAIL count, clean flag, cap
// and streak, and each review round's snapshot difference drawn beside that
// round's findings, all read from tp export's loops.
func TestPage_DrawsASpecsHistoryFromItsExportedRounds(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, field := range []string{"sv-history", ".recorded_at", ".fails", ".clean", ".cap", ".streak", ".clean_rounds_needed", ".snapshot_diff", ".changed", ".removed", ".finding_rows"} {
		assert.Contains(t, script, field)
	}
}

// TestPage_DrawsPlanReviewFromTheSliceFields is §5's panel at the page: each
// slice row reads the export's slices with their cap, grants spent, stale
// unit ids and open findings by role, and the summary's per-round counts,
// so the page derives no staleness cause and no count itself.
func TestPage_DrawsPlanReviewFromTheSliceFields(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, field := range []string{".slices", ".max_rounds", ".grants_spent", ".stale_units", ".open_by_role", ".rounds[", ".total"} {
		assert.Contains(t, script, field)
	}
}

// TestPage_DrawsTheMapFromTheExportedLayout is §9 at the page: every epic is
// placed by the export's layout (its lane, the lane's kind, its milestone's
// column and its row) and the critical path by tp epic status's by_count,
// and the script never reads opens_after, so it cannot lay the map out again.
func TestPage_DrawsTheMapFromTheExportedLayout(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, field := range []string{".layout", ".lane", ".lane_kind", ".column", ".row", "critical_path.by_count", ".depends_on", ".any_of"} {
		assert.Contains(t, script, field)
	}
	assert.NotContains(t, script, "opens_after")
}

// TestPage_FoldsTurkishLettersInTheEpicSearch is §9's search rule at the
// page: the folding table the search applies before Unicode decomposition
// maps the Turkish dotless and dotted i, and the letters decomposition alone
// would not reduce reliably, to their plain Latin letter.
func TestPage_FoldsTurkishLettersInTheEpicSearch(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	m := regexp.MustCompile(`const FOLD = (\{[^}]*\});`).FindStringSubmatch(script)
	require.NotNil(t, m, "the script declares the folding table as const FOLD = {...};")
	var fold map[string]string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &fold))
	for from, to := range map[string]string{"ı": "i", "İ": "i", "ş": "s", "ğ": "g", "ç": "c", "ö": "o", "ü": "u"} {
		assert.Equal(t, to, fold[from], "FOLD maps %s", from)
	}
}

// TestPage_DrawsTheLifecycleChartFromTheSeries is §17's chart at the page: it
// reads the summary's series and stacks one band per stage after planned plus
// the unknown band, so an epic whose stage time tp did not record is drawn as
// unknown rather than on a guessed day, and its ⓘ says an epic's planning
// time is not recorded.
func TestPage_DrawsTheLifecycleChartFromTheSeries(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "sum.series")
	m := regexp.MustCompile(`const CFD_BANDS = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, m, "the script declares the chart's bands as const CFD_BANDS = [...];")
	var bands [][]string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &bands))
	keys := make([]string, 0, len(bands))
	for _, b := range bands {
		keys = append(keys, b[0])
	}
	assert.Equal(t, []string{"done", "in_progress", "specced", "claimed", "unknown"}, keys)
	assert.Contains(t, script, "planning time is not recorded, so an epic added to the graph later counts as planned from the chart's first day")
}

// TestPage_FiltersTheMapToOneEpicsNeighbours is §9's neighbours filter at the
// page: a toggle keeps the selected epic with its direct dependencies and
// dependents, each drawn edge is marked incoming or outgoing by the direction
// table the script declares, and the edge carries its why for the hover tip.
func TestPage_FiltersTheMapToOneEpicsNeighbours(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="f-nb"`)
	assert.Contains(t, script, "function neighboursOf(")
	m := regexp.MustCompile(`const NB_DIR = (\{[^}]*\});`).FindStringSubmatch(script)
	require.NotNil(t, m, "the script declares the edge directions as const NB_DIR = {...};")
	var dirs map[string]string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &dirs))
	assert.ElementsMatch(t, []string{"in", "out"}, keys(dirs))
	assert.Contains(t, script, `data-why="`)
	for _, sel := range []string{".edge.in", ".edge.out"} {
		assert.Contains(t, page, sel)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestPage_BoardColumnsPartitionTheExportedTaskStates is §13's Board at the
// page: its columns, as the script declares them, take every task state tp
// export reports exactly once, ready and waiting holding the open tasks, so
// the column counts add up to tp status's open, wip and done; and the Tasks
// view reads the task fields its side panel shows.
func TestPage_BoardColumnsPartitionTheExportedTaskStates(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	m := regexp.MustCompile(`const BOARD = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, m, "the script declares the Board's columns as const BOARD = [...];")
	var cols [][]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(m[1]), &cols))
	byCol := map[string][]string{}
	seen := map[string]int{}
	for _, c := range cols {
		require.Len(t, c, 3, "each column is [key, label, states]")
		var key string
		var states []string
		require.NoError(t, json.Unmarshal(c[0], &key))
		require.NoError(t, json.Unmarshal(c[2], &states))
		byCol[key] = states
		for _, s := range states {
			seen[s]++
		}
	}
	assert.Equal(t, map[string]int{"ready": 1, "waits_on_task": 1, "waits_on_epic": 1, "wip": 1, "done": 1}, seen)
	assert.ElementsMatch(t, []string{"ready", "waits_on_task", "waits_on_epic"}, append(append([]string{}, byCol["ready"]...), byCol["waiting"]...))
	assert.Equal(t, []string{"wip"}, byCol["wip"])
	assert.Equal(t, []string{"done"}, byCol["done"])
	for _, field := range []string{".state", ".waits_on", ".depends_on", ".estimate_minutes", ".started_at", ".closed_at", ".closed_reason", ".commit_shas", ".acceptance", ".source_sections"} {
		assert.Contains(t, script, field)
	}
}

// TestPage_ReadsEveryAddressKey is §14 at the page: the address keys the
// script declares cover the tab, the spec, its view, the picked task, the
// selected epic and every active filter, and the script reads each one back
// from the address, so a copied link lands where its writer was.
func TestPage_ReadsEveryAddressKey(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	m := regexp.MustCompile(`const URL_KEYS = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, m, "the script declares the address keys as const URL_KEYS = [...];")
	var urlKeys []string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &urlKeys))
	assert.Subset(t, urlKeys, []string{"tab", "spec", "view", "task", "epic", "tracks", "edges", "nb", "q", "phase", "sq", "tview"})
	for _, k := range urlKeys {
		assert.Contains(t, script, `p.get("`+k+`")`, "the script reads %s from the address", k)
	}
	assert.Contains(t, script, `"popstate"`)
}

// TestPage_DrawsASlicesGraphHistoryFromTheHistoryEndpoint is §11 at the page:
// a slice's history is fetched from /api/history, tp epic review --history's
// payload, its rounds read with their findings by role and their difference
// from the round before, two chosen rounds fetched with from and to, every
// part of a difference drawn, and the picked slice and rounds kept in the
// address.
func TestPage_DrawsASlicesGraphHistoryFromTheHistoryEndpoint(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="slice-history"`)
	assert.Contains(t, script, `"/api/history?"`)
	for _, field := range []string{".findings_by_role", ".diff_from", ".current", ".units_added", ".units_removed", ".units_changed",
		".edges_added", ".edges_removed", ".whys_changed", ".fields_changed", ".hashes_only", `"from"`, `"to"`} {
		assert.Contains(t, script, field)
	}
	m := regexp.MustCompile(`const URL_KEYS = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, m)
	var urlKeys []string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &urlKeys))
	assert.Subset(t, urlKeys, []string{"slice", "hr"})
}

// TestPage_DrawsTheBandAndMilestoneStateFromThePayloads is §6 at the page:
// the band's cells read the summary's band stages and phases, a stage
// filters the map by the export's epics[].status, kept in the address, and
// each milestone entry reads the export's open, waiting_on and state (members
// done of members, the threshold's state), so the page counts nothing itself.
func TestPage_DrawsTheBandAndMilestoneStateFromThePayloads(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="msstate"`)
	for _, field := range []string{"band.stages", "band.phases", "e.status === mapView.stage", ".waiting_on", ".members_done", ".members", ".threshold", "data-stage", "data-phase"} {
		assert.Contains(t, script, field)
	}
	m := regexp.MustCompile(`const THRESHOLD_LABEL = (\{[^}]*\});`).FindStringSubmatch(script)
	require.NotNil(t, m, "the script declares the threshold states as const THRESHOLD_LABEL = {...};")
	var labels map[string]string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &labels))
	assert.ElementsMatch(t, []string{"written", "absent", "placeholder", "met"}, keys(labels))
	k := regexp.MustCompile(`const URL_KEYS = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, k)
	var urlKeys []string
	require.NoError(t, json.Unmarshal([]byte(k[1]), &urlKeys))
	assert.Contains(t, urlKeys, "stage")
	assert.Contains(t, script, `p.get("stage")`)
}

// TestPage_ReadsTheSpecInAnchoredSections is §13's Read view at the page: the
// spec's text is drawn from tp export's sections and their paragraphs, each
// carrying the anchor tp computed, a section's task count opens Tasks filtered
// to it with the filter kept in the address.
func TestPage_ReadsTheSpecInAnchoredSections(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="sv-read"`)
	for _, field := range []string{".sections", ".paragraphs", ".anchor", ".heading", ".text", ".source_sections", "data-sec"} {
		assert.Contains(t, script, field)
	}
	m := regexp.MustCompile(`const URL_KEYS = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, m)
	var urlKeys []string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &urlKeys))
	assert.Contains(t, urlKeys, "sec")
	assert.Contains(t, script, `p.get("sec")`)
}

// TestPage_AWaitingEpicNamesItsAnyOfGroups is the Waiting list's fix: an
// epic held only by an any-of group printed "waits on" with nothing after it,
// because the row read blocked_by alone; it reads the export's
// blocked_by_any_of too and names each group's members as one of them.
func TestPage_AWaitingEpicNamesItsAnyOfGroups(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, ".blocked_by_any_of")
	assert.Contains(t, script, `"one of "`)
}

// TestPage_DrawsAnEpicsDescriptionAsEscapedPreformattedText is the drawer's
// description (spec/backlog/epic-description.md): the graph epic's
// description goes through descHTML, which wraps the escaped text in a <pre>
// and renders no markup. With node on PATH the page's own esc and descHTML
// run on a description holding a script and HTML, which come out as text.
func TestPage_DrawsAnEpicsDescriptionAsEscapedPreformattedText(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "descHTML(ge.description)")
	var defs []string
	for line := range strings.SplitSeq(script, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "const esc = ") || strings.HasPrefix(strings.TrimSpace(line), "const descHTML = ") {
			defs = append(defs, line)
		}
	}
	require.Len(t, defs, 2, "the page defines esc and descHTML on one line each")
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH: the static half above ran")
	}
	prog := strings.Join(defs, "\n") + "\nprocess.stdout.write(descHTML(\"<script>alert(1)</script>\\n<b>bold</b> & 'q'\"));\n"
	out, err := exec.CommandContext(t.Context(), node, "-e", prog).Output()
	require.NoError(t, err)
	assert.Equal(t, `<div><h4>Description</h4><pre class="ddesc">&lt;script&gt;alert(1)&lt;/script&gt;`+"\n"+`&lt;b&gt;bold&lt;/b&gt; &amp; &#39;q&#39;</pre></div>`, string(out))
}

// TestPage_DrawsTheEpicDrawerItsChainAndTheMatrix is §4 and §11 at the page:
// a selected epic opens a drawer reading the export's fields, downstream and
// bound files with their phase and loops, the graph's whys and claim for its
// stepper, and its graph history from /api/epic-history, a hash-only round
// and an epic in no slice each said; the chain panel and the milestone matrix,
// read from the summary's matrix, have their own cards.
func TestPage_DrawsTheEpicDrawerItsChainAndTheMatrix(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, id := range []string{`id="drawer"`, `id="chaincard"`, `id="matrixcard"`} {
		assert.Contains(t, page, id)
	}
	assert.Contains(t, script, `"/api/epic-history?epic="`)
	for _, field := range []string{".hashes_only", ".message", "sum.matrix", ".edges", ".claimed", ".bound_at", ".done_at", ".downstream", ".depends_on", ".why", ".phase", ".review", ".audit"} {
		assert.Contains(t, script, field)
	}
}
