package engine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RolesHashBuiltin is the sentinel corpus hash for a phase with no user role
// files — the embedded default corpus is in force (§9.1).
const RolesHashBuiltin = "builtin"

// ComputeRolesHash derives a phase's corpus hash (review_roles_hash /
// audit_roles_hash, §9.1). It is the sha256 over the phase's user role files
// sorted by repo-relative path, hashing each file's repo-relative path (forward
// slashes) and its CRLF-normalized content, so the hash is identical across
// clones regardless of checkout path or git autocrlf/eol settings. A phase with
// no user files hashes to the sentinel "builtin". The built-in regression role is
// never a file, so it is never part of review_roles_hash (§5.2); a spec-
// frontmatter override is covered by spec_hash, not here (no double-count).
func ComputeRolesHash(startDir, phase string) (string, error) {
	tpDir := DiscoverTPDir(startDir)
	if tpDir == "" {
		return RolesHashBuiltin, nil
	}
	phaseDir := filepath.Join(tpDir, phase)
	entries, err := os.ReadDir(phaseDir)
	if err != nil {
		return RolesHashBuiltin, nil
	}
	root := ProjectRoot(startDir)

	type roleFile struct {
		relPath string
		abs     string
	}
	files := make([]roleFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		abs := filepath.Join(phaseDir, e.Name())
		rel, relErr := filepath.Rel(root, abs)
		if relErr != nil {
			rel = e.Name()
		}
		files = append(files, roleFile{relPath: filepath.ToSlash(rel), abs: abs})
	}
	if len(files) == 0 {
		return RolesHashBuiltin, nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })

	h := sha256.New()
	for i := range files {
		data, readErr := os.ReadFile(files[i].abs)
		if readErr != nil {
			return "", fmt.Errorf("cannot read role file %s: %w", files[i].abs, readErr)
		}
		normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
		h.Write([]byte(files[i].relPath))
		h.Write([]byte{0})
		h.Write([]byte(normalized))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
