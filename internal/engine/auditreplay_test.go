package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditReplayDir holds the committed projection of tp's own recorded audit
// rounds for the v0.35.0 and v0.36.0 cycles, written by its gen.go. Replay
// assertions read it and never spec/.tp-review/, which is live state for every
// cycle still in flight — a test reading that directory changes verdict when an
// unrelated cycle records a round.
const auditReplayDir = "testdata/auditreplay"

// auditReplayRowKeys are the row fields the projection carries. The `round`
// ordinal is projected alongside them and is stripped by the loader below,
// which encodes it as position instead.
var auditReplayRowKeys = map[string]bool{
	"item_id":  true,
	"role":     true,
	"severity": true,
	"status":   true,
}

// auditReplayManifest mirrors the part of testdata/auditreplay/manifest.json
// this package asserts on. The manifest is the fixture's provenance: it names
// the commit the sources were read at and carries the sha256 of every source
// and output file, so a disagreement is locatable rather than merely visible.
type auditReplayManifest struct {
	SourceCommit string                   `json:"source_commit"`
	SourcesClean bool                     `json:"sources_clean_at_commit"`
	Cycles       []auditReplayCycleRecord `json:"cycles"`
	Totals       struct {
		Rounds int `json:"rounds"`
		Rows   int `json:"rows"`
	} `json:"totals"`
}

type auditReplayCycleRecord struct {
	Cycle      string `json:"cycle"`
	Rounds     int    `json:"rounds"`
	Rows       int    `json:"rows"`
	OutputFile string `json:"output_file"`
	OutputSHA  string `json:"output_sha256"`
	Census     struct {
		Status         map[string]int `json:"status"`
		SeverityShapes map[string]int `json:"severity_shapes"`
	} `json:"census"`
}

func loadAuditReplayManifest(t *testing.T) auditReplayManifest {
	t.Helper()
	blob, err := os.ReadFile(filepath.Join(auditReplayDir, "manifest.json"))
	require.NoError(t, err)
	var m auditReplayManifest
	require.NoError(t, json.Unmarshal(blob, &m))
	return m
}

// loadAuditReplayRounds returns one recorded cycle's projected rows grouped by
// round, index i holding round i+1 — the shape engine.AuditRowsClean consumes,
// one call per round, in the order the cycle recorded them.
//
// The rows are decoded into map[string]any exactly as the record path builds
// them, so the distinction the grading rule turns on survives: a `severity` key
// holding JSON null arrives as a present key holding nil, and a key absent from
// the recorded row is absent here too. The synthesized `round` key is removed
// rather than carried, because it was never a recorded field and a caller
// reading it out of a row would be reading the fixture's own bookkeeping.
func loadAuditReplayRounds(t *testing.T, cycle string) [][]map[string]any {
	t.Helper()
	blob, err := os.ReadFile(filepath.Join(auditReplayDir, cycle+".ndjson"))
	require.NoError(t, err)

	var rounds [][]map[string]any
	for i, line := range strings.Split(strings.TrimRight(string(blob), "\n"), "\n") {
		var row map[string]any
		require.NoErrorf(t, json.Unmarshal([]byte(line), &row), "%s line %d", cycle, i+1)

		ordinal, ok := row["round"].(float64)
		require.Truef(t, ok, "%s line %d: no round ordinal", cycle, i+1)
		n := int(ordinal)
		require.GreaterOrEqualf(t, n, 1, "%s line %d: round ordinal %d", cycle, i+1, n)
		switch n {
		case len(rounds) + 1:
			rounds = append(rounds, nil)
		case len(rounds):
			// Same round as the previous row.
		default:
			require.Failf(t, "rounds are not contiguous and ascending",
				"%s line %d: round %d follows round %d", cycle, i+1, n, len(rounds))
		}
		delete(row, "round")
		rounds[n-1] = append(rounds[n-1], row)
	}
	return rounds
}

// TestAuditReplayFixtureMatchesItsManifest is the fixture's own guard: it does
// not replay anything, it establishes that what the replay assertions will read
// is what gen.go wrote. The totals are the two figures spec/0.37.0.md §7 states
// and this task counted — 22 recorded rounds, 3,046 rows — and both cycles are
// shipped and closed, so neither can legitimately grow.
func TestAuditReplayFixtureMatchesItsManifest(t *testing.T) {
	m := loadAuditReplayManifest(t)

	assert.Len(t, m.Cycles, 2)
	assert.Equal(t, 22, m.Totals.Rounds)
	assert.Equal(t, 3046, m.Totals.Rows)
	assert.Len(t, m.SourceCommit, 40, "the fixture must name the commit it was taken at")
	assert.True(t, m.SourcesClean, "the named commit does not describe the sources it was read from")

	sumRounds, sumRows := 0, 0
	for _, c := range m.Cycles {
		t.Run(c.Cycle, func(t *testing.T) { assertAuditReplayCycle(t, &c) })
		sumRounds += c.Rounds
		sumRows += c.Rows
	}
	assert.Equal(t, m.Totals.Rounds, sumRounds)
	assert.Equal(t, m.Totals.Rows, sumRows)
}

func assertAuditReplayCycle(t *testing.T, c *auditReplayCycleRecord) {
	t.Helper()

	blob, err := os.ReadFile(filepath.Join(auditReplayDir, c.OutputFile))
	require.NoError(t, err)
	sum := sha256.Sum256(blob)
	assert.Equal(t, c.OutputSHA, hex.EncodeToString(sum[:]),
		"fixture and manifest disagree: regenerate with "+
			"`go run internal/engine/testdata/auditreplay/gen.go` and commit both")

	rounds := loadAuditReplayRounds(t, c.Cycle)
	assert.Len(t, rounds, c.Rounds)

	rows, nullSeverity := 0, 0
	for _, round := range rounds {
		rows += len(round)
		for _, row := range round {
			for key := range row {
				assert.Truef(t, auditReplayRowKeys[key], "unprojected key %q", key)
			}
			if value, present := row["severity"]; present && value == nil {
				nullSeverity++
			}
		}
	}
	assert.Equal(t, c.Rows, rows)
	assert.Equal(t, c.Rows, sumCounts(c.Census.Status))
	assert.Equal(t, c.Rows, sumCounts(c.Census.SeverityShapes))
	assert.Equal(t, c.Census.SeverityShapes["null"], nullSeverity,
		"a recorded `severity: null` must reach a caller as a present key holding nil, "+
			"since that is the shape AuditSeverityBucket grades")
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, n := range counts {
		total += n
	}
	return total
}
