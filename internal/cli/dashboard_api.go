package cli

import (
	"encoding/csv"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

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
// the path leaves the repository. The task file's bytes are read through the
// repository's os.Root (readInRepository) and exported under the path as
// given, so the answer is tp export --file's own, payload and errors alike.
func (dashboardAPI) Export(file string) (any, *dashboard.Error) {
	var payload exportPayload
	var rerr *epicRunError
	if file == "" {
		payload, rerr = exportOf("", false)
	} else {
		data, derr := readInRepository(file)
		if derr != nil {
			return nil, derr
		}
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

// readInRepository reads the task file file names, relative to the working
// directory or absolute, failing closed. The path is resolved as the OS
// resolves it (resolveAsOS), and the bytes are only ever read through an
// os.Root opened on the project root, under the resolved name relative to
// it, which holds no symlink and no .., so the root re-checks the containment
// in the same operation as the read. The answers:
//
//   - resolved inside the repository: the bytes the root reads; any error of
//     that read refuses (403).
//   - missing, or through a non-directory, at a point the OS reaches inside
//     the repository: tp export --file's not-found error for file as given,
//     read from nothing. These are the inputs the command's os.Stat refuses
//     with that same error while the path stays inside.
//   - reaching outside the repository, missing or not: refused (403).
//   - anything that cannot be decided — too many symlinks, a component that
//     cannot be read, a root that cannot be opened or resolved: refused (403)
//     with its cause.
func readInRepository(file string) ([]byte, *dashboard.Error) {
	refuse := func(why string) *dashboard.Error {
		return dashboard.Forbidden("task file "+file+" "+why, dashboardOutsideHint)
	}
	top := engine.ProjectRoot(".")
	root, err := os.OpenRoot(top)
	if err != nil {
		return nil, refuse("is refused: the repository root cannot be opened: " + err.Error())
	}
	defer func() { _ = root.Close() }()
	resolvedTop, missing, err := resolveAsOS(top)
	if err == nil && missing {
		err = os.ErrNotExist
	}
	if err != nil {
		return nil, refuse("is refused: the repository root cannot be resolved: " + err.Error())
	}
	resolved, missing, err := resolveFromWorkingDirectory(file)
	if err != nil {
		return nil, refuse("is refused: it cannot be told whether it lies outside the repository: " + err.Error())
	}
	name, inside := nameInside(resolvedTop, resolved)
	switch {
	case !inside:
		return nil, refuse("is outside the repository")
	case missing:
		return nil, runErrorOf(exportDiscoverError(engine.ExplicitTaskFileMissing(file)))
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return nil, refuse("is refused: the repository cannot read it: " + err.Error())
	}
	return data, nil
}

// resolveFromWorkingDirectory is resolveAsOS over file, absolute or joined
// to the working directory uncleaned: a lexical clean would drop lnk/..
// before lnk is followed, and the OS follows it first.
func resolveFromWorkingDirectory(file string) (resolved string, missing bool, err error) {
	if !filepath.IsAbs(file) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false, err
		}
		file = cwd + string(filepath.Separator) + file
	}
	return resolveAsOS(file)
}

// nameInside is resolved, a path resolveAsOS returned, relative to top, the
// resolved project root, and whether it lies inside top at all.
func nameInside(top, resolved string) (string, bool) {
	if resolved == top {
		return ".", true
	}
	return strings.CutPrefix(resolved, strings.TrimSuffix(top, string(filepath.Separator))+string(filepath.Separator))
}

// maxSymlinks is how many symlinks the OS follows resolving one path before
// it fails with ELOOP: MAXSYMLINKS, 32, on darwin and 40 on Linux.
func maxSymlinks() int {
	if runtime.GOOS == "darwin" {
		return 32
	}
	return 40
}

// resolveAsOS follows path, absolute and uncleaned, one component at a time
// the way the OS resolves it: a symlink's target replaces it, relative to the
// directory reached so far, and a .. climbs from the directory actually
// reached, not lexically. It returns the path with every symlink resolved.
// When a component does not exist, or the path goes on through one that is
// not a directory, missing is true and resolved is the directory the OS had
// reached, where it would fail with ENOENT or ENOTDIR. Following more than
// maxSymlinks symlinks is ELOOP; any other failure to inspect a component is
// returned as is.
func resolveAsOS(path string) (resolved string, missing bool, err error) {
	sep := string(filepath.Separator)
	vol := filepath.VolumeName(path)
	dest, rest := vol+sep, path[len(vol):]
	links := 0
	for {
		rest = strings.TrimLeft(rest, sep)
		if rest == "" {
			return dest, false, nil
		}
		comp, tail, more := strings.Cut(rest, sep)
		rest = ""
		if more {
			rest = sep + tail
		}
		switch comp {
		case ".":
			continue
		case "..":
			dest = filepath.Dir(dest)
			continue
		}
		next := filepath.Join(dest, comp)
		info, err := os.Lstat(next)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return dest, true, nil
		case err != nil:
			return "", false, err
		case info.Mode()&fs.ModeSymlink == 0:
			if more && !info.IsDir() {
				return dest, true, nil
			}
			dest = next
			continue
		}
		links++
		if links > maxSymlinks() {
			return "", false, &fs.PathError{Op: "resolve", Path: path, Err: syscall.ELOOP}
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", false, err
		}
		if filepath.IsAbs(target) {
			tvol := filepath.VolumeName(target)
			dest, target = tvol+sep, target[len(tvol):]
		}
		rest = target + rest
	}
}
