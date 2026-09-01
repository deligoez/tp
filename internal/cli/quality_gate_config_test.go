package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoQualityGate is the literal gate every task file in this repo must resolve
// to (v0.34.0 §5, extended with the deadcode and complexity steps). It is
// spelled out here rather than read from .tp/config.json on purpose: comparing
// the resolved value against the project value passes when the project gate
// itself weakens, and a substring check for "-race" passes when the rest of the
// gate changes.
//
// The third step closes a hole the first two cannot see: golangci-lint's
// `unused` skips exported identifiers by design, so an exported function with
// no callers passes both. Dropping it would restore that blindness silently,
// which is why it belongs in the literal rather than in prose somewhere.
//
// The fourth is a ratchet rather than a threshold: scripts/check-complexity.sh
// fails on a cognitive-complexity violation absent from its committed baseline,
// and equally on a baseline entry that no longer violates — so the baseline can
// only shrink. A plain gocognit linter entry would have to fail the 54 existing
// functions on day one, or sit at a threshold high enough to measure nothing.
const repoQualityGate = "./scripts/check-suite-state.sh && golangci-lint run && ./scripts/check-deadcode.sh && ./scripts/check-complexity.sh"

// TestRepoTaskFilesResolveRaceQualityGate guards the v0.34.0 §5 outcome: the
// gate resolved for every task file in this repo — this release's included —
// runs the race detector. Two ways to lose that are checked separately:
//
//   - the resolved value stops being the literal gate, which covers the project
//     gate in .tp/config.json weakening (e.g. -race dropped) as well as a
//     task-level override reintroducing the pre-race gate;
//   - a task file carries a quality_gate override at all, which masks the
//     project layer even while it happens to agree with it — the shape
//     `tp init --quality-gate "…"` used to create on every new task file.
func TestRepoTaskFilesResolveRaceQualityGate(t *testing.T) {
	root := repoRoot(t)
	taskFiles, err := filepath.Glob(filepath.Join(root, "spec", "*.tasks.json"))
	require.NoError(t, err)
	require.NotEmpty(t, taskFiles, "spec/*.tasks.json must exist at the repo root")

	for _, path := range taskFiles {
		rel, relErr := filepath.Rel(root, path)
		require.NoError(t, relErr)

		override, loadErr := engine.LoadTaskWorkflowOverride(path)
		require.NoError(t, loadErr, "%s must parse", rel)
		assert.Nil(t, override.QualityGate,
			"%s must not carry a task-level quality_gate: it masks the project gate", rel)

		workflow, _, resolveErr := engine.ResolveEffectiveWorkflow(root, &override)
		require.NoError(t, resolveErr, "%s must resolve against the project config", rel)
		assert.Equal(t, repoQualityGate, workflow.QualityGate,
			"the gate resolved for %s must be the literal repo gate", rel)
	}
}

