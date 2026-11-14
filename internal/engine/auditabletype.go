package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// IsAuditableType reports whether a path survives the type filtering the
// auto-detection applies: binary files, markdown, task files, tp's own state
// directory and extensionless repository metadata are skipped.
func IsAuditableType(path string) bool {
	if IsBinaryPath(path) {
		return false
	}
	if strings.HasSuffix(path, ".md") {
		return false
	}
	if strings.HasSuffix(path, ".tasks.json") {
		return false
	}
	if IsTPStatePath(path) {
		return false
	}
	if isRepoMetadataDotfile(path) {
		return false
	}
	return true
}

// IsTPStatePath reports whether the path lies inside tp's own state directory.
// .tp/ holds the project config, the active pointer, the role corpus and the
// run state: tp's bookkeeping about the cycle, never the code a spec is audited
// against. A task that edits one commits it like any other file, so the path
// reaches an auto-detected file list honestly and costs every role its
// attention.
func IsTPStatePath(path string) bool {
	for part := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		// .tp-review/ beside a spec holds its round files, snapshots and
		// state.json: tp's record of the review and audit loops, as .tp/ is
		// of the cycle.
		if part == ".tp" || part == ".tp-review" {
			return true
		}
	}
	return false
}

// isRepoMetadataDotfile reports whether the base name is a dotfile carrying no
// further extension — .gitignore, .gitattributes, .dockerignore.
//
// The leading dot is stripped before the check because filepath.Ext(".gitignore")
// returns ".gitignore", not "": the final dot is at index 0, so the whole name
// reads as the extension. A dotfile that does carry a real extension is left
// auditable, which is what keeps .golangci.yml and .github workflows in the set.
func isRepoMetadataDotfile(path string) bool {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, ".") || base == "." || base == ".." {
		return false
	}
	return !strings.Contains(base[1:], ".")
}

// FileCheckText is the checklist text of a file_check item: the role's rules
// applied to one file.
func FileCheckText(roleID, path string) string {
	return fmt.Sprintf("Apply the %s role rules to %s", roleID, path)
}

// FileCheckItemID is a file_check item's id. It is a function of the role and
// the file's path and of nothing else (not its position in the list, not which
// other files share the list, not the shard it was emitted in):
// file-<roleID>-<slug>.<digest>, where <slug> is the readable prefix
// SlugifySubject derives from the subject (path + checklist text, cut to 40
// characters) and <digest> is fileCheckDigest(path). Two files whose slugs
// collide are told apart by the digest, so there is no positional suffix. The
// old -2, -3 suffix depended on the neighbours, and two shards of one round
// could give one id to two different files.
func FileCheckItemID(roleID, path string) string {
	return "file-" + roleID + "-" + SlugifySubject(path+" "+FileCheckText(roleID, path)) + "." + fileCheckDigest(path)
}

// fileCheckDigestLen is how many hex characters of the path's SHA-256 a
// file_check id carries: 64 bits, so two files sharing a 40-character slug
// and a digest is not a case worth a code path.
const fileCheckDigestLen = 16

// fileCheckDigest is the first fileCheckDigestLen hex characters of the
// SHA-256 of the file's cleaned, slash-separated path, so one file spelled
// "app/a.go", "./app/a.go" or "app//a.go" keeps one id.
func fileCheckDigest(p string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(filepath.Clean(p))))
	return hex.EncodeToString(sum[:])[:fileCheckDigestLen]
}

// SlugifySubject derives a stable slug from an audit item's subject text
// (§10.3): the subject is lowercased; runs of non-alphanumeric characters
// collapse to a single "-"; leading and trailing "-" are trimmed; then the
// result is truncated to at most 40 characters with any trailing "-" trimmed.
// Because the subject always includes a file path, the slug always contains at
// least one letter (§10.9).
func SlugifySubject(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteRune('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	return slug
}
