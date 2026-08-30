package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// captureNotices is captureAuditRoundNotices under the name this file's subject
// reads by: output.Notice is one channel, and §3.2.2's run-start report shares
// it with every other advisory.
func captureNotices(t *testing.T, fn func()) string {
	t.Helper()
	return captureAuditRoundNotices(t, fn)
}

// absentLog is the table's way of saying "no file at all", which is a different
// case from an empty one and has to be reachable without a second table.
const absentLog = "\x00absent"

// fp returns a pointer to a float, for a table that has to distinguish a
// reported zero from no measurement at all.
func fp(v float64) *float64 { return &v }

// §3.2.2: after a child exits the driver reads one number from the final line
// of that unit's log — the key its runner declares, and nothing else.
//
// Every arm that expects nil is a separate way of having no measurement, and
// they are all null rather than zero: a runner that declares no key, a key the
// line does not carry, a value that is not a number, a line that is not JSON.
// Reporting zero for any of them would be a cost nobody measured.
func TestReadSpend(t *testing.T) {
	cases := []struct {
		name string
		key  string
		log  string
		want *float64
	}{
		{
			name: "the final line's key",
			key:  claudeSpendKey,
			log:  "{\"type\":\"assistant\"}\n{\"type\":\"result\",\"total_cost_usd\":1.25}\n",
			want: fp(1.25),
		},
		{
			name: "trailing blank lines are not the final line",
			key:  claudeSpendKey,
			log:  "{\"total_cost_usd\":0.5}\n\n   \n",
			want: fp(0.5),
		},
		{
			name: "the last line wins",
			key:  claudeSpendKey,
			log:  "{\"total_cost_usd\":9}\n{\"total_cost_usd\":2}\n",
			want: fp(2),
		},
		{
			name: "a log with no trailing newline",
			key:  claudeSpendKey,
			log:  "{\"total_cost_usd\":3}",
			want: fp(3),
		},
		{
			name: "a dot path walks into the nested object",
			key:  "usage.cost.total_usd",
			log:  "{\"usage\":{\"cost\":{\"total_usd\":4.5}}}\n",
			want: fp(4.5),
		},
		{
			name: "a runner declaring no spend_key reads nothing",
			key:  "",
			log:  "{\"total_cost_usd\":1.25}\n",
			want: nil,
		},
		{
			name: "a key the final line does not carry",
			key:  claudeSpendKey,
			log:  "{\"other\":1.25}\n",
			want: nil,
		},
		{
			name: "a dot path through a value that is not an object",
			key:  "usage.total",
			log:  "{\"usage\":1.25}\n",
			want: nil,
		},
		{
			name: "a value that is not a number",
			key:  claudeSpendKey,
			log:  "{\"total_cost_usd\":\"1.25\"}\n",
			want: nil,
		},
		{
			name: "a final line that is not JSON",
			key:  claudeSpendKey,
			log:  "{\"total_cost_usd\":1.25}\nrun finished.\n",
			want: nil,
		},
		{
			name: "an empty log",
			key:  claudeSpendKey,
			log:  "",
			want: nil,
		},
		{
			name: "a log the child never wrote",
			key:  claudeSpendKey,
			log:  absentLog,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "unit.jsonl")
			if tc.log != absentLog {
				require.NoError(t, os.WriteFile(path, []byte(tc.log), 0o600))
			}

			got := readSpend(path, tc.key)

			if tc.want == nil {
				assert.Nil(t, got, "no number was read, so the row reports spend null")
				return
			}
			require.NotNil(t, got)
			assert.InDelta(t, *tc.want, *got, 1e-9)
		})
	}
}

// A final line longer than the bounded tail read is not guessed at: the read
// starts mid-line, so the bytes it holds are a fragment of a line rather than a
// line.
//
// The fixture is aligned so the fragment is itself valid JSON carrying the key,
// which is the only arrangement that can tell the guard from the JSON parser
// that would otherwise refuse the fragment anyway. A single line of padding
// followed by an object exactly as long as the tail puts the read's start
// precisely on the object's opening brace, so a reader without the guard
// reports 2.5 from bytes it cannot know are the whole line.
func TestReadSpend_AFinalLineLongerThanTheTailIsNotGuessedAt(t *testing.T) {
	const objectFixedBytes = len(`{"pad":"`) + len(`","total_cost_usd":2.5}`)
	object := `{"pad":"` + strings.Repeat("x", spendTailBytes-objectFixedBytes) + `","total_cost_usd":2.5}`
	require.Len(t, object, spendTailBytes, "the fragment must start exactly where the tail read does")
	require.NotNil(t, spendFromLine([]byte(object), claudeSpendKey),
		"control: the fragment is valid JSON carrying the key, so only the guard can refuse it")

	path := filepath.Join(t.TempDir(), "big.jsonl")
	require.NoError(t, os.WriteFile(path, []byte("Z"+object), 0o600))

	assert.Nil(t, readSpend(path, claudeSpendKey),
		"the tail read starts mid-line, so there is no final line to read a number from")
}

