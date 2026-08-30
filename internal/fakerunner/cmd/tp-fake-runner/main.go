// Command tp-fake-runner is the executable TP_RUNNER_SEAM pins in tests
// (§3.2.1). It records the invocation it was spawned for — argv, environment,
// spawn time, exit time — writes the final spend line a real runner would, and
// exits with a scripted code.
//
// Its arguments are the seam's positional list, so it needs no flag parsing:
//
//	<unit_kind> <unit_id> <log_path> <max_budget_usd>
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/fakerunner"
)

// The seam's positional arguments, by index into os.Args.
const (
	argLogPath = 3
	argCount   = 5
)

// exitSetup is what the fake exits with when its own setup failed — a record
// directory it cannot claim in. It is deliberately not one of tp's exit codes:
// a test scripting exit 2 must be able to tell its scripted failure from the
// fake having never run at all.
const exitSetup = 97

func main() {
	os.Exit(run())
}

// run does the whole invocation and returns the code to exit with, so every
// path closes the record file rather than losing it to an os.Exit.
func run() int {
	spawnedAt := time.Now().UTC()

	dir := os.Getenv(fakerunner.EnvDir)
	if dir == "" {
		fmt.Fprintf(os.Stderr, "%s is unset: the fake runner records every invocation and has nowhere to write\n",
			fakerunner.EnvDir)
		return exitSetup
	}
	file, seq, err := fakerunner.Claim(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitSetup
	}
	defer file.Close()

	if ms := intEnv(fakerunner.EnvSleepMS); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	if err := writeSpendLine(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitSetup
	}
	if durableAt(os.Getenv(fakerunner.EnvDurable), seq) {
		if err := durableWrite(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitSetup
		}
	}

	code := scriptedExit(os.Getenv(fakerunner.EnvExits), seq)
	record := fakerunner.Invocation{
		Seq:       seq,
		Argv:      os.Args[1:],
		Env:       environ(),
		SpawnedAt: spawnedAt,
		ExitedAt:  time.Now().UTC(),
		ExitCode:  code,
	}
	if err := json.NewEncoder(file).Encode(record); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitSetup
	}
	return code
}

// environ returns the child's whole environment as a map. An entry without "="
// is not one and is dropped, matching how the driver builds it (§3.2.1).
func environ() map[string]string {
	entries := os.Environ()
	env := make(map[string]string, len(entries))
	for _, entry := range entries {
		if key, value, ok := strings.Cut(entry, "="); ok {
			env[key] = value
		}
	}
	return env
}

// writeSpendLine appends the final log line §3.2.1 says the fake writes: one
// JSON object carrying the seam's spend_key, which is what makes the spend and
// budget-cap paths testable without an agent.
//
// The log path is the seam's third positional argument; an empty one means the
// caller does not care about the log, so nothing is written. The parents are
// created here so a test can name a path under a fresh temporary directory
// without first building the tree the driver would have built.
func writeSpendLine() error {
	if len(os.Args) < argCount || os.Args[argLogPath] == "" {
		return nil
	}
	path := os.Args[argLogPath]
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("log directory: %w", err)
	}
	line, err := json.Marshal(map[string]float64{"total_cost_usd": floatEnv(fakerunner.EnvSpend)})
	if err != nil {
		return fmt.Errorf("spend line: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("log file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write spend line: %w", err)
	}
	return nil
}

// durableWrite performs the unit kind's §3.3 durable write, which is what lets
// a test drive the cycle forward a phase instead of watching every unit fail.
//
// It reads the unit's identity from the child environment the driver exported
// (§3.1.1) rather than from its own arguments, because that environment is
// exactly what a real agent would work from. Only the kinds a loop test needs
// are covered; anything else writes nothing and the driver's success test
// reports the unit as unfinished, which is the honest answer.
func durableWrite() error {
	switch os.Getenv("TP_UNIT_KIND") {
	case "implement":
		return markTaskDone(os.Getenv("TP_FILE"), os.Getenv("TP_UNIT_ID"))
	case "review-role", "audit-role":
		return writeRoleFindings(os.Getenv("TP_ROUND_DIR"), os.Getenv("TP_UNIT_ID"))
	}
	return nil
}

// markTaskDone flips one task's status to done in the task file, the implement
// kind's durable write. The document is edited as generic JSON so the fixture
// carries no opinion about tp's own schema beyond the one field the predicate
// reads.
func markTaskDone(taskFile, id string) error {
	data, err := os.ReadFile(taskFile) //nolint:gosec // a test fixture reading the path its driver exported
	if err != nil {
		return fmt.Errorf("read task file: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse task file: %w", err)
	}
	tasks, _ := doc["tasks"].([]any)
	for _, entry := range tasks {
		task, ok := entry.(map[string]any)
		if !ok || task["id"] != id {
			continue
		}
		task["status"] = "done"
		task["closed_at"] = time.Now().UTC().Format(time.RFC3339)
		task["closed_reason"] = "- closed by the fake runner"
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("render task file: %w", err)
	}
	if err := os.WriteFile(taskFile, append(out, '\n'), 0o600); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}
	return nil
}

// writeRoleFindings writes the role kinds' durable write: one parseable line
// into role-<id>.ndjson.part. The .part suffix is deliberate — §3.3.1 makes
// the driver's rename to the final name what completes the unit, so a fixture
// that wrote the final name directly would hide a driver that never renames.
func writeRoleFindings(roundDir, roleID string) error {
	if roundDir == "" {
		return errors.New("TP_ROUND_DIR is unset: a role unit has nowhere to write its findings")
	}
	if err := os.MkdirAll(roundDir, 0o750); err != nil {
		return fmt.Errorf("round directory: %w", err)
	}
	line, err := json.Marshal(map[string]string{"role": roleID, "status": "PASS"})
	if err != nil {
		return fmt.Errorf("findings line: %w", err)
	}
	path := filepath.Join(roundDir, "role-"+roleID+".ndjson.part")
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		return fmt.Errorf("write findings: %w", err)
	}
	return nil
}

// scriptedExit returns the code this invocation exits with: the seq-th entry of
// the scripted list, or success once the list runs out. An entry that is not a
// number is success too — the seam is a test fixture, and a typo in it should
// fail the assertion the test actually makes rather than the fake's own parse.
func scriptedExit(scripted string, seq int) int {
	if scripted == "" {
		return 0
	}
	codes := strings.Split(scripted, ",")
	if seq >= len(codes) {
		return 0
	}
	code, err := strconv.Atoi(strings.TrimSpace(codes[seq]))
	if err != nil {
		return 0
	}
	return code
}

// durableAt reports whether this invocation performs its unit kind's durable
// write. The knob is a comma-separated list read by invocation order, and an
// invocation past its end repeats the last entry rather than defaulting: the
// plain "1" every caller writes therefore still means every invocation, while
// "1,0" makes the first invocation write and every later one leave the artifact
// absent. That asymmetry is what a retry test needs — an attempt that writes
// nothing after one that wrote a .part is the only arrangement in which a
// driver that fails to clear the leftover behaves differently from one that
// clears it.
func durableAt(scripted string, seq int) bool {
	if scripted == "" {
		return false
	}
	entries := strings.Split(scripted, ",")
	if seq >= len(entries) {
		seq = len(entries) - 1
	}
	return strings.TrimSpace(entries[seq]) == "1"
}

// intEnv reads an integer knob, treating absent and unparseable alike as 0.
func intEnv(name string) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return 0
	}
	return value
}

// floatEnv reads a decimal knob, treating absent and unparseable alike as 0.
func floatEnv(name string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(name)), 64)
	if err != nil {
		return 0
	}
	return value
}
