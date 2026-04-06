package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuggestSimilarIDs(t *testing.T) {
	allIDs := []string{
		"linker-detect-impl",
		"linker-strategy",
		"user-auth-model",
		"auth-api",
		"create-tests",
		"setup-db",
	}

	tests := []struct {
		name     string
		givenID  string
		expected []string
	}{
		{
			"prefix match",
			"linker-detect-strategy",
			[]string{"linker-strategy", "linker-detect-impl"},
		},
		{
			"word overlap",
			"auth-model",
			[]string{"auth-api", "user-auth-model"},
		},
		{
			"no match",
			"zzz-unknown",
			nil,
		},
		{
			"top 3 limit",
			"linker",
			[]string{"linker-strategy", "linker-detect-impl"},
		},
		{
			"exact ID excluded",
			"auth-api",
			[]string{"user-auth-model"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SuggestSimilarIDs(tt.givenID, allIDs)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSplitWords(t *testing.T) {
	assert.Equal(t, []string{"auth", "model"}, splitWords("auth-model"))
	assert.Equal(t, []string{"linker", "detect", "impl"}, splitWords("linker-detect-impl"))
	// Short words (<3 chars) filtered
	assert.Equal(t, []string{"auth"}, splitWords("a-auth-db"))
}
