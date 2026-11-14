package engine

import (
	"path/filepath"
	"strings"
)

// CleanTree returns the tree id of dir's HEAD when dir's working tree is
// clean: no change to a tracked file and no untracked file git does not
// ignore. A gate runs on a working directory, so only a clean one gated the
// committed tree; anything else, or a git error, returns "".
func CleanTree(dir string) string {
	status, err := gitOut(dir, "status", "--porcelain", "--untracked-files=normal")
	if err != nil || status != "" {
		return ""
	}
	tree, err := gitOut(dir, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return ""
	}
	return tree
}

// ReusableGate reports whether a close on the main branch may reuse the gate
// a unit passed in its worktree instead of running it again: the pending close
// recorded the tree it gated and the command, the command is the one the close
// would run, the main branch's HEAD has that same tree, and its working tree
// differs from HEAD only in paths tp itself writes. Anything unequal or
// unknown answers false, and the gate runs (the operator's decision of
// 2026-11-14, not --skip-gate: a gate did run on this exact tree).
func ReusableGate(root, taskFile, spec string, p *PendingClose, gateCmd string) bool {
	if p == nil || p.GatedTree == "" || gateCmd == "" || p.GateCmd != gateCmd {
		return false
	}
	tree, err := gitOut(root, "rev-parse", "HEAD^{tree}")
	if err != nil || tree != p.GatedTree {
		return false
	}
	return onlyTPOwnedChanges(root, taskFile, spec)
}

// onlyTPOwnedChanges reports whether every change in root's working tree, an
// untracked file included, is a path only tp writes. A path outside the
// project root, or a status git cannot give, is not.
func onlyTPOwnedChanges(root, taskFile, spec string) bool {
	// Two path-only listings rather than `status --porcelain`, whose status
	// columns open with a space that output trimming would eat.
	// --no-renames lists a renamed file's old path too, so a file moved
	// into .tp/ is not read as only a tp-owned change; `:/` makes the
	// untracked listing cover the whole repository, as the diff does.
	changed, err := gitOut(root, "diff", "--no-renames", "--name-only", "HEAD")
	if err != nil {
		return false
	}
	untracked, err := gitOut(root, "ls-files", "--others", "--exclude-standard", "--full-name", "--", ":/")
	if err != nil {
		return false
	}
	prefix, err := gitOut(root, "rev-parse", "--show-prefix")
	if err != nil {
		return false
	}
	for _, list := range []string{changed, untracked} {
		for path := range strings.SplitSeq(list, "\n") {
			if path == "" {
				continue
			}
			rel, found := strings.CutPrefix(filepath.ToSlash(path), filepath.ToSlash(prefix))
			if !found || !tpOwnedPath(root, taskFile, spec, rel) {
				return false
			}
		}
	}
	return true
}

// PendingCloseFor returns the cycle's pending close for id, or nil when there
// is none or it cannot be read.
func PendingCloseFor(root, taskFile, id string) *PendingClose {
	closes, err := ReadPendingCloses(root, taskFile)
	if err != nil {
		return nil
	}
	for i := range closes {
		if closes[i].ID == id {
			return &closes[i]
		}
	}
	return nil
}
