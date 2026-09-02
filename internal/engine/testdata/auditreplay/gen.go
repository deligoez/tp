//go:build ignore

// Command gen writes the audit replay fixture: a projection of tp's own
// recorded audit rounds for the v0.35.0 and v0.36.0 cycles.
//
// Run it from the repository root:
//
//	go run internal/engine/testdata/auditreplay/gen.go
//
// Why a projection rather than a copy. The recorded rounds live under
// spec/.tp-review/, which is live state for every cycle still in flight — a
// test that read them would be measuring a moving target, and would change
// verdict whenever an unrelated cycle records a round. The replay assertions
// read four fields per row (status, severity, role, item_id) plus the round
// ordinal they compare against, so that is exactly what is projected here and
// committed. The full round files are several times larger and carry the notes
// and evidence pointers no assertion reads.
//
// What is preserved. Each of the four projected keys is copied as its raw JSON
// value, and a key absent from the source row stays absent from the projection.
// That distinction is load-bearing: engine.AuditSeverityBucket grades an absent
// `severity`, a JSON null, a non-string value and a string outside the enum
// alike as unrecognised, and a projection that normalised them to "" would fold
// four shapes the graded rule keeps apart.
//
// What is selected. Only files named exactly audit-round-<n>.ndjson are read.
// The 0.36.0 directory also holds per-role files (audit-r9-spec-coverage.ndjson
// and its siblings) whose rows are already merged into the round files; reading
// both would double-count them.
//
// Provenance. manifest.json beside the output records the commit the sources
// were read at, the sha256 and row count of every source file, and the sha256
// of each output file. Regenerate and diff: the manifest is what makes a
// disagreement locatable rather than merely visible.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// cycles are the recorded audit cycles the fixture projects, in the order the
// manifest lists them.
var cycles = []string{"0.35.0", "0.36.0"}

// projectedKeys are the row fields the replay assertions read. The round
// ordinal is added separately — it is a property of the file a row came from,
// not of the row.
var projectedKeys = []string{"item_id", "role", "severity", "status"}

// roundFile matches a merged round file and captures its ordinal. Per-role
// files (audit-r9-spec-coverage.ndjson) deliberately do not match.
var roundFile = regexp.MustCompile(`^audit-round-(\d+)\.ndjson$`)

const (
	sourceRoot = "spec/.tp-review"
	outDir     = "internal/engine/testdata/auditreplay"
)

type sourceEntry struct {
	File   string `json:"file"`
	Round  int    `json:"round"`
	Rows   int    `json:"rows"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// census is a purely syntactic description of the projected rows: what literal
// `status` values appear, and what shapes the `severity` key takes. It embeds
// no grading rule on purpose — engine.AuditSeverityBucket owns that, and a
// second copy of the vocabulary here would be free to drift from it. Its use is
// §2.1's: the previous cycle's severity mix is recoverable only by counting,
// and this is that count, taken once and committed.
type census struct {
	Status         map[string]int `json:"status"`
	SeverityShapes map[string]int `json:"severity_shapes"`
}

type cycleEntry struct {
	Cycle       string        `json:"cycle"`
	Rounds      int           `json:"rounds"`
	Rows        int           `json:"rows"`
	SourceBytes int           `json:"source_bytes"`
	OutputFile  string        `json:"output_file"`
	OutputBytes int           `json:"output_bytes"`
	OutputSHA   string        `json:"output_sha256"`
	Census      census        `json:"census"`
	Sources     []sourceEntry `json:"sources"`
}

type totals struct {
	Rounds      int `json:"rounds"`
	Rows        int `json:"rows"`
	SourceBytes int `json:"source_bytes"`
	OutputBytes int `json:"output_bytes"`
}

type manifest struct {
	Generator      string       `json:"generator"`
	Command        string       `json:"command"`
	SourceCommit   string       `json:"source_commit"`
	SourcesClean   bool         `json:"sources_clean_at_commit"`
	SourceRoot     string       `json:"source_root"`
	ProjectedKeys  []string     `json:"projected_keys"`
	RoundOrdinal   string       `json:"round_ordinal"`
	AbsentKeysNote string       `json:"absent_keys"`
	Cycles         []cycleEntry `json:"cycles"`
	Totals         totals       `json:"totals"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	commit, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	m := manifest{
		Generator:     filepath.Join(outDir, "gen.go"),
		Command:       "go run " + filepath.Join(outDir, "gen.go"),
		SourceCommit:  commit,
		SourceRoot:    sourceRoot,
		ProjectedKeys: projectedKeys,
		RoundOrdinal: "added as \"round\"; it is the n of the source file's " +
			"audit-round-<n>.ndjson name, not a recorded field",
		AbsentKeysNote: "a projected key absent from the source row stays absent here; " +
			"null is preserved as null",
	}

	// The commit above names the sources only if the source directories are
	// committed at it, so that is checked rather than assumed.
	var dirty string
	for _, cycle := range cycles {
		d, err := gitOutput("status", "--porcelain", "--", filepath.Join(sourceRoot, cycle))
		if err != nil {
			return err
		}
		dirty += d
	}
	m.SourcesClean = strings.TrimSpace(dirty) == ""

	for _, cycle := range cycles {
		entry, err := projectCycle(cycle)
		if err != nil {
			return err
		}
		m.Cycles = append(m.Cycles, entry)
		m.Totals.Rounds += entry.Rounds
		m.Totals.Rows += entry.Rows
		m.Totals.SourceBytes += entry.SourceBytes
		m.Totals.OutputBytes += entry.OutputBytes
	}

	blob, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(blob, '\n'), 0o644); err != nil {
		return err
	}

	fmt.Printf("rounds=%d rows=%d source_bytes=%d output_bytes=%d commit=%s clean=%v\n",
		m.Totals.Rounds, m.Totals.Rows, m.Totals.SourceBytes, m.Totals.OutputBytes,
		m.SourceCommit, m.SourcesClean)
	for _, c := range m.Cycles {
		fmt.Printf("  %s: rounds=%d rows=%d source_bytes=%d output_bytes=%d\n",
			c.Cycle, c.Rounds, c.Rows, c.SourceBytes, c.OutputBytes)
	}
	return nil
}

