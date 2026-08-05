package engine

import "strings"

// SafeGitRev reports whether a revision string can be passed to git as a bare
// argument. git parses any argument beginning with "-" as an option, so a
// stored value such as "--output=/path" would make git write that file instead
// of resolving a revision. Revisions reach git from several directions — a
// recorded commit_sha, an imported task file, an --base flag — so the check
// lives at every sink rather than at one entry point: guarding a single writer
// leaves every other writer open.
func SafeGitRev(rev string) bool {
	rev = strings.TrimSpace(rev)
	return rev != "" && !strings.HasPrefix(rev, "-")
}
