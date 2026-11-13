package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// RoleInboxPath is the file one review or audit role writes outside a run:
// <spec dir>/.tp-review/<base>/<phase>-inbox/round-<N>-<role>.ndjson. It lives
// in the spec's own state directory so two same-base specs in different
// directories never share it, and in a subdirectory so an in-flight file is
// never mistaken for the recorded round beside it
// (spec/backlog/role-files-in-state-dir.md). Inside a run the role writes
// RoleFindingsPartPath instead.
func RoleInboxPath(specPath, phase string, round int, role string) string {
	return filepath.Join(roleInboxDir(specPath, phase), fmt.Sprintf("round-%d-%s.ndjson", round, role))
}

// roleInboxDir is a phase's inbox in the spec's state directory.
func roleInboxDir(specPath, phase string) string {
	return filepath.Join(ReviewStateDir(specPath), phase+"-inbox")
}

// legacyRoleFileRe is the pre-1.12.0 role file name,
// <phase>-<base>-r<N>-<role>.ndjson in the working directory.
var legacyRoleFileRe = regexp.MustCompile(`^(review|audit)-(.+)-r([1-9][0-9]*)-([a-z0-9]+(?:-[a-z0-9]+)*)\.ndjson$`)

// LegacyRoleFile reports whether path's name is a role file in the
// pre-1.12.0 shape, its role part naming a role tp recognises, and if so the inbox file it belongs at relative to its
// spec's directory: .tp-review/<base>/<phase>-inbox/round-<N>-<role>.ndjson.
// The old name is never read, since two same-base specs share it.
func LegacyRoleFile(path string) (target string, ok bool) {
	m := legacyRoleFileRe.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return "", false
	}
	round, err := strconv.Atoi(m[3])
	if err != nil {
		return "", false
	}
	// Only a role tp knows: an operator's own review-spec-r2-all.ndjson has
	// the shape and is no role file, so it is merged as before.
	if !RoleIsRecognised(filepath.Dir(path), "", m[4]) {
		return "", false
	}
	return filepath.Join(".tp-review", m[2], m[1]+"-inbox", fmt.Sprintf("round-%d-%s.ndjson", round, m[4])), true
}

// RemoveRoundInbox removes a recorded round's role files from the phase's
// inbox, and the inbox when it is left empty. The files sit inside the
// repository, so leaving them would keep a recorded round showing in the
// working tree's status. It reports the first file it could not remove.
func RemoveRoundInbox(specPath, phase string, round int) error {
	dir := roleInboxDir(specPath, phase)
	files, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("round-%d-*.ndjson", round)))
	if err != nil {
		return err
	}
	// An audit round's pools (AuditPoolsPath) are in-flight state of the same
	// round, recorded into state.json by now.
	if phase == PhaseAudit {
		files = append(files, AuditPoolsPath(specPath, round))
	}
	for _, f := range files {
		if rmErr := os.Remove(f); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
	}
	_ = os.Remove(dir) // fails, as intended, while another round's file is there
	return nil
}
