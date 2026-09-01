package cli_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// divergentAuditRepo builds a git working tree in which the AUDIT unit set and
// the audit emitted set diverge, which is the case §6.2 property 10 is written
// for.
//
// The divergence is produced, not asserted into existence: the only dirty file
// is under testdata/, which the audit universe filter drops, so every role but
// spec-coverage is skipped `no-checklist-items` while still being spawned as a
// unit. §4.2.1's table calls that row out in bold — it drops a prompt and does
// not drop a unit — and it is the row that would make a unit's own first
// command fail under an exit-2 rule.
//
// The tree is left DIRTY on purpose. `tp audit <spec>` on a clean tree exits 4
// with "no changed files detected", so a brief run against a clean tree would
// measure that refusal instead of the property. An audit-role unit runs after
// implementation, when the tree is not clean; the fixture reproduces that.
func divergentAuditRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "testdata"), 0o750))
	fixture := filepath.Join(dir, "testdata", "fixture.go")
	require.NoError(t, os.WriteFile(fixture, []byte("package main\n"), 0o600))
	initGitRepo(t, dir)

	// One unstaged change, under a path the universe filter drops.
	require.NoError(t, os.WriteFile(fixture, []byte("package main\n\nfunc F() {}\n"), 0o600))
	return dir
}

// skippedReasons reads skipped_roles from an emission in dir.
func skippedReasons(t *testing.T, dir string, args ...string) map[string]string {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "emission failed: %s", stderr)

	var out struct {
		Skipped []struct {
			Role   string `json:"role"`
			Reason string `json:"reason"`
		} `json:"skipped_roles"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	got := map[string]string{}
	for _, s := range out.Skipped {
		got[s.Role] = s.Reason
	}
	return got
}

// TestEveryUnitBriefCommandRunsInItsOwnPhase is §6.2 property 10.
//
// Two halves, and the second is the one that has teeth: every role unit's
// brief_command names that unit's own id, AND running it exits 0 in the phase
// that unit belongs to -- including for a role the emission skips.
func TestEveryUnitBriefCommandRunsInItsOwnPhase(t *testing.T) {
	dir := divergentAuditRepo(t)
	spec := filepath.Join(dir, "spec.md")
	taskFile := filepath.Join(dir, "spec.tasks.json")

	// The fixture must actually be divergent, or the test measures the easy
	// case and reports the hard one.
	reasons := skippedReasons(t, dir, "audit", "spec.md")
	skippedByEmission := make([]string, 0, len(reasons))
	for role, reason := range reasons {
		if reason == engine.SkipNoChecklistItems {
			skippedByEmission = append(skippedByEmission, role)
		}
	}
	require.NotEmpty(t, skippedByEmission,
		"the fixture must skip at least one corpus role with no-checklist-items, or the divergence is not present")

	for _, phase := range []string{engine.PhaseReview, engine.PhaseAudit} {
		t.Run(phase, func(t *testing.T) {
			units, _ := engine.BuildNextUnits(dir, taskFile, spec, phase, nil, nil, nil)
			require.NotEmpty(t, units, "%s must spawn role units", phase)

			sawSkipped := false
			for _, u := range units {
				assert.Contains(t, u.BriefCommand, "--role "+u.ID,
					"%s: a unit's brief names its own id", u.ID)
				for _, name := range skippedByEmission {
					if u.ID == name && phase == engine.PhaseAudit {
						sawSkipped = true
					}
				}
			}

			// The briefs run CONCURRENTLY, because that is how they run: §4.1
			// marks role units concurrent and REFERENCE.md says sibling roles
			// are spawned in parallel. Running them one at a time made this
			// guard blind to the only code defect audit round 7 found — every
			// sibling writes the round's snapshot, and a shared temp path made
			// them race to rename it. Sequentially: 0 failures in 20. In
			// parallel, before the fix: 5 of 40 on review, 9 of 40 on audit.
			//
			// The arrangement is part of the property, not an implementation
			// detail of the test.
			type result struct {
				id   string
				out  string
				code int
			}
			results := make([]result, len(units))
			var wg sync.WaitGroup
			for i, u := range units {
				wg.Add(1)
				go func(i int, u engine.NextUnit) {
					defer wg.Done()
					out, code := runShellIn(t, dir, u.BriefCommand)
					results[i] = result{u.ID, out, code}
				}(i, u)
			}
			wg.Wait()

			for _, r := range results {
				assert.Equal(t, 0, r.code,
					"%s/%s: the brief must exit 0 in its own phase, run beside its siblings; got: %s",
					phase, r.id, truncate(r.out))
			}
			if phase == engine.PhaseAudit {
				assert.True(t, sawSkipped,
					"the divergent role must be spawned as a unit; that is the row §4.2.1 puts in bold")
			}
		})
	}
}

// runShellIn runs a brief command as written, through the freshly built binary.
//
// The command names `tp`, which need not be on PATH and need not be the version
// under test -- CLAUDE.md's rule is that self-development always runs the
// in-progress binary. Substituting the test's own binary for the leading token
// is what keeps this measuring the code being shipped.
func runShellIn(t *testing.T, dir, command string) (output string, exitCode int) {
	t.Helper()
	fields := strings.Fields(command)
	require.NotEmpty(t, fields)
	require.Equal(t, "tp", fields[0], "a brief command starts with tp")

	cmd := exec.Command(binaryPath, fields[1:]...) //nolint:gosec // arguments come from tp's own brief
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TP_HC=0")
	out, err := cmd.CombinedOutput()

	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err, "the brief command must run")
	}
	return string(out), code
}

func truncate(s string) string {
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
