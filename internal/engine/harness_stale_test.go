package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHarnessStale covers the per-phase harness-note staleness comparator
// (§6.2/§6.3): stale is true ONLY when the two most recently recorded rounds
// both carry non-empty notes that differ, judged on the trimmed note.
func TestHarnessStale(t *testing.T) {
	t.Run("fewer than two rounds is false", func(t *testing.T) {
		assert.False(t, HarnessStale(nil))
		assert.False(t, HarnessStale([]ReviewRound{}))
		assert.False(t, HarnessStale([]ReviewRound{{HarnessNote: "a"}}))
	})

	t.Run("two non-empty differing notes is true", func(t *testing.T) {
		assert.True(t, HarnessStale([]ReviewRound{
			{HarnessNote: "round one"},
			{HarnessNote: "round two"},
		}))
	})

	t.Run("two identical notes is false", func(t *testing.T) {
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: "same"},
			{HarnessNote: "same"},
		}))
	})

	t.Run("empty latest note is a no-signal", func(t *testing.T) {
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: "a"},
			{HarnessNote: ""},
		}))
	})

	t.Run("empty previous note (opt-out or predate) is a no-signal", func(t *testing.T) {
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: ""},
			{HarnessNote: "b"},
		}))
	})

	t.Run("whitespace-only note is a no-signal", func(t *testing.T) {
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: "   \t\n"},
			{HarnessNote: "b"},
		}))
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: "a"},
			{HarnessNote: "  "},
		}))
	})

	t.Run("difference is judged trimmed", func(t *testing.T) {
		// Same text with different surrounding whitespace is NOT a difference.
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: "  note  "},
			{HarnessNote: "note"},
		}))
		// Genuinely different content, even padded, IS stale.
		assert.True(t, HarnessStale([]ReviewRound{
			{HarnessNote: " first "},
			{HarnessNote: " second "},
		}))
	})

	t.Run("compares only the two most recent rounds", func(t *testing.T) {
		// Earlier rounds are ignored; only the last two matter.
		assert.True(t, HarnessStale([]ReviewRound{
			{HarnessNote: "x"},
			{HarnessNote: "match"},
			{HarnessNote: "differs"},
		}))
		assert.False(t, HarnessStale([]ReviewRound{
			{HarnessNote: "differs"},
			{HarnessNote: "match"},
			{HarnessNote: "match"},
		}))
	})
}

// TestLatestHarnessNote returns the latest round's note verbatim (untrimmed),
// or "" when there are no rounds.
func TestLatestHarnessNote(t *testing.T) {
	assert.Equal(t, "", LatestHarnessNote(nil))
	assert.Equal(t, "  padded note  ", LatestHarnessNote([]ReviewRound{
		{HarnessNote: "earlier"},
		{HarnessNote: "  padded note  "},
	}), "the stored note is returned verbatim, not trimmed")
}