// §3.2.2's run-start question: which unit kinds does this run's runner meter?
//
// The per-kind arm is the one that matters, because that is where a partial
// total can be mistaken for a complete one.
func TestUnmeteredKinds(t *testing.T) {
	cases := []struct {
		name   string
		runner string
		want   []UnitKind
	}{
		{"the default template meters every kind", ``, nil},
		{"claude by name meters every kind", `"claude"`, nil},
		{"opencode declares no spend_key", `"opencode"`, UnitKinds()},
		{
			"a runner object carrying spend_key",
			`{"cmd":"x","spend_key":"cost"}`,
			nil,
		},
		{
			"a runner object without one",
			`{"cmd":"x"}`,
			UnitKinds(),
		},
		{
			"a per-kind map naming its one unmetered kind",
			`{"implement":"claude","audit-role":"opencode","default":"claude"}`,
			[]UnitKind{UnitAuditRole},
		},
		{
			"a per-kind map whose default is unmetered",
			`{"implement":"claude","default":"opencode"}`,
			[]UnitKind{UnitReviewRole, UnitReviewRecord, UnitReviewResolve,
				UnitDecompose, UnitAuditRole, UnitAuditRecord, UnitAuditFix},
		},
		{"a runner value no shape accepts reports nothing", `{"nope":1}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The seam outranks the runner field entirely, so a value
			// leaking in from the ambient environment would silently
			// replace the runner under test.
			t.Setenv(EnvRunnerSeam, "")

			got := unmeteredKinds(json.RawMessage(tc.runner))

			assert.Equal(t, tc.want, got, "the unmetered kinds, in §3.3's table order")
		})
	}
}

// The seam declares the claude template's key, so a run under it meters
// everything and reports nothing at start.
func TestUnmeteredKinds_TheSeamMetersEveryKind(t *testing.T) {
	t.Setenv(EnvRunnerSeam, "/bin/true")
	assert.Empty(t, unmeteredKinds(json.RawMessage(`"opencode"`)),
		"the seam replaces the field, so what the field declares does not decide this")
}

// The report itself: silence when everything is metered, and the kinds named
// when only some of them are.
func TestSpendNotice(t *testing.T) {
	assert.Empty(t, spendNotice(nil), "a fully metered run reports nothing at start")

	all := spendNotice(UnitKinds())
	assert.Contains(t, all, "no spend_key")
	assert.Contains(t, all, "run_max_budget_usd")
	assert.NotContains(t, all, string(UnitImplement),
		"a runner that meters nothing at all names no kind: there is no partial total to explain")

	partial := spendNotice([]UnitKind{UnitAuditRole})
	assert.Contains(t, partial, string(UnitAuditRole), "the unmetered kind is named")
	assert.NotContains(t, partial, string(UnitImplement), "the metered kinds are not")
}

// §3.2.2 end to end: the driver reads the number the runner's final log line
// carries and records it against the unit that spent it.
func TestRunDriver_ReadsSpendFromTheRunnerLog(t *testing.T) {
	root, spec, taskFile, _ := seamProject(t, twoOpenTasks)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSpend, "1.25")

	res := driveOnce(t, root, spec, taskFile, driverWorkflow())
	require.Equal(t, StopConverged, res.StopReason)

	st := readRunStateFile(t, root, taskFile)
	require.Len(t, st.Units, 2, "one implement unit per open task")
	for _, row := range st.Units {
		require.NotNil(t, row.SpendUSD, "seq %d reported no spend", row.Seq)
		assert.InDelta(t, 1.25, *row.SpendUSD, 1e-9)
	}
	assert.InDelta(t, 2.5, st.Totals.SpendUSD, 1e-9, "the run accrues what its units reported")
}

// The spend the driver reads is what cap-budget bounds, so the cap trips from a
// real run rather than only from a hand-built total.
func TestRunDriver_CapBudgetStopsTheRun(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, twoOpenTasks)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSpend, "1")

	wf := driverWorkflow()
	wf.RunMaxBudgetUSD = 0.5
	res := driveOnce(t, root, spec, taskFile, wf)

	assert.Equal(t, StopCapBudget, res.StopReason,
		"the first unit's reported spend reached the cap")
	assert.Len(t, invocations(t, records), 1, "the cap stops the run after the iteration that reached it")
}

// fakeRunnerProject wires a temp project to the fake runner as an ordinary
// configured runner rather than through the seam, which is the only way to
// reach a runner that declares no spend_key: the seam always declares one.
//
// spendKey empty omits the field entirely, so the runner object is exactly the
// unmetered shape §3.2.2 describes.
func fakeRunnerProject(t *testing.T, tasksJSON, spendKey string) (root, spec, taskFile string, wf *model.Workflow) {
	t.Helper()
	root, spec, taskFile = setupResumeProject(t, tasksJSON)
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))
	t.Setenv(EnvRunnerSeam, "")
	t.Setenv(fakerunner.EnvDir, records)

	runner := map[string]any{
		"cmd":  bin,
		"args": []string{"{unit_kind}", "{unit_id}", "{log_path}", "{max_budget_usd}"},
	}
	if spendKey != "" {
		runner["spend_key"] = spendKey
	}
	raw, err := json.Marshal(runner)
	require.NoError(t, err)
	wf = driverWorkflow()
	wf.Runner = raw
	return root, spec, taskFile, wf
}

// Test 66: a runner declaring no spend_key is reported once at run start, its
// units report spend null, and run_max_budget_usd is inert for them.
//
// The cap is set below what the child writes into its own log, so a driver that
// read the number anyway would stop with cap-budget after one unit. The log is
// asserted to carry that number, which is what makes this a test of the
// declaration rather than of an empty log.
func TestRunDriver_UnmeteredRunnerReportsNullAndLeavesCapBudgetInert(t *testing.T) {
	root, spec, taskFile, wf := fakeRunnerProject(t, twoOpenTasks, "")
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSpend, "5")
	wf.RunMaxBudgetUSD = 0.01

	var res DriverResult
	notices := captureNotices(t, func() { res = driveOnce(t, root, spec, taskFile, wf) })

	assert.Equal(t, StopConverged, res.StopReason,
		"run_max_budget_usd is inert for a runner that meters nothing")
	assert.Equal(t, 1, strings.Count(notices, "no spend_key"),
		"the report is made once, at run start, and not per unit")

	st := readRunStateFile(t, root, taskFile)
	require.Len(t, st.Units, 2)
	for _, row := range st.Units {
		assert.Nil(t, row.SpendUSD, "seq %d belongs to a runner that declares no spend_key", row.Seq)
	}
	assert.Zero(t, st.Totals.SpendUSD, "an unmetered run accrues nothing")

	log, err := os.ReadFile(st.Units[0].LogPath)
	require.NoError(t, err)
	assert.Contains(t, string(log), claudeSpendKey,
		"control: the number was in the log and was left unread because the runner declares no key")
}

// The other half of test 66: a per-kind map with one metered kind and one
// unmetered kind names the unmetered one at run start.
func TestRunDriver_PerKindMapNamesItsUnmeteredKinds(t *testing.T) {
	root, spec, taskFile, wf := fakeRunnerProject(t, twoOpenTasks, claudeSpendKey)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSpend, "1.25")

	var runner map[string]any
	require.NoError(t, json.Unmarshal(wf.Runner, &runner))
	raw, err := json.Marshal(map[string]any{"implement": runner, "default": "opencode"})
	require.NoError(t, err)
	wf.Runner = raw

	var res DriverResult
	notices := captureNotices(t, func() { res = driveOnce(t, root, spec, taskFile, wf) })
	require.Equal(t, StopConverged, res.StopReason)

	assert.Contains(t, notices, string(UnitAuditRole), "the kinds the default leaves unmetered are named")
	assert.NotContains(t, notices, string(UnitImplement), "the metered kind is not")

	st := readRunStateFile(t, root, taskFile)
	require.Len(t, st.Units, 2)
	for _, row := range st.Units {
		require.NotNil(t, row.SpendUSD, "seq %d is an implement unit, and that entry meters", row.Seq)
		assert.InDelta(t, 1.25, *row.SpendUSD, 1e-9)
	}
}
