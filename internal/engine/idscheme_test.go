package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsLegacyRound_DetectionByMarker: a round's id scheme is detected by the
// stored id_scheme marker, never by parsing the id (§10.9). A marker-less round
// — recorded before this release — is legacy; a slug-marked round is not.
func TestIsLegacyRound_DetectionByMarker(t *testing.T) {
	assert.True(t, IsLegacyRound(&ReviewRound{}), "a marker-less round is legacy")
	assert.True(t, IsLegacyRound(&ReviewRound{IDScheme: ""}), "an empty marker is legacy")

	assert.False(t, IsLegacyRound(&ReviewRound{IDScheme: IDSchemeSlug}),
		"a slug-marked round recorded under this release is not legacy")
}
