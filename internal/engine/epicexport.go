package engine

import (
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicGraphMarkdown renders the graph as one Markdown table per milestone, in
// track order (spec/1.3.0.md §6): one row per epic with its id, title, tags and
// every dependency with its why. Rows keep the file's epic order. A milestone
// named twice gets one table, and epics whose milestone is in no track follow
// in tables of their own, so every epic is in exactly one row.
func EpicGraphMarkdown(g *model.EpicGraph) string {
	byMilestone := map[string][]model.Epic{}
	var order []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
	}
	for _, track := range g.Milestones {
		for _, name := range track {
			add(name)
		}
	}
	for i := range g.Epics {
		e := &g.Epics[i]
		add(e.Milestone)
		byMilestone[e.Milestone] = append(byMilestone[e.Milestone], *e)
	}

	var b strings.Builder
	for i, name := range order {
		if i > 0 {
			b.WriteString("\n")
		}
		writeMilestoneTable(&b, name, byMilestone[name])
	}
	return b.String()
}

// writeMilestoneTable writes one milestone's heading and table.
func writeMilestoneTable(b *strings.Builder, name string, epics []model.Epic) {
	b.WriteString("## " + markdownCell(name) + "\n\n")
	b.WriteString("| id | title | tags | depends on |\n|---|---|---|---|\n")
	for k := range epics {
		e := &epics[k]
		deps := make([]string, 0, len(e.DependsOn))
		for _, d := range e.DependsOn {
			deps = append(deps, d.ID+": "+d.Why)
		}
		cells := []string{e.ID, e.Title, strings.Join(e.Tags, ", "), strings.Join(deps, "; ")}
		for i := range cells {
			cells[i] = markdownCell(cells[i])
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
}

// markdownCell keeps a value inside one table cell: a pipe is escaped and a
// line break becomes a space.
func markdownCell(s string) string {
	return strings.NewReplacer("|", `\|`, "\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}
