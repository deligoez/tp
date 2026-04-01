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

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name      string
		criterion string
		check     func(t *testing.T, keywords []string)
	}{
		{
			name:      "removes stop words",
			criterion: "the user is a member",
			check: func(t *testing.T, keywords []string) {
				assert.NotContains(t, keywords, "the")
				assert.NotContains(t, keywords, "is")
				assert.NotContains(t, keywords, "a")
				assert.Contains(t, keywords, "user")
				assert.Contains(t, keywords, "member")
			},
		},
		{
			name:      "keeps file paths",
			criterion: "update app/Models/User.php",
			check: func(t *testing.T, keywords []string) {
				assert.Contains(t, keywords, "app/Models/User.php")
			},
		},
		{
			name:      "keeps CamelCase terms",
			criterion: "the UserModel class",
			check: func(t *testing.T, keywords []string) {
				assert.Contains(t, keywords, "UserModel")
			},
		},
		{
			name:      "keeps snake_case terms",
			criterion: "set user_name field",
			check: func(t *testing.T, keywords []string) {
				assert.Contains(t, keywords, "user_name")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords := ExtractKeywords(tt.criterion)
			require.NotNil(t, keywords)
			tt.check(t, keywords)
		})
	}
}

func TestVerifyClosure(t *testing.T) {
	tests := []struct {
		name       string
		acceptance string
		reason     string
		gatePassed bool
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "pass addresses all criteria",
			acceptance: "User login works. Session persists.",
			reason:     "Implemented user login with session persistence using JWT tokens",
			wantErr:    false,
		},
		{
			name:       "fail missing criterion keyword",
			acceptance: "Database migration runs. API endpoint responds.",
			reason:     "Added the database migration script",
			wantErr:    true,
			errMsg:     "does not address criterion",
		},
		{
			name:       "fail reason too short",
			acceptance: "The acceptance criteria must be thoroughly addressed with detailed evidence of completion.",
			reason:     "did acceptance criteria",
			wantErr:    true,
			errMsg:     "too short",
		},
		{
			name:       "fail empty reason",
			acceptance: "Something.",
			reason:     "",
			wantErr:    true,
			errMsg:     "closure reason is required",
		},
		{
			name:       "gate-passed skips keyword matching",
			acceptance: "Composer quality passes. All tests green.",
			reason:     "2559 tests pass, 0 failures, PHPStan level 8 clean",
			gatePassed: true,
			wantErr:    false,
		},
		{
			name:       "gate-passed still checks forbidden patterns",
			acceptance: "Something works.",
			reason:     "deferred to next sprint",
			gatePassed: true,
			wantErr:    true,
			errMsg:     "deferral is forbidden",
		},
		{
			name:       "gate-passed still checks minimum length",
			acceptance: "The acceptance criteria must be thoroughly addressed with detailed evidence.",
			reason:     "tests pass",
			gatePassed: true,
			wantErr:    true,
			errMsg:     "too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyClosure(tt.acceptance, tt.reason, tt.gatePassed)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
