package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopeFenceText_StatesEveryProhibition(t *testing.T) {
	got := ScopeFenceText()

	// §7.1: each prohibition the fence forbids, plus the out-of-scope escape
	// hatch the brief must name so a unit knows where to put a finding.
	for _, want := range []string{
		"acceptance criteria",
		"refactor",
		"rename",
		"reformat",
		`"clean up"`,
		"task file",
		".tp-review/",
		`"Out of scope:"`,
	} {
		assert.Contains(t, got, want)
	}
	assert.NotContains(t, got, "fixing it outside", "the fence forbids fixing, not only fixing outside")
}

func TestExtractOutOfScope(t *testing.T) {
	t.Run("present returns note without prefix", func(t *testing.T) {
		reason := "- model at internal/model/task.go:18\n- tests green\nOut of scope: typo in README.md"
		assert.Equal(t, "typo in README.md", ExtractOutOfScope(reason))
	})

	t.Run("absent returns empty", func(t *testing.T) {
		reason := "- model added\n- tests green"
		assert.Equal(t, "", ExtractOutOfScope(reason))
	})

	t.Run("empty reason returns empty", func(t *testing.T) {
		assert.Equal(t, "", ExtractOutOfScope(""))
	})

	t.Run("prefix only returns empty", func(t *testing.T) {
		assert.Equal(t, "", ExtractOutOfScope("- done\nOut of scope:"))
	})

	t.Run("leading spaces after prefix trimmed", func(t *testing.T) {
		assert.Equal(t, "note here", ExtractOutOfScope("Out of scope:   note here"))
	})

	t.Run("not matched when embedded mid line", func(t *testing.T) {
		// The line must BEGIN with the prefix; a mention inside another line
		// is not the out-of-scope escape hatch.
		reason := "- evidence line\n- saw Out of scope: inline in docs"
		assert.Equal(t, "", ExtractOutOfScope(reason))
	})
}

func TestCloseRecipeText_Builtin(t *testing.T) {
	got := CloseRecipeText(CommitStrategyBuiltin, "go test ./... && golangci-lint run")

	// §8.1: the builtin close command.
	assert.Contains(t, got, `tp done <id> --auto-commit -- "<evidence>"`)
	// §8.2: the gate verbatim and the red-gate rule.
	assert.Contains(t, got, "go test ./... && golangci-lint run")
	assert.Contains(t, got, "A red gate is never closed over")
	assert.Contains(t, got, "--skip-gate is a human decision")
	// §8.3: the evidence contract.
	assert.Contains(t, got, "One line per acceptance criterion")
	assert.Contains(t, got, "Written in English")
	assert.Contains(t, got, "first line becomes the next unit's summary")
	assert.Contains(t, got, `"Out of scope:"`)
	// §8.4: the "--" separator is named.
	assert.Contains(t, got, `"--" separator`)
	// Under builtin, --auto-commit is the path, not a rejection, and the hc
	// command/rejected note are absent.
	assert.NotContains(t, got, "are all rejected")
	assert.NotContains(t, got, "--commit <sha>")
}

func TestCloseRecipeText_HC(t *testing.T) {
	got := CloseRecipeText(CommitStrategyHC, "go test ./... && golangci-lint run")

	// §8.1: the hc close command.
	assert.Contains(t, got, "commit with hc")
	assert.Contains(t, got, `tp done <id> --commit <sha> -- "<evidence>"`)
	assert.Contains(t, got, "repeat --commit per sha")
	// §8.2/§8.3: the gate and evidence contract carry over.
	assert.Contains(t, got, "A red gate is never closed over")
	assert.Contains(t, got, "One line per acceptance criterion")
	// §8.4: the three rejected invocations under hc.
	assert.Contains(t, got, "are all rejected")
	assert.Contains(t, got, `bare "tp done"`)
	assert.Contains(t, got, `"tp commit"`)
	assert.Contains(t, got, `"--auto-commit"`)
	// The hc recipe does not offer --auto-commit as the close command.
	assert.NotContains(t, got, "--auto-commit --")
}

func TestCloseRecipeText_GateVerbatim(t *testing.T) {
	// Whatever the resolved gate string is, it appears verbatim in the recipe.
	got := CloseRecipeText(CommitStrategyBuiltin, "make check && ./bin/lint")
	assert.Contains(t, got, "make check && ./bin/lint")
}

func TestCloseRecipeText_EmptyGateRenderedHonestly(t *testing.T) {
	got := CloseRecipeText(CommitStrategyBuiltin, "")
	assert.Contains(t, got, "(none configured)")
}

func TestCloseRecipeText_AutoResolvesToMatchingRecipe(t *testing.T) {
	// auto → hc when hc is on PATH (§8.1).
	effHC, _ := EffectiveCommitStrategy(CommitStrategyAuto, true)
	assert.Equal(t, CommitStrategyHC, effHC)
	assert.Contains(t, CloseRecipeText(effHC, "go test"), "--commit <sha>")

	// auto → builtin when hc is absent (§8.1).
	effBuiltin, _ := EffectiveCommitStrategy(CommitStrategyAuto, false)
	assert.Equal(t, CommitStrategyBuiltin, effBuiltin)
	assert.Contains(t, CloseRecipeText(effBuiltin, "go test"), "--auto-commit")
}
