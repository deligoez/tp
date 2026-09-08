package engine

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnitKind_BriefCommand_DocumentedPerKind pins every kind's brief to the
// command §3.3.1 documents for it, byte for byte. The spec path is a literal so
// a change in the rendering shows up as a diff in the expectation, not in a
// helper both sides share.
func TestUnitKind_BriefCommand_DocumentedPerKind(t *testing.T) {
	target := UnitTarget{
		TaskFile: "spec/0.35.0.tasks.json",
		Spec:     "spec/0.35.0.md",
		RoundDir: "/repo/.tp/rounds/0.35.0/review-r3",
		Round:    3,
		ID:       "implementer",
	}
	// The review chain's separator is `&&`, not `;`: a merge that refuses
	// leaves `-o` untouched, so the record step must not be reached.
	const reviewRecord = "[ -f $TP_ROUND_DIR/merged.ndjson ] || " +
		"tp review --merge $TP_ROUND_DIR/role-*.ndjson -o $TP_ROUND_DIR/merged.ndjson && " +
		"tp review spec/0.35.0.md --record $TP_ROUND_DIR/merged.ndjson"
	const auditRecord = "[ -f $TP_ROUND_DIR/merged.ndjson ] || " +
		"tp audit --merge $TP_ROUND_DIR/role-*.ndjson -o $TP_ROUND_DIR/merged.ndjson; " +
		"tp audit spec/0.35.0.md --record $TP_ROUND_DIR/merged.ndjson"

	want := map[UnitKind]string{
		UnitImplement:     "tp next --brief",
		UnitReviewRole:    "tp review spec/0.35.0.md --role implementer",
		UnitReviewRecord:  reviewRecord,
		UnitReviewResolve: "tp review spec/0.35.0.md --status",
		UnitDecompose:     "tp resume",
		UnitAuditRole:     "tp audit spec/0.35.0.md --role implementer",
		UnitAuditRecord:   auditRecord,
		UnitAuditFix:      "tp audit spec/0.35.0.md --status",
	}
	// Every one of the eight is covered, and nothing else is asserted about.
	require.Len(t, want, len(UnitKinds()))
	for _, k := range UnitKinds() {
		got := k.BriefCommand(target)
		assert.Equal(t, want[k], got, "%s brief_command", k)
		assert.NotEmpty(t, got, "%s has a brief", k)
	}

	// A kind outside the eight has no brief.
	assert.Empty(t, UnitKind("mystery").BriefCommand(target))
}

// TestUnitKind_BriefCommand_RecordMergeSkippedWhenMergedExists runs the record
// kinds' brief through /bin/sh against a fake tp and checks what it invoked:
// the merge step runs only when merged.ndjson is absent, so a re-run record
// unit never merges over the dispositions §6.3 accumulates in that file
// (test 57). Asserting on the shell's behaviour rather than on the string is
// the point — a guard that reads right but expands wrong would pass a string
// comparison.
func TestUnitKind_BriefCommand_RecordMergeSkippedWhenMergedExists(t *testing.T) {
	for _, tc := range []struct {
		kind UnitKind
		verb string
	}{
		{UnitReviewRecord, "review"},
		{UnitAuditRecord, "audit"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Run("merged absent: merges, then records", func(t *testing.T) {
				calls := runBrief(t, tc.kind, false)
				require.Len(t, calls, 2)
				assert.Contains(t, calls[0], "tp "+tc.verb+" --merge ")
				assert.Contains(t, calls[0], "-o ")
				assert.Contains(t, calls[1], "--record ")
			})
			t.Run("merged present: records only", func(t *testing.T) {
				calls := runBrief(t, tc.kind, true)
				require.Len(t, calls, 1, "the merge step is skipped")
				assert.Contains(t, calls[0], "--record ")
				assert.NotContains(t, calls[0], "--merge")
			})
		})
	}
}

// runBrief executes the kind's brief command with a fake `tp` first on PATH and
// returns one line per tp invocation. mergedExists seeds $TP_ROUND_DIR with the
// merged file the guard tests for.
func runBrief(t *testing.T, kind UnitKind, mergedExists bool) []string {
	t.Helper()
	dir := t.TempDir()
	roundDir := filepath.Join(dir, "round")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	// One role file, so the glob has something real to expand to.
	writeUnitFile(t, RoleFindingsPath(roundDir, "implementer"), "{}\n")
	if mergedExists {
		writeUnitFile(t, MergedFindingsPath(roundDir), `{"id":"f1","resolved":{"status":"fixed"}}`+"\n")
	}

	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	log := filepath.Join(dir, "calls.log")
	fake := "#!/bin/sh\necho \"$@\" >> " + log + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tp"), []byte(fake), 0o700))

	brief := kind.BriefCommand(UnitTarget{Spec: filepath.Join(dir, "spec.md"), RoundDir: roundDir})
	cmd := exec.Command("/bin/sh", "-c", brief)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TP_ROUND_DIR="+roundDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "brief exited non-zero: %s", out)

	data, err := os.ReadFile(log)
	require.NoError(t, err, "the brief invoked tp at least once")
	calls := make([]string, 0)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) != "" {
			calls = append(calls, "tp "+line)
		}
	}
	return calls
}

