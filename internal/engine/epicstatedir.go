package engine

import (
	"path/filepath"
	"slices"
	"strings"
)

// EpicReviewStateRoot is the directory under .tp/ that holds one state
// directory per tp epic review slice (spec/1.3.0.md §5).
const EpicReviewStateRoot = "epic-review"

// EpicReviewStateDir returns key's state directory under tpDir: its name is
// the lowercase hex SHA-256 of the slice key with its kind (D13), so that
// --milestone A and --epic A never share one and no id reaches the file
// system. Round files and snapshots of the slice live there.
func EpicReviewStateDir(tpDir string, key EpicSliceKey) string {
	return filepath.Join(tpDir, EpicReviewStateRoot, epicHash(epicSliceKeyText(key)))
}

// epicSliceKeyText renders key with its kind and a NUL separator, which no
// milestone or epic id can make ambiguous. A named set is keyed by its name,
// an unnamed one by its sorted, de-duplicated ids, under two different kinds.
func epicSliceKeyText(key EpicSliceKey) string {
	if key.Name != "" {
		return "epics-named\x00" + key.Name
	}
	if len(key.Epics) > 0 {
		ids := slices.Compact(slices.Sorted(slices.Values(key.Epics)))
		return "epics\x00" + strings.Join(ids, "\x00")
	}
	if key.Milestone != "" {
		return "milestone\x00" + key.Milestone
	}
	return "epic\x00" + key.Epic
}
