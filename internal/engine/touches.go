package engine

import (
	"fmt"
	"path"
	"strings"
)

// The task field touches names, as repo-relative globs, the paths a task may
// change (spec/backlog/parallel-implement.md §2.1). A glob reads as
// path.Match does; one ending in "/" or "/**" takes its whole subtree.

// touchesPrefix is a glob's literal text before its first wildcard, lowered,
// with a subtree suffix dropped.
func touchesPrefix(glob string) string {
	g := strings.TrimSuffix(strings.TrimSuffix(glob, "**"), "/")
	if i := strings.IndexAny(g, "*?["); i >= 0 {
		g = g[:i]
	}
	return strings.ToLower(g)
}

// globsOverlap reports whether two globs might name a common path: unless one
// literal prefix is a prefix of the other, they diverge and cannot. Case is
// ignored, since a case-insensitive filesystem holds README.md and readme.md
// as one file; an empty prefix overlaps everything.
func globsOverlap(a, b string) bool {
	pa, pb := touchesPrefix(a), touchesPrefix(b)
	return strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa)
}

// TouchesOverlap reports whether two tasks' touches may name a common path. A
// task with no touches overlaps every other, so it runs alone.
func TouchesOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	for _, x := range a {
		for _, y := range b {
			if globsOverlap(x, y) {
				return true
			}
		}
	}
	return false
}

// TouchesMatch reports whether a repo-relative path lies inside globs.
func TouchesMatch(globs []string, p string) bool {
	for _, g := range globs {
		if sub, ok := touchesSubtree(g); ok {
			if p == sub || strings.HasPrefix(p, sub+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(g, p); ok {
			return true
		}
	}
	return false
}

// touchesSubtree returns the directory a subtree glob ("dir/" or "dir/**")
// names.
func touchesSubtree(g string) (string, bool) {
	switch {
	case strings.HasSuffix(g, "/**"):
		return strings.TrimSuffix(g, "/**"), true
	case strings.HasSuffix(g, "/"):
		return strings.TrimSuffix(g, "/"), true
	}
	return "", false
}

// ValidTouches refuses a glob that is empty, absolute, escapes the
// repository, or that path.Match cannot read.
func ValidTouches(globs []string) error {
	for _, g := range globs {
		switch {
		case strings.TrimSpace(g) == "":
			return fmt.Errorf("touches holds an empty glob")
		case strings.HasPrefix(g, "/"):
			return fmt.Errorf("touches glob %q is absolute; write it relative to the repository root", g)
		case g == ".." || strings.HasPrefix(g, "../") || strings.Contains(g, "/../"):
			return fmt.Errorf("touches glob %q leaves the repository", g)
		}
		base := g
		if sub, ok := touchesSubtree(g); ok {
			base = sub
		}
		if _, err := path.Match(base, ""); err != nil {
			return fmt.Errorf("touches glob %q is malformed: %w", g, err)
		}
	}
	return nil
}
