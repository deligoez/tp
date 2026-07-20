package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAcceptanceCriteria(t *testing.T) {
	tests := []struct {
		name       string
		acceptance string
		want       []string
	}{
		{
			name:       "period separated",
			acceptance: "A. B. C.",
			want:       []string{"A", "B", "C"},
		},
		{
			name:       "semicolon separated",
			acceptance: "A; B",
			want:       []string{"A", "B"},
		},
		{
			name:       "empty string",
			acceptance: "",
			want:       nil,
		},
		{
			name:       "single criterion",
			acceptance: "Single criterion",
			want:       []string{"Single criterion"},
		},
		{
			name:       "bullet list separated",
			acceptance: "- item1\n- item2\n- item3",
			want:       []string{"item1", "item2", "item3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAcceptanceCriteria(tt.acceptance)
			if tt.want == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestVerifyClosure(t *testing.T) {
	tests := []struct {
		name       string
		acceptance string
		reason     string
		coveredBy  bool
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "two criteria two evidence lines pass",
			acceptance: "User login works. Session persists.",
			reason:     "- login via JWT at internal/auth/login.go:42\n- session cookie persists across restart, tested",
			wantErr:    false,
		},
		{
			name:       "two criteria one evidence line fails",
			acceptance: "Database migration runs. API endpoint responds.",
			reason:     "- added the database migration script and endpoint",
			wantErr:    true,
			errMsg:     "2 criteria but reason has 1 evidence line",
		},
		{
			name:       "indented sub-bullets do not count",
			acceptance: "Model exists. Migration runs.",
			reason:     "- model added\n  - migration also added as sub-bullet",
			wantErr:    true,
			errMsg:     "2 criteria but reason has 1 evidence line",
		},
		{
			name:       "single criterion free text passes",
			acceptance: "The acceptance criteria must be thoroughly addressed with detailed evidence of completion",
			reason:     "did acceptance criteria",
			wantErr:    false,
		},
		{
			name:       "fail empty reason",
			acceptance: "Something.",
			reason:     "",
			wantErr:    true,
			errMsg:     "closure reason is required",
		},
		{
			name:       "forbidden pattern still rejected",
			acceptance: "Something works.",
			reason:     "deferred to next sprint",
			wantErr:    true,
			errMsg:     "deferral is forbidden",
		},
		{
			name:       "covered-by skips verification",
			acceptance: "A. B. C.",
			reason:     "covered by task other-task",
			coveredBy:  true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyClosure(tt.acceptance, tt.reason, tt.coveredBy)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClosureError_HintEnumeratesCriteria(t *testing.T) {
	err := VerifyClosure("Model exists. Migration runs. Tests pass.", "- only one line", false)
	require.Error(t, err)

	hint := ClosureHint(err, "fallback")
	assert.Contains(t, hint, "(1) Model exists")
	assert.Contains(t, hint, "(2) Migration runs")
	assert.Contains(t, hint, "(3) Tests pass")

	assert.Equal(t, "fallback", ClosureHint(assert.AnError, "fallback"))
}

func TestCountEvidenceLines(t *testing.T) {
	assert.Equal(t, 0, CountEvidenceLines(""))
	assert.Equal(t, 2, CountEvidenceLines("- a\n- b"))
	assert.Equal(t, 1, CountEvidenceLines("- a\n  - indented\n\ttext"))
	assert.Equal(t, 0, CountEvidenceLines("-no space\n -leading space"))
}

func TestForbiddenPatterns(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "deferred is forbidden",
			reason:  "This is deferred to next sprint",
			wantErr: true,
			errMsg:  "deferral is forbidden",
		},
		{
			name:    "will be done later is forbidden",
			reason:  "This will be done later by the team",
			wantErr: true,
			errMsg:  "deferral is forbidden",
		},
		{
			name:    "single word is forbidden",
			reason:  "done",
			wantErr: true,
			errMsg:  "must address each acceptance criterion",
		},
		{
			name:    "covered by existing without path is forbidden",
			reason:  "covered by existing tests in the suite",
			wantErr: true,
			errMsg:  "claim requires evidence",
		},
		{
			name:    "covered by existing with path is OK",
			reason:  "covered by existing at src/foo.go line 42",
			wantErr: false,
		},
		{
			name:    "not needed without because is forbidden",
			reason:  "not needed for this release",
			wantErr: true,
			errMsg:  "explain why",
		},
		{
			name:    "not needed with because is OK",
			reason:  "not needed because the feature was removed from scope",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkForbiddenPatterns(tt.reason)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyClosure_EvidenceLines(t *testing.T) {
	acceptance := "Model exists. Migration runs. Tests pass."

	t.Run("3 criteria 3 column-0 lines passes", func(t *testing.T) {
		reason := "- Task model at internal/model/task.go:18\n- migration 0007 applied, schema verified\n- go test ./... green (312 tests)"
		assert.NoError(t, VerifyClosure(acceptance, reason, false))
	})

	t.Run("3 criteria 2 lines fails naming counts", func(t *testing.T) {
		err := VerifyClosure(acceptance, "- model added\n- migration applied and tests green", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "3 criteria")
		assert.Contains(t, err.Error(), "2 evidence line")
	})

	t.Run("indented lines do not count", func(t *testing.T) {
		reason := "- model added\n  - migration applied\n  - tests green"
		err := VerifyClosure(acceptance, reason, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 evidence line")
	})

	t.Run("1 criterion free text passes", func(t *testing.T) {
		assert.NoError(t, VerifyClosure("Single criterion here", "implemented and verified manually", false))
	})
}

func TestVerifyClosure_ForbiddenPatternsRetained(t *testing.T) {
	acceptance := "Something works."

	t.Run("deferred rejected", func(t *testing.T) {
		err := VerifyClosure(acceptance, "deferred to next sprint", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deferral is forbidden")
	})

	t.Run("single word rejected", func(t *testing.T) {
		err := VerifyClosure(acceptance, "done", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must address each acceptance criterion")
	})

	t.Run("covered by existing without path rejected", func(t *testing.T) {
		err := VerifyClosure(acceptance, "covered by existing tests", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "claim requires evidence")
	})
}

func TestVerifyClosure_KeywordMachineryGone(t *testing.T) {
	// Prose criteria in any language pass with a matching evidence-line
	// count — no keyword overlap between acceptance and reason required.
	t.Run("turkish prose criteria", func(t *testing.T) {
		acceptance := "Görev modeli mevcut. Migrasyon çalışıyor."
		reason := "- ilgili yapı eklendi ve dosyaya yazıldı\n- şema güncellemesi uygulandı, kontrol edildi"
		assert.NoError(t, VerifyClosure(acceptance, reason, false))
	})

	t.Run("no keyword overlap single criterion", func(t *testing.T) {
		acceptance := "The UserModel class persists to user_name field"
		assert.NoError(t, VerifyClosure(acceptance, "structural change verified end to end", false))
	})
}
