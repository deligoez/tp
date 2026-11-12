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
// the path leaves the repository. The task file is read through the
// repository's os.Root and exported under the path as given, so the answer
// is tp export --file's own, payload and errors alike.
func (dashboardAPI) Export(file string) (any, *dashboard.Error) {
	var payload exportPayload
	var rerr *epicRunError
	var data []byte
	var err error
	if file != "" {
		data, err = readInRepository(file)
	}
	switch {
	case file == "":
		payload, rerr = exportOf("", false)
	case errors.Is(err, errOutsideRepository):
		return nil, dashboard.Forbidden("task file "+file+" is outside the repository", dashboardOutsideHint)
	case err != nil:
		// Inside but unreadable, a missing file above all: the command's
		// own error, naming the path as given.
		payload, rerr = exportOf(file, false)
	default:
		payload, rerr = exportDataOf(file, data)
	}
	if rerr != nil {
		return nil, runErrorOf(rerr)
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

// errOutsideRepository is readInRepository's refusal of a path that leaves
// the repository.
var errOutsideRepository = errors.New("outside the repository")

// readInRepository reads the task file file names, relative to the working
// directory or absolute, through an os.Root opened on the project root, so
// the check and the read are one operation: a path that leaves the root, by
// .., a symlink or an absolute name, fails with errOutsideRepository and
// cannot be swapped for one that does between a check and an open. Any
// other failure, a missing file or one through a missing directory, is the
// root's own error, which the caller answers as tp export would. os.Root
// also refuses every absolute symlink and a .. that climbs back in through a
// symlink, even when the OS would land inside; such a path is followed as
// the OS follows it and the result, when inside, is read through the root.
func readInRepository(file string) ([]byte, error) {
	top := engine.ProjectRoot(".")
	root, err := os.OpenRoot(top)
	if err != nil {
		return nil, errOutsideRepository
	}
	defer func() { _ = root.Close() }()
	if name, ok := nameInRoot(top, file); ok {
		data, err := root.ReadFile(name)
		if err == nil || !escapesRoot(root, err) {
			return data, err
		}
	}
	name, ok := resolvedNameInRoot(top, file)
	if !ok {
		return nil, errOutsideRepository
	}
	data, err := root.ReadFile(name)
	if err != nil && escapesRoot(root, err) {
		return nil, errOutsideRepository
	}
	return data, err
}

// escapesRoot reports whether err is root's refusal of a path that leaves
// it. os.Root exports no sentinel for that refusal, so it is taken from the
// error root gives for "..", which always leaves the root; without one, every
// error counts as leaving, so a failed probe refuses rather than serves.
func escapesRoot(root *os.Root, err error) bool {
	_, probe := root.Stat("..")
	sentinel := errors.Unwrap(probe)
	return sentinel == nil || errors.Is(err, sentinel)
}

// nameInRoot is file as a name relative to top, the root's directory,
// uncleaned so the root walks every .. as the OS does: a relative file under
// the working directory's place in top, an absolute one with top's prefix,
// as written or with its symlinks resolved, cut off. An absolute path under
// neither prefix has no such name.
func nameInRoot(top, file string) (string, bool) {
	sep := string(filepath.Separator)
	if !filepath.IsAbs(file) {
		cwd, err := filepath.Abs(".")
		if err != nil {
			return "", false
		}
		prefix, err := filepath.Rel(top, cwd)
		if err != nil {
			return "", false
		}
		if prefix == "." {
			return file, true
		}
		return prefix + sep + file, true
	}
	bases := []string{top}
	if resolvedTop, err := filepath.EvalSymlinks(top); err == nil && resolvedTop != top {
		bases = append(bases, resolvedTop)
	}
	for _, base := range bases {
		if rest, ok := strings.CutPrefix(file, strings.TrimSuffix(base, sep)+sep); ok {
			return rest, true
		}
	}
	return "", false
}

// resolvedNameInRoot follows file, against the working directory and
// uncleaned, through every symlink in the order the OS does, and returns the
// result relative to top's resolved directory when it lies inside. A path
// that does not resolve, missing or dangling, has no such name.
func resolvedNameInRoot(top, file string) (string, bool) {
	sep := string(filepath.Separator)
	raw := file
	if !filepath.IsAbs(raw) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		// Concatenated, not filepath.Join: Join cleans, and a lexical clean
		// drops lnk/.. before lnk is followed.
		raw = cwd + sep + file
	}
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", false
	}
	resolvedTop, err := filepath.EvalSymlinks(top)
	if err != nil {
		return "", false
	}
	if resolved == resolvedTop {
		return ".", true
	}
	return strings.CutPrefix(resolved, strings.TrimSuffix(resolvedTop, sep)+sep)
}