// TestUnitKind_BriefCommand_ReviewRecordFencesRecordBehindTheMerge runs both
// record briefs through /bin/sh against a fake tp that fails the merge step,
// and asserts on which commands the shell actually reached.
//
// Measured against the real binary before this test existed: a role file whose
// every content line is malformed makes `tp review --merge` exit 1 and write no
// `-o` at all (§5 row 10), and the `;` chain then ran `--record` on a file that
// is not there — exit 3, "cannot read findings file", reporting a missing path
// instead of the merge failure that caused it. The review chain is fenced with
// `&&` so the record step is not reached; the audit phase keeps `;`, because §4
// fences this release out of it and its merge still writes `-o` before refusing.
func TestUnitKind_BriefCommand_ReviewRecordFencesRecordBehindTheMerge(t *testing.T) {
	t.Run("review: a failed merge stops the chain", func(t *testing.T) {
		calls, exitCode := runBriefWithFailingMerge(t, UnitReviewRecord)
		require.Len(t, calls, 1, "the record step must not run after a failed merge")
		assert.Contains(t, calls[0], "--merge ")
		assert.NotEqual(t, 0, exitCode, "the chain reports the merge's own failure")
	})
	t.Run("audit: the chain continues past a failed merge", func(t *testing.T) {
		calls, _ := runBriefWithFailingMerge(t, UnitAuditRecord)
		require.Len(t, calls, 2, "the audit phase keeps ';' — §4 fences it out of this release")
		assert.Contains(t, calls[1], "--record ")
	})
}

// runBriefWithFailingMerge is runBrief with a fake tp that exits 1 on any
// invocation carrying --merge, and without the zero-exit requirement: the exit
// code is what the caller is measuring.
func runBriefWithFailingMerge(t *testing.T, kind UnitKind) (calls []string, exitCode int) {
	t.Helper()
	dir := t.TempDir()
	roundDir := filepath.Join(dir, "round")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	writeUnitFile(t, RoleFindingsPath(roundDir, "implementer"), "{}\n")

	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	log := filepath.Join(dir, "calls.log")
	fake := "#!/bin/sh\necho \"$@\" >> " + log + "\n" +
		"for a in \"$@\"; do [ \"$a\" = \"--merge\" ] && exit 1; done\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tp"), []byte(fake), 0o700))

	brief := kind.BriefCommand(UnitTarget{Spec: filepath.Join(dir, "spec.md"), RoundDir: roundDir})
	cmd := exec.Command("/bin/sh", "-c", brief)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TP_ROUND_DIR="+roundDir)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		exitCode = 0
	case errors.As(err, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		require.NoError(t, err, "the brief could not be run: %s", out)
	}

	data, err := os.ReadFile(log)
	require.NoError(t, err, "the brief invoked tp at least once")
	calls = make([]string, 0)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) != "" {
			calls = append(calls, "tp "+line)
		}
	}
	return calls, exitCode
}

// TestUnitKind_BriefCommand_RecordChainsDifferOnlyByVerbAndFence derives one
// record brief from the other rather than restating both, so the two cannot
// drift apart in any way except the two this release intends: the tp
// subcommand, and the separator that fences the record step behind the merge.
// A change to the shared shape — the `[ -f … ] ||` guard, the glob, the `-o`
// path — fails here whichever phase it lands in.
func TestUnitKind_BriefCommand_RecordChainsDifferOnlyByVerbAndFence(t *testing.T) {
	t.Parallel()
	target := UnitTarget{Spec: "spec/1.1.0.md", RoundDir: "/repo/rounds/review-r1"}
	audit := UnitAuditRecord.BriefCommand(target)
	review := UnitReviewRecord.BriefCommand(target)

	require.Contains(t, audit, "; tp audit ", "the audit chain is the unfenced one")
	derived := strings.Replace(strings.ReplaceAll(audit, "tp audit ", "tp review "), "; tp review ", " && tp review ", 1)
	assert.Equal(t, derived, review,
		"the review chain is the audit chain with its verb renamed and its record step fenced")
}

// TestUnitKind_Succeeded is test 51: a unit succeeded when it exited 0 AND its
// durable write is present. A zero exit with nothing written, and a non-zero
// exit over a complete write, are both failed attempts.
func TestUnitKind_Succeeded(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exitCode int
		written  bool
		want     bool
	}{
		{"exit 0 with the durable write present", 0, true, true},
		{"exit 0 with the durable write absent", 0, false, false},
		{"non-zero exit with the durable write present", 1, true, false},
		{"non-zero exit with the durable write absent", 2, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			status := "open"
			if tc.written {
				status = "done"
			}
			writeUnitFile(t, unitTaskFile(dir), taskFileJSON(status))
			target := UnitTarget{TaskFile: unitTaskFile(dir), ID: "t1"}

			assert.Equal(t, tc.written, UnitImplement.DurableWrite(target))
			assert.Equal(t, tc.want, UnitImplement.Succeeded(tc.exitCode, target))
		})
	}
}
