package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// epicGrantFile names the file in a slice's state directory that records the
// operator's grant of one round past review_max_rounds (spec/1.7.0.md §8).
const epicGrantFile = "grant.json"

// epicGrant is the grant file: the one round it lets the slice emit and
// record. The round is the one after the last recorded round at grant time,
// so recording it spends the grant without the file being removed.
type epicGrant struct {
	Round int `json:"round"`
}

// WriteEpicReviewGrant records the grant of round in stateDir under the
// slice's lock, through an atomic rename.
func WriteEpicReviewGrant(stateDir string, round int) error {
	data, err := json.MarshalIndent(epicGrant{Round: round}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return WithFileLock(stateDir, func() error {
		return writeFileAtomic(filepath.Join(stateDir, epicGrantFile), append(data, '\n'))
	})
}

// readEpicReviewGrant returns the round stateDir's grant names, 0 when the
// slice has no grant file.
func readEpicReviewGrant(stateDir string) (int, error) {
	path := filepath.Join(stateDir, epicGrantFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var g epicGrant
	if err := json.Unmarshal(data, &g); err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	return g.Round, nil
}
