package engine

import (
	"os"
	"path/filepath"
)

// RecordEpicReviewRound writes data as the slice's next round file under the
// state directory's flock and returns the round it recorded. The round is
// chosen inside the lock, so two concurrent records take two rounds, and the
// file lands through an atomic rename so a partial round is never on disk.
func RecordEpicReviewRound(stateDir string, data []byte) (int, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return 0, err
	}
	var round int
	err := WithFileLock(stateDir, func() error {
		round = EpicReviewNextRound(stateDir)
		return writeFileAtomic(filepath.Join(stateDir, epicRoundFile(round)), data)
	})
	return round, err
}
