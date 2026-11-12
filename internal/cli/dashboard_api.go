package cli

import (
	"encoding/csv"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/dashboard"
	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// dashboardAPI answers the dashboard's GET endpoints with the payloads tp
// export and tp epic review --history print, derived by the same code
// (spec/1.8.0.md §2). Every payload is the uncompacted one.
type dashboardAPI struct{}

// dashboardHistoryHint answers a /api/history query that names no slice, or
// one the command would refuse.
const dashboardHistoryHint = "name one slice as tp epic review does: ?milestone=<m>, ?epic=<id> or ?epics=<ids>[&name=<name>], with &from=<a>&to=<b> together or neither"

// dashboardOutsideHint answers a task file path that leaves the repository.
const dashboardOutsideHint = "pass ?file= a task file inside this repository, relative to the directory tp dashboard runs in"

// runErrorOf converts a command's exit error into the endpoint's answer.
func runErrorOf(err *epicRunError) *dashboard.Error {
	return dashboard.NewError(err.code, err.msg, err.hint)
}

// Export answers /api/export?file=: the task file's tp export, refused when
// the path resolves outside the repository. The export reads the resolved
// path the containment check passed, never the raw one.
func (dashboardAPI) Export(file string) (any, *dashboard.Error) {
	if file != "" {
		resolved, ok := resolveInsideRepository(file)
		if !ok {
			return nil, dashboard.Forbidden("task file "+file+" is outside the repository", dashboardOutsideHint)
		}
		file = resolved
	}
	payload, err := exportOf(file, false)
	if err != nil {
		return nil, runErrorOf(err)
	}
	return payload, nil
}

// History answers /api/history: the slice's tp epic review --history.
func (dashboardAPI) History(q url.Values) (any, *dashboard.Error) {
	key, from, to, derr := dashboardHistoryQuery(q)
	if derr != nil {
		return nil, derr
	}
	res, err := epicReviewHistoryOf(key, from, to, false)
	if err != nil {
		return nil, runErrorOf(err)
	}
	return res, nil
}

// EpicHistory answers /api/epic-history?epic=: tp epic review --history
// --of-epic.
func (dashboardAPI) EpicHistory(epic string) (any, *dashboard.Error) {
	res, err := epicOfEpicOf(epic)
	if err != nil {
		return nil, runErrorOf(err)
	}
	return res, nil
}

// Live derives what the server pushes (spec/1.8.0.md §2): tp epic export
// while a graph exists, read afresh each tick so a graph written while the
// server runs is pushed, tp export --list, and the summary of the figures no
// command prints. A derivation that fails carries tp's error and hint.
func (dashboardAPI) Live() []dashboard.Payload {
	hasGraph, _ := engine.DashboardSources(".")
	out := make([]dashboard.Payload, 0, 3)
	if hasGraph {
		p := dashboard.Payload{Event: "epic-export"}
		if res, err := epicExport(); err != nil {
			p.Err = runErrorOf(err)
		} else {
			p.Data = res
		}
		out = append(out, p)
	}
	out = append(out, dashboard.Payload{Event: "export-list", Data: engine.TaskFileListOf(".")})
	return append(out, dashboardSummaryPayload(hasGraph))
}

// dashboardSummaryPayload derives the summary over the graph, or over an
// empty one when the repository holds none.
func dashboardSummaryPayload(hasGraph bool) dashboard.Payload {
	p := dashboard.Payload{Event: "summary"}
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		tpDir = filepath.Join(engine.ProjectRoot("."), ".tp")
	}
	g := &model.EpicGraph{}
	if hasGraph {
		eg, err := readEpicReviewGraph()
		if err != nil {
			p.Err = runErrorOf(err)
			return p
		}
		g = eg.g
	}
	sum, err := engine.DashboardSummaryOf(tpDir, g)
	if mce := (*engine.MalformedConfigError)(nil); errors.As(err, &mce) {
		p.Err = runErrorOf(malformedConfigRunError(mce))
		return p
	}
	if err != nil {
		// task-file-hint: an unreadable round file or task file
		p.Err = dashboard.NewError(ExitFile, err.Error(), epicBoundFileHint)
		return p
	}
	p.Data = sum
	return p
}

