package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	graphTag  string
	graphFrom string
)

type graphNode struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	EstimateMinutes int    `json:"estimate_minutes"`
}

type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type graphResult struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

func newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "ASCII dependency graph",
		RunE:  runGraph,
	}
	cmd.Flags().StringVar(&graphTag, "tag", "", "filter by tag")
	cmd.Flags().StringVar(&graphFrom, "from", "", "subgraph from a specific task")
	return cmd
}

func runGraph(_ *cobra.Command, _ []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	// Build task map
	taskMap := make(map[string]*model.Task)
	for i := range tf.Tasks {
		taskMap[tf.Tasks[i].ID] = &tf.Tasks[i]
	}

	// Filter tasks
	include := make(map[string]bool)
	if graphFrom != "" {
		// BFS from the starting task
		queue := []string{graphFrom}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if include[id] {
				continue
			}
			include[id] = true
			for i := range tf.Tasks {
				for _, dep := range tf.Tasks[i].DependsOn {
					if dep == id {
						queue = append(queue, tf.Tasks[i].ID)
					}
				}
			}
		}
	} else {
		for i := range tf.Tasks {
			include[tf.Tasks[i].ID] = true
		}
	}

	// Apply tag filter
	if graphTag != "" {
		for id := range include {
			t := taskMap[id]
			hasTag := false
			for _, tag := range t.Tags {
				if tag == graphTag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				delete(include, id)
			}
		}
	}

	// JSON output
	if output.IsJSON() {
		result := graphResult{}
		for id := range include {
			t := taskMap[id]
			result.Nodes = append(result.Nodes, graphNode{
				ID:              t.ID,
				Status:          t.Status,
				EstimateMinutes: t.EstimateMinutes,
			})
			for _, dep := range t.DependsOn {
				if include[dep] {
					result.Edges = append(result.Edges, graphEdge{From: dep, To: t.ID})
				}
			}
		}
		return output.JSON(result)
	}

	// ASCII output - find roots (no deps in the included set)
	roots := findRoots(tf, include)
	printed := make(map[string]bool)
	for _, root := range roots {
		printTree(taskMap, tf, include, root, "", true, printed)
	}
	return nil
}

func findRoots(tf *model.TaskFile, include map[string]bool) []string {
	var roots []string
	for i := range tf.Tasks {
		id := tf.Tasks[i].ID
		if !include[id] {
			continue
		}
		isRoot := true
		for _, dep := range tf.Tasks[i].DependsOn {
			if include[dep] {
				isRoot = false
				break
			}
		}
		if isRoot {
			roots = append(roots, id)
		}
	}
	return roots
}

func printTree(taskMap map[string]*model.Task, tf *model.TaskFile, include map[string]bool, id, prefix string, isLast bool, printed map[string]bool) {
	t := taskMap[id]
	if t == nil {
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	suffix := ""
	if printed[id] {
		suffix = "  [already shown]"
	}

	fmt.Printf("%s%s%s (%dm, %s)%s\n", prefix, connector, id, t.EstimateMinutes, t.Status, suffix)

	if printed[id] {
		return
	}
	printed[id] = true

	// Find children (tasks that depend on this one)
	var children []string
	for i := range tf.Tasks {
		if !include[tf.Tasks[i].ID] {
			continue
		}
		for _, dep := range tf.Tasks[i].DependsOn {
			if dep == id {
				children = append(children, tf.Tasks[i].ID)
			}
		}
	}

	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range children {
		printTree(taskMap, tf, include, child, childPrefix, i == len(children)-1, printed)
	}
}
