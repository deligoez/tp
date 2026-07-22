package engine

import (
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// ClassifiedTree is the keep-list classification of a set of changed paths:
// Kept holds one {path, reason} per matched file (path is the matched file, not
// the glob), Changes holds the unmatched paths (the unexplained changes). Both
// preserve input order; the caller sorts (§4.5).
type ClassifiedTree struct {
	Changes []string
	Kept    []model.KeepEntry
}

// MatchKeepList reports whether path matches any keep-list entry, returning the
// reason of the FIRST matching entry in stored order — so a file covered by
// several patterns yields exactly one classification (§7.3). Matching is Go
// filepath.Match: * and ? do not cross /, character classes are supported, and
// there is no ** recursive wildcard. A malformed entry pattern surfaces as
// filepath.ErrBadPattern.
func MatchKeepList(entries []model.KeepEntry, path string) (matched bool, reason string, err error) {
	for _, e := range entries {
		ok, matchErr := filepath.Match(e.Path, path)
		if matchErr != nil {
			return false, "", matchErr
		}
		if ok {
			return true, e.Reason, nil
		}
	}
	return false, "", nil
}

// ClassifyPaths splits paths against the keep-list: a path matching an entry
// goes to Kept as {path, reason} (the winning entry's reason, §7.3), an
// unmatched path goes to Changes. Input order is preserved; the caller sorts. A
// malformed entry pattern surfaces as filepath.ErrBadPattern.
func ClassifyPaths(entries []model.KeepEntry, paths []string) (ClassifiedTree, error) {
	res := ClassifiedTree{Changes: []string{}, Kept: []model.KeepEntry{}}
	for _, p := range paths {
		matched, reason, err := MatchKeepList(entries, p)
		if err != nil {
			return ClassifiedTree{}, err
		}
		if matched {
			res.Kept = append(res.Kept, model.KeepEntry{Path: p, Reason: reason})
		} else {
			res.Changes = append(res.Changes, p)
		}
	}
	return res, nil
}