// dashboardHistoryQuery reads the slice and the rounds from q under the
// command's rules: exactly one slice, --name with --epics alone, --epics as
// one CSV value, and from and to together, each a number.
func dashboardHistoryQuery(q url.Values) (key engine.EpicSliceKey, from, to int, derr *dashboard.Error) {
	key = engine.EpicSliceKey{Milestone: q.Get("milestone"), Epic: q.Get("epic"), Name: q.Get("name")}
	given := 0
	for _, name := range []string{"milestone", "epic", "epics"} {
		if q.Get(name) != "" {
			given++
		}
	}
	bad := func(msg string) (engine.EpicSliceKey, int, int, *dashboard.Error) {
		return engine.EpicSliceKey{}, 0, 0, dashboard.NewError(ExitUsage, msg, dashboardHistoryHint)
	}
	switch {
	case given != 1:
		return bad("/api/history takes exactly one of milestone, epic or epics")
	case key.Name != "" && q.Get("epics") == "":
		return bad("name names a set slice and needs epics")
	case q.Has("from") != q.Has("to"):
		return bad("from and to name two rounds and come together")
	}
	if raw := q.Get("epics"); raw != "" {
		ids, err := csv.NewReader(strings.NewReader(raw)).Read()
		if err != nil || len(ids) == 0 || slices.Contains(ids, "") {
			return bad("epics needs at least one id and no empty one")
		}
		key.Epics = ids
	}
	if q.Has("from") {
		var errFrom, errTo error
		from, errFrom = strconv.Atoi(q.Get("from"))
		to, errTo = strconv.Atoi(q.Get("to"))
		if errFrom != nil || errTo != nil {
			return bad("from and to are round numbers")
		}
	}
	return key, from, to, nil
}

// resolveInsideRepository resolves file the way the OS will when tp opens
// it, against the working directory with every symlink followed, and
// returns that path when it lies under the project root. Containment is
// checked on the resolved path and the caller opens that same path, so the
// check and the open cannot diverge: a lexical clean would drop lnk/.. before
// lnk is followed, while the OS follows lnk first and climbs out of its
// target. A path that does not exist resolves through its nearest existing
// ancestor, so a missing file inside the repository reaches the command's
// not-found error and one through a symlink leading out is refused like an
// existing one.
func resolveInsideRepository(file string) (string, bool) {
	root, err := filepath.EvalSymlinks(engine.ProjectRoot("."))
	if err != nil {
		return "", false
	}
	raw := file
	if !filepath.IsAbs(raw) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		// Concatenated, not filepath.Join: Join cleans, and cleaning is the
		// lexical .. this resolution exists to avoid.
		raw = cwd + string(filepath.Separator) + file
	}
	resolved, ok := resolveThroughExistingAncestor(raw)
	if !ok {
		return "", false
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return resolved, true
}

// resolveThroughExistingAncestor follows every symlink of the uncleaned
// absolute path raw in the order the OS does: raw itself when it exists,
// else its nearest existing ancestor with the missing rest joined back on.
// Ancestors are cut textually, since filepath.Dir cleans. A dangling symlink,
// which names nothing tp can check, is not resolved, and neither is a
// missing rest that climbs with .., which the OS would not open either.
func resolveThroughExistingAncestor(raw string) (string, bool) {
	sep := string(filepath.Separator)
	var missing []string
	for dir := raw; ; {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			if slices.Contains(missing, "..") {
				return "", false
			}
			return filepath.Join(append([]string{resolved}, missing...)...), true
		}
		if _, err := os.Lstat(dir); err == nil {
			return "", false // a dangling symlink
		}
		trimmed := strings.TrimRight(dir, sep)
		i := strings.LastIndex(trimmed, sep)
		if i < 0 {
			return "", false
		}
		missing = append([]string{trimmed[i+1:]}, missing...)
		dir = trimmed[:i]
		if dir == "" {
			dir = sep
		}
	}
}
