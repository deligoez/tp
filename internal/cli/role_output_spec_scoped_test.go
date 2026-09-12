package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoSpecProject seeds two specs and one affected file in a single directory —
// the shape an operator has whenever more than one spec is reviewed or audited
// by hand from the same working directory.
func twoSpecProject(t *testing.T) (dir, specA, specB string) {
	t.Helper()
	dir = t.TempDir()
	const head = "# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n"
	specA = filepath.Join(dir, "a.md")
	specB = filepath.Join(dir, "b.md")
	require.NoError(t, os.WriteFile(specA, []byte(head+"Spec A carries the first rule.\n"), 0o600))
	require.NoError(t, os.WriteFile(specB, []byte(head+"Spec B carries the second rule.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"),
		[]byte("package main\nfunc Foo() int { return 42 }\n"), 0o600))
	return dir, specA, specB
}

// outputPathByRole indexes an emission's output_path values by role id.
func outputPathByRole(t *testing.T, prompts []map[string]any) map[string]string {
	t.Helper()
	byRole := make(map[string]string, len(prompts))
	for _, pm := range prompts {
		byRole[pm["role"].(string)] = pm["output_path"].(string)
	}
	require.NotEmpty(t, byRole)
	return byRole
}

// noRoundDir clears TP_ROUND_DIR explicitly rather than leaving it unset: the
// child inherits the test process's environment, so an ambient value — a
// `go test` run from inside a run unit — would otherwise emit the round-scoped
// path and turn these arms red for a reason that is not the code.
func noRoundDir() []string { return []string{engine.EnvRoundDir + "="} }

// TestRoleOutputPathCarriesTheSpecBase is the silent-loss repair: outside a run
// the emitted role file is named per spec, `<phase>-<base>-r<N>-<role>.ndjson`,
// so two specs emitted in one directory never name one file. With the round and
// the role alone, `tp review a.md` and `tp review b.md` both said
// `review-r1-<role>.ndjson` and the second panel's findings overwrote the
// first's with no error anywhere.
func TestRoleOutputPathCarriesTheSpecBase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		phase string
		args  func(spec string) []string
	}{
		{"review", func(s string) []string { return []string{"review", s, "--no-state"} }},
		{"audit", func(s string) []string { return []string{"audit", s, "--affected-files", "code.go"} }},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			t.Parallel()
			dir, specA, specB := twoSpecProject(t)
			gotA := outputPathByRole(t, emittedPrompts(t, dir, noRoundDir(), tc.args(specA)...))
			gotB := outputPathByRole(t, emittedPrompts(t, dir, noRoundDir(), tc.args(specB)...))

			for role, path := range gotA {
				assert.Equal(t, tc.phase+"-a-r1-"+role+".ndjson", path,
					"the emitted name carries the spec's base, matching ground's scratch name")
			}
			for role, path := range gotB {
				assert.Equal(t, tc.phase+"-b-r1-"+role+".ndjson", path)
			}

			shared := 0
			for role, path := range gotA {
				other, ok := gotB[role]
				if !ok {
					continue
				}
				shared++
				assert.NotEqual(t, path, other,
					"role %s: two specs emitted in one directory must not name one file", role)
			}
			assert.Positive(t, shared,
				"the two panels share at least one role, or the collision could not have happened")
		})
	}
}

// TestRoleOutputPathIsNamedInTheTwoPromptTexts is the same property one level
// down: §10.4's line in the prompt body names the same per-spec file, and never
// the other spec's. A role told the wrong filename writes over another spec's
// findings however correct the output_path field is.
func TestRoleOutputPathIsNamedInTheTwoPromptTexts(t *testing.T) {
	t.Parallel()
	dir, specA, specB := twoSpecProject(t)
	for _, tc := range []struct {
		phase string
		args  func(spec string) []string
	}{
		{"review", func(s string) []string { return []string{"review", s, "--no-state"} }},
		{"audit", func(s string) []string { return []string{"audit", s, "--affected-files", "code.go"} }},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			promptA := emittedPrompts(t, dir, noRoundDir(), tc.args(specA)...)
			promptB := emittedPrompts(t, dir, noRoundDir(), tc.args(specB)...)
			pathsB := outputPathByRole(t, promptB)
			for _, pm := range promptA {
				role := pm["role"].(string)
				body := pm["prompt"].(string)
				assert.Contains(t, body, "Write this round's findings to: "+pm["output_path"].(string),
					"role %s: §10.4's line names this spec's file", role)
				if other, ok := pathsB[role]; ok {
					assert.NotContains(t, body, other,
						"role %s: a.md's prompt must never name b.md's findings file", role)
				}
			}
		})
	}
}