// projectCycle reads one cycle's merged round files and writes its projection.
func projectCycle(cycle string) (cycleEntry, error) {
	entry := cycleEntry{
		Cycle:      cycle,
		OutputFile: cycle + ".ndjson",
		Census: census{
			Status:         map[string]int{},
			SeverityShapes: map[string]int{},
		},
	}

	dir := filepath.Join(sourceRoot, cycle)
	names, err := os.ReadDir(dir)
	if err != nil {
		return entry, err
	}
	ordinals := map[int]string{}
	for _, n := range names {
		match := roundFile.FindStringSubmatch(n.Name())
		if match == nil {
			continue
		}
		ordinal, err := strconv.Atoi(match[1])
		if err != nil {
			return entry, err
		}
		if prior, dup := ordinals[ordinal]; dup {
			return entry, fmt.Errorf("%s: round %d claimed by both %s and %s", dir, ordinal, prior, n.Name())
		}
		ordinals[ordinal] = n.Name()
	}
	rounds := make([]int, 0, len(ordinals))
	for ordinal := range ordinals {
		rounds = append(rounds, ordinal)
	}
	sort.Ints(rounds)
	for i, ordinal := range rounds {
		if ordinal != i+1 {
			return entry, fmt.Errorf("%s: round ordinals are not 1..%d (saw %d at position %d)",
				dir, len(rounds), ordinal, i+1)
		}
	}

	var out strings.Builder
	for _, ordinal := range rounds {
		path := filepath.Join(dir, ordinals[ordinal])
		raw, err := os.ReadFile(path)
		if err != nil {
			return entry, err
		}
		rows, err := projectRows(raw, ordinal, &out, &entry.Census)
		if err != nil {
			return entry, fmt.Errorf("%s: %w", path, err)
		}
		sum := sha256.Sum256(raw)
		entry.Sources = append(entry.Sources, sourceEntry{
			File:   path,
			Round:  ordinal,
			Rows:   rows,
			Bytes:  len(raw),
			SHA256: hex.EncodeToString(sum[:]),
		})
		entry.Rows += rows
		entry.SourceBytes += len(raw)
	}
	entry.Rounds = len(rounds)

	blob := []byte(out.String())
	sum := sha256.Sum256(blob)
	entry.OutputBytes = len(blob)
	entry.OutputSHA = hex.EncodeToString(sum[:])
	return entry, os.WriteFile(filepath.Join(outDir, entry.OutputFile), blob, 0o644)
}

// jsonShape names a raw JSON value's shape for the census: an absent key, a
// JSON null, a string (reported with its literal value, since the vocabulary is
// small and closed) or the kind of whatever else it is.
func jsonShape(raw json.RawMessage, present bool) string {
	switch {
	case !present:
		return "absent"
	case string(raw) == "null":
		return "null"
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return "string:" + s
	}
	return "non-string"
}

// projectRows appends one round file's projected rows to out and reports how
// many it wrote. Blank lines are skipped; anything else that is not a JSON
// object is an error rather than a silent drop, because a dropped row moves
// every count the fixture exists to pin.
func projectRows(raw []byte, ordinal int, out *strings.Builder, c *census) (int, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	rows := 0
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal([]byte(text), &row); err != nil {
			return rows, fmt.Errorf("line %d: %w", line, err)
		}
		projected := map[string]json.RawMessage{
			"round": json.RawMessage(strconv.Itoa(ordinal)),
		}
		for _, key := range projectedKeys {
			if value, ok := row[key]; ok {
				projected[key] = value
			}
		}
		status, statusPresent := row["status"]
		c.Status[jsonShape(status, statusPresent)]++
		severity, severityPresent := row["severity"]
		c.SeverityShapes[jsonShape(severity, severityPresent)]++
		// json.Marshal sorts map keys, so the output is byte-stable across runs.
		blob, err := json.Marshal(projected)
		if err != nil {
			return rows, fmt.Errorf("line %d: %w", line, err)
		}
		out.Write(blob)
		out.WriteByte('\n')
		rows++
	}
	if err := scanner.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	blob, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(blob)), nil
}
