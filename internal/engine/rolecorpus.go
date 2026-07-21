package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// LoadRoleCorpus loads the committed user role files for a phase
// (PhaseReviewers -> .tp/reviewers/, PhaseAuditors -> .tp/auditors/), anchored at
// the git-boundary .tp/ (§4.1-4.3, reusing the v0.24.0 anchor). The phase is
// inferred from the directory (§3.1). It returns the parsed roles sorted by file
// name and whether the phase directory is populated — it holds at least one
// .json file (§4.4). An absent or empty phase directory returns populated=false
// with no roles, so the caller falls back to the embedded default corpus.
// Non-JSON files are ignored. A malformed or invalid role file returns an error
// (the caller aborts that phase, §3.6).
func LoadRoleCorpus(startDir, phase string) (roles []model.Role, populated bool, err error) {
	tpDir := DiscoverTPDir(startDir)
	if tpDir == "" {
		return nil, false, nil
	}
	phaseDir := filepath.Join(tpDir, phase)
	entries, readErr := os.ReadDir(phaseDir)
	if readErr != nil {
		return nil, false, nil // absent phase directory: embedded default applies
	}

	jsonFiles := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) == 0 {
		return nil, false, nil // empty (or non-JSON-only) directory: embedded default applies
	}
	sort.Strings(jsonFiles)

	roles = make([]model.Role, 0, len(jsonFiles))
	for _, name := range jsonFiles {
		role, parseErr := ParseRoleFile(filepath.Join(phaseDir, name))
		if parseErr != nil {
			return nil, true, parseErr
		}
		roles = append(roles, role)
	}
	return roles, true, nil
}