// TestSpecScopedRoleFilesRecordEndToEnd walks the whole hand-run loop for two
// specs in one directory — emit, write each role's file at the path the prompt
// named, merge those files, record the round — and checks that each spec's
// recorded round holds its own findings. Under the round-and-role-only name the
// second emission's roles overwrote the first's files, so a.md's recorded round
// carried b.md's rows: loss with no error anywhere in the chain.
func TestSpecScopedRoleFilesRecordEndToEnd(t *testing.T) {
	t.Parallel()
	dir, specA, specB := twoSpecProject(t)
	specs := []string{specA, specB}

	written := make(map[string][]string, len(specs))
	for _, spec := range specs {
		base := engine.SpecBaseName(spec)
		for _, pm := range emittedPrompts(t, dir, noRoundDir(), "review", spec) {
			path := pm["output_path"].(string)
			require.False(t, filepath.IsAbs(path), "outside a run the path is relative to the working directory")
			full := filepath.Join(dir, path)
			require.NoFileExists(t, full,
				"%s: another spec's role already wrote this file — that write is the silent loss", path)
			row := `{"evidence":"read the cited section","severity":"high","category":"c",` +
				`"location":"` + base + `.md:1","finding":"finding from ` + base + `","suggestion":"s"}` + "\n"
			require.NoError(t, os.WriteFile(full, []byte(row), 0o600))
			written[spec] = append(written[spec], path)
		}
		require.NotEmpty(t, written[spec])
	}

	for _, spec := range specs {
		base := engine.SpecBaseName(spec)
		merged := base + "-merged.ndjson"
		args := append([]string{"review", "--merge"}, written[spec]...)
		_, stderr, code := runTPEnv(t, dir, noRoundDir(), append(args, "-o", merged)...)
		require.Equal(t, 0, code, "merge %s: %s", base, stderr)

		_, stderr, code = runTPEnv(t, dir, noRoundDir(), "review", spec, "--record", merged)
		require.Equal(t, 0, code, "record %s: %s", base, stderr)

		body, err := os.ReadFile(filepath.Join(dir, ".tp-review", base, "review-round-1.ndjson"))
		require.NoError(t, err)
		assert.Contains(t, string(body), "finding from "+base,
			"%s's recorded round holds the findings its own roles wrote", base)
		for _, otherSpec := range specs {
			if other := engine.SpecBaseName(otherSpec); other != base {
				assert.NotContains(t, string(body), "finding from "+other,
					"%s's recorded round must not hold %s's findings", base, other)
			}
		}
	}
}

// TestRoleBriefCommandNamesTheFileThePromptNames runs the brief tp hands a
// role unit, argv for argv, and checks the prompt it emits names the per-spec
// file. The brief and the prompt are built in different packages, so a name
// that moved in one and not the other would send the unit to a file nothing
// merges.
func TestRoleBriefCommandNamesTheFileThePromptNames(t *testing.T) {
	t.Parallel()
	dir, specA, specB := twoSpecProject(t)
	// `tp audit <spec> --role <id>` carries no --affected-files, so it takes
	// its file selection from git the way it does under a run: a repo with one
	// uncommitted change is the smallest fixture that gives it one.
	initGitRepo(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"),
		[]byte("package main\nfunc Foo() int { return 43 }\n"), 0o600))
	for _, tc := range []struct {
		phase string
		kind  engine.UnitKind
	}{
		{"review", engine.UnitReviewRole},
		{"audit", engine.UnitAuditRole},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			for _, spec := range []string{specA, specB} {
				base := engine.SpecBaseName(spec)
				// The role is taken from the panel this spec emits rather
				// than named here, so the brief is built for a role the
				// phase really has.
				role := emittedPrompts(t, dir, noRoundDir(), tc.phase, spec)[0]["role"].(string)
				brief := tc.kind.BriefCommand(engine.UnitTarget{Spec: spec, ID: role})
				argv := strings.Fields(brief)
				require.Equal(t, "tp", argv[0], "the brief invokes tp: %s", brief)

				prompts := emittedPrompts(t, dir, noRoundDir(), argv[1:]...)
				require.Len(t, prompts, 1, "--role emits that role alone")
				assert.Equal(t, tc.phase+"-"+base+"-r1-"+role+".ndjson", prompts[0]["output_path"].(string),
					"the brief's own emission names this spec's file")
			}
		})
	}
}
