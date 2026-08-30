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
