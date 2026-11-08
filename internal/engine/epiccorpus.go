package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// PhaseEpicReviewers is the graph-review role directory under .tp/
// (spec/1.3.0.md §5). It is a phase of its own: the task-spec reviewer and
// auditor corpora never read it, and it never reads theirs.
const PhaseEpicReviewers = "epic-reviewers"

// epicCorpusOrder is the §5 built-in graph role table, in emitted panel order.
var epicCorpusOrder = []string{"dod-satisfiability", "why-auditor", "phase-economist"}

// DefaultEpicCorpusFiles returns the built-in graph role files in panel order,
// each with its raw JSON bytes so eject writes byte-identical copies.
func DefaultEpicCorpusFiles() ([]DefaultRoleFile, error) {
	out := make([]DefaultRoleFile, 0, len(epicCorpusOrder))
	for _, id := range epicCorpusOrder {
		path := "corpus/graph/" + PhaseEpicReviewers + "/" + id + ".json"
		data, err := defaultCorpusFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("embedded corpus missing %s: %w", path, err)
		}
		out = append(out, DefaultRoleFile{ID: id, Data: data})
	}
	return out, nil
}

// ResolveEpicRoleCorpus returns the active graph-review roles. A populated
// .tp/epic-reviewers/ replaces the built-ins outright: every file is an active
// role, sorted by file name, and `domains` is not read. With no such directory,
// or one holding no .json file, the three built-in roles run.
func ResolveEpicRoleCorpus(startDir string) ([]model.Role, error) {
	roles, populated, err := LoadRoleCorpus(startDir, PhaseEpicReviewers)
	if err != nil || populated {
		return roles, err
	}
	files, err := DefaultEpicCorpusFiles()
	if err != nil {
		return nil, err
	}
	roles = make([]model.Role, 0, len(files))
	for _, f := range files {
		role, parseErr := ParseRoleBytes(f.Data, f.ID)
		if parseErr != nil {
			return nil, fmt.Errorf("embedded graph role %s is invalid: %w", f.ID, parseErr)
		}
		roles = append(roles, role)
	}
	return roles, nil
}
