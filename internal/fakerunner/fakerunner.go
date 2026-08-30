// Package fakerunner is the child TP_RUNNER_SEAM pins in tests (§3.2.1): a
// runner that spawns in milliseconds, records what it was given, and exits with
// a scripted code, so the unattended loop is testable without an agent.
//
// It is deliberately a real executable rather than an in-process double. The
// driver's contract with a runner is a process boundary — argv, environment,
// stdin, an exit code, a log file — and a double that skips the boundary cannot
// observe the two things the loop's own tests rest on: that a child received
// exactly the argv the template expanded to, and that two role children were
// alive at the same moment.
package fakerunner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The environment the fake runner itself reads. They are namespaced apart from
// tp's own TP_ variables (§3.1.1) so a recorded child environment can be
// asserted on without the fake's own knobs being mistaken for the driver's.
const (
	// EnvDir is the directory invocations are recorded in. It is required:
	// a fake runner that records nothing is indistinguishable from one that
	// was never spawned, which is the failure these tests exist to catch.
	EnvDir = "TP_FAKE_RUNNER_DIR"
	// EnvExits is a comma-separated list of exit codes, consumed in
	// invocation order. An invocation past the end of the list succeeds, so
	// a test scripts only the failures it cares about.
	EnvExits = "TP_FAKE_RUNNER_EXITS"
	// EnvSleepMS holds each invocation open for that many milliseconds,
	// which is how a test creates the overlap §10.1 test 5 asserts on.
	EnvSleepMS = "TP_FAKE_RUNNER_SLEEP_MS"
	// EnvSpend is the dollar amount the final log line reports under the
	// seam's spend_key. It defaults to 0 rather than to absent, because a
	// runner that declares a spend_key and reports nothing is its own case
	// (§3.2.2) and should be reached on purpose.
	EnvSpend = "TP_FAKE_RUNNER_SPEND"
	// EnvDurable, set to "1", makes the invocation perform its unit kind's
	// §3.3 durable write before it exits: an implement unit marks its task
	// done, a role unit writes the role-<id>.ndjson.part the driver renames.
	// It is off by default so a test that wants a failed attempt — exit 0
	// with nothing written — gets one without scripting an exit code, and on
	// when a test needs the cycle to actually advance a phase.
	//
	// Like EnvExits it is a comma-separated list read by invocation order,
	// except that an invocation past its end repeats the last entry: "1"
	// therefore still means every invocation, and "1,0" means the first one
	// writes and every later one does not — the arrangement a retry test
	// needs to tell a cleared leftover from a promoted one.
	EnvDurable = "TP_FAKE_RUNNER_DURABLE"
)

// Invocation is one spawn of the fake runner, as it recorded itself.
//
// The two timestamps are the child's own rather than the driver's: what a test
// needs to know is when the process was alive, and a driver that mis-sequences
// its own bookkeeping would otherwise be marking its own homework.
type Invocation struct {
	// Seq is the invocation's order, counted from 0 across one record
	// directory. It is claimed before the child does anything else, so it
	// orders concurrent invocations by spawn rather than by exit.
	Seq int `json:"seq"`
	// Argv is the child's arguments, without the executable itself.
	Argv []string `json:"argv"`
	// Env is the child's whole environment.
	Env map[string]string `json:"env"`
	// SpawnedAt is the first thing the child records; ExitedAt the last.
	SpawnedAt time.Time `json:"spawned_at"`
	ExitedAt  time.Time `json:"exited_at"`
	// ExitCode is the code the child exited with, so a record and a waited-on
	// process can be checked against each other.
	ExitCode int `json:"exit_code"`
}

// The record file's name is <prefix><seq><suffix>, zero-padded so a directory
// listing is already in invocation order.
const (
	recordPrefix = "invocation-"
	recordSuffix = ".json"
	recordDigits = 4
	// maxInvocations bounds the claim scan. A run that spawns more children
	// than this into one directory is a runaway loop, and failing is a better
	// answer than scanning forever.
	maxInvocations = 10000
)

// RecordName returns the file name one invocation records itself in.
func RecordName(seq int) string {
	return fmt.Sprintf("%s%0*d%s", recordPrefix, recordDigits, seq, recordSuffix)
}

// Claim takes the next free sequence number in dir and returns the file the
// invocation will record itself in, still open for writing.
//
// The claim is the exclusive creation of the file itself, which is what makes
// it safe for children spawned at the same instant: two processes cannot both
// create the same name, so each walks on to the next. The file is left empty
// until the child exits, and Records skips an empty one — an invocation that
// has claimed its slot but not yet finished is not a completed record.
func Claim(dir string) (*os.File, int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, 0, fmt.Errorf("record directory: %w", err)
	}
	for seq := range maxInvocations {
		file, err := os.OpenFile(filepath.Join(dir, RecordName(seq)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return file, seq, nil
		}
		if !os.IsExist(err) {
			return nil, 0, fmt.Errorf("claim invocation %d: %w", seq, err)
		}
	}
	return nil, 0, fmt.Errorf("more than %d invocations recorded in %s", maxInvocations, dir)
}

// Records returns the completed invocations recorded in dir, in sequence order.
//
// A claimed-but-unfinished record is skipped rather than reported: a test that
// reads while a child is still alive is asking what has finished. A record that
// exists and does not parse is an error, because that is the fake runner itself
// having failed, and silently dropping it would read as "the child never ran".
func Records(dir string) ([]Invocation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}

	records := make([]Invocation, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, recordPrefix) || !strings.HasSuffix(name, recordSuffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read record %s: %w", name, err)
		}
		if len(data) == 0 {
			continue
		}
		var rec Invocation
		if err := json.Unmarshal(data, &rec); err != nil {
			return nil, fmt.Errorf("read record %s: %w", name, err)
		}
		records = append(records, rec)
	}
	slices.SortFunc(records, func(a, b Invocation) int { return a.Seq - b.Seq })
	return records, nil
}

// fakeRunnerPkg is the fake runner's own package path, built rather than
// vendored as a script: a Go toolchain is the one thing a Go test can count on
// having, and a shell script cannot read a clock finely enough to prove two
// children overlapped.
const fakeRunnerPkg = "github.com/deligoez/tp/internal/fakerunner/cmd/tp-fake-runner"

// Build compiles the fake runner into dir and returns the binary's path. dir is
// the caller's to clean up — t.TempDir() is the intended argument.
func Build(dir string) (string, error) {
	bin := filepath.Join(dir, "tp-fake-runner")
	out, err := exec.Command("go", "build", "-o", bin, fakeRunnerPkg).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build fake runner: %w: %s", err, out)
	}
	return bin, nil
}
