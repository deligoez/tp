package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taskLayoutRepo is a repository with one task file and no epic graph: a
// chain a → b → c, e beside it on a, and d a root of its own.
func taskLayoutRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\n## 1. X\n\nx.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	for _, task := range [][2]string{{"a", ""}, {"b", `"a"`}, {"c", `"b"`}, {"d", ""}, {"e", `"a"`}} {
		_, stderr, code = runTP(t, dir, "add", fmt.Sprintf(`{"id":%q,"title":"T %s","description":"task %s","status":"open","depends_on":[%s],"estimate_minutes":5,"acceptance":"done","source_sections":["## 1. X"]}`, task[0], task[0], task[0], task[1]))
		require.Equal(t, 0, code, stderr)
	}
	initGitRepo(t, dir)
	return dir
}

// TestDashboard_LayoutLaysATaskFileOutAsAForest: view=tasks lays a listed
// task file out with the forest the epic map uses, layered by dependency
// depth with no step bands, a chain kept in one column, every task drawn
// whatever its status, and it needs no epic graph.
func TestDashboard_LayoutLaysATaskFileOutAsAForest(t *testing.T) {
	t.Parallel()
	dir := taskLayoutRepo(t)
	srv := startDashboard(t, dir, freePort(t))
	for _, pack := range []string{"", "&pack=tree"} {
		status, body := getPage(t, strings.TrimSuffix(srv.url, "/")+"/api/layout?view=tasks&file=spec.tasks.json"+pack)
		require.Equal(t, http.StatusOK, status, body)
		var got struct {
			View   string            `json:"view"`
			Layers int               `json:"layers"`
			Bands  []json.RawMessage `json:"bands"`
			Nodes  []struct {
				ID    string  `json:"id"`
				Layer int     `json:"layer"`
				Col   float64 `json:"col"`
			} `json:"nodes"`
			Edges []struct{ From, To string } `json:"edges"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &got))
		assert.Equal(t, "tasks", got.View)
		assert.Equal(t, 3, got.Layers)
		assert.Empty(t, got.Bands)
		place := map[string][2]float64{}
		for _, n := range got.Nodes {
			place[n.ID] = [2]float64{float64(n.Layer), n.Col}
		}
		require.Len(t, place, 5, pack)
		assert.InDelta(t, 0.0, place["a"][0], 0)
		assert.InDelta(t, 1.0, place["b"][0], 0)
		assert.InDelta(t, 2.0, place["c"][0], 0)
		assert.InDelta(t, 0.0, place["d"][0], 0)
		assert.InDelta(t, 1.0, place["e"][0], 0)
		assert.InDelta(t, place["b"][1], place["c"][1], 0, "the chain b → c keeps its column")
		assert.NotEqual(t, place["a"][1], place["d"][1], "two roots share no column")
		assert.Len(t, got.Edges, 3)
	}
}

// TestDashboard_LayoutRefusesATaskViewWithoutAListedFile: view=tasks needs
// file, takes nothing else beside pack, and a file tp export --list does not
// list answers as absent, never read.
func TestDashboard_LayoutRefusesATaskViewWithoutAListedFile(t *testing.T) {
	t.Parallel()
	srv := startDashboard(t, taskLayoutRepo(t), freePort(t))
	for _, bad := range []string{"/api/layout?view=tasks", "/api/layout?view=tasks&file=", "/api/layout?view=tasks&file=spec.tasks.json&epic=a"} {
		assert.Equal(t, 2, getDashboardError(t, srv, bad, http.StatusBadRequest).Code, bad)
	}
	for _, absent := range []string{"/api/layout?view=tasks&file=nope.tasks.json", "/api/layout?view=tasks&file=..%2Fspec.tasks.json"} {
		assert.Equal(t, 4, getDashboardError(t, srv, absent, http.StatusNotFound).Code, absent)
	}
}