// wrapperArgv runs scripts/check-suite-state.sh with a stub `go` first on PATH
// and returns the arguments the wrapper actually handed it.
//
// Every text-based version of this guard has been falsified, three times, each
// by an input its author did not build. The last one scanned the script for a
// `go test … ./...` line and kept the LAST match, with no notion of the if/else
// it sat inside: audit round 12 inverted the comparator, gave both branches an
// explicit ./..., and watched the race detector vanish from the branch that
// runs while three guards stayed green. Reading the script cannot answer which
// branch executes. Running it can, so this runs it.
func wrapperArgv(t *testing.T, args ...string) string {
	t.Helper()
	root := repoRoot(t)
	bin := t.TempDir()
	record := filepath.Join(bin, "argv")

	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + record + "\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o755)) //nolint:gosec // a stub the test executes itself

	cmd := exec.Command(filepath.Join(root, "scripts", "check-suite-state.sh"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "the wrapper must run under a stub go: %s", out)

	argv, readErr := os.ReadFile(record) //nolint:gosec // a path this test built
	require.NoError(t, readErr, "the wrapper never invoked go at all")
	return string(argv)
}

// TestTheWrapperActuallyInvokesTheSuiteItClaims measures the wrapper's own
// no-argument branch by executing it, not by reading it.
//
// Two flags and a scope, and each fails in a different direction. Without
// -race the repo gate stops running the race detector. Without -count=1
// `go test` serves a cached PASS for a package it did not run, so the
// before/after digest brackets nothing (measured in audit round 8: caught on
// run 1, "(cached)" and exit 0 on runs 2 and 3, mutation present each time).
// Without ./... it is not the suite.
func TestTheWrapperActuallyInvokesTheSuiteItClaims(t *testing.T) {
	argv := wrapperArgv(t)
	for _, want := range []string{"test", "-count=1", "-race", "./..."} {
		assert.Contains(t, argv, want,
			"the wrapper's no-argument branch invoked go with %q; it must carry %s", argv, want)
	}
}

// TestTheWrapperNarrowsWithoutLosingTheCache checks the other branch, which no
// guard reached before: a caller narrowing the run must still defeat the test
// cache, or the digest brackets nothing there too.
func TestTheWrapperNarrowsWithoutLosingTheCache(t *testing.T) {
	argv := wrapperArgv(t, "./internal/model/")
	assert.Contains(t, argv, "-count=1",
		"the narrowed branch invoked go with %q; -count=1 is what makes the digest mean anything", argv)
	assert.Contains(t, argv, "./internal/model/",
		"the narrowed branch must pass the caller's arguments through: %q", argv)
}

// TestCIQuotesTheWrapperCommandItActuallyRuns pins ONE pairing: the command
// ci.yml's comment says the wrapper RUNS must be a command the wrapper really
// invokes. The two tests above own the behaviour; this one owns the prose, so
// that a correct wrapper cannot be described by a wrong comment.
//
// It compares against the executed argv rather than the script's text, because
// every text-to-text version of this has been falsified. Deliberately outside
// it, named rather than implied: CLAUDE.md quotes this command too, and a
// comment elsewhere in ci.yml mentioning a command historically rather than
// assertively is not the subject.
func TestCIQuotesTheWrapperCommandItActuallyRuns(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml")) //nolint:gosec // a fixed path inside the repo under test
	require.NoError(t, err)

	claimed := regexp.MustCompile("RUNS `go test ([^`]*)`").FindAllStringSubmatch(string(workflow), -1)
	require.Len(t, claimed, 1,
		"ci.yml must say exactly once what the wrapper RUNS; without it this guard has no subject, and twice is two claims to keep true")

	argv := strings.TrimSpace(wrapperArgv(t))
	assert.Equal(t, "test "+claimed[0][1], argv,
		"ci.yml says the wrapper runs `go test %s`; it invoked go with %q", claimed[0][1], argv)
}

// TestSuiteStateWrapperStillRunsTheRaceDetector keeps the guard above able to
// see what it is named for.
//
// v0.36.0 §6.1 moved the suite behind scripts/check-suite-state.sh, so the gate
// literal no longer contains `-race` — the flag is inside the script now. A
// literal comparison alone would go on passing while the wrapper quietly
// dropped it, which is the exact failure mode v0.34.0 §5 added the guard for.
// Naming a thing is not the same as being able to observe it.
func TestSuiteStateWrapperStillRunsTheRaceDetector(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "check-suite-state.sh")
	body, err := os.ReadFile(script) //nolint:gosec // a fixed path inside the repo under test
	require.NoError(t, err, "the gate's first step must exist")

	// The assertion is over the script's `go test` INVOCATION LINES, not over
	// the file text, and that distinction was measured rather than assumed. A
	// bare assert.Contains(body, "-count=1") does not discriminate: the flag
	// appears in two branches and again in the comment explaining it, so
	// deleting it from one branch leaves the assertion true. That mutation was
	// run — dropping it from the ./... branch left a Contains-based guard GREEN
	// while dropping -race turned it red, which is the same defect this cycle
	// found eight times: an assertion whose falsifying input was never built.
	//
	// Both flags fail in opposite directions and every invocation needs both.
	// Without -race the repo gate stops running the race detector. Without
	// -count=1 `go test` serves a cached PASS for a package it did not run, so
	// the wrapper's before/after digest brackets nothing and reports green with
	// a live-state mutation still in the tree (audit round 8: caught on run 1,
	// "(cached)" and exit 0 on runs 2 and 3, mutation present each time).
	var invocations []string
	for _, line := range strings.Split(string(body), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "go test ") {
			invocations = append(invocations, trimmed)
		}
	}
	require.Len(t, invocations, 2,
		"the wrapper runs go test in exactly two branches — a narrowed run and the gate's own")
	for _, invocation := range invocations {
		assert.Contains(t, invocation, "-count=1",
			"every branch must defeat the test cache: %q", invocation)
	}
	assert.Contains(t, strings.Join(invocations, "\n"), "-race ./...",
		"the no-argument branch is where -race lives now; it must run the whole suite under it")

	info, statErr := os.Stat(script)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Mode().Perm()&0o111,
		"the gate invokes it as ./scripts/check-suite-state.sh, so it must be executable")
}
