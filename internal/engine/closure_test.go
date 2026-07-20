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
