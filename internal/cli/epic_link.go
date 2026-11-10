package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// epicEdgeNotFoundHint answers an unlink of an edge the graph does not hold.
const epicEdgeNotFoundHint = "tp epic export --md lists each epic's edges; tp epic link adds one"

// epicAnyOfFlag names the flag that makes link and unlink write an any-of
// group instead of a plain edge (spec/1.7.0.md §7).
const epicAnyOfFlag = "any-of"

func newEpicLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <from> (<to> | --any-of <csv>) <why>",
		Short: "Add the edge from -> to, or the any-of group --any-of names, with its why, or replace that edge's or group's why; a write that adds a lint error exits 1 and writes nothing",
		Args:  epicAnyOfArgs(3),
		RunE:  runEpicLink,
	}
	cmd.Flags().String(epicAnyOfFlag, "", "comma-separated members of an any-of group, in place of <to>")
	epicIfRevisionFlag(cmd)
	return cmd
}

func newEpicUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <from> (<to> | --any-of <csv>)",
		Short: "Remove the edge from -> to, or the any-of group with exactly the members --any-of names; an edge or group the graph does not hold exits 4",
		Args:  epicAnyOfArgs(2),
		RunE:  runEpicUnlink,
	}
	cmd.Flags().String(epicAnyOfFlag, "", "comma-separated members of the any-of group to remove, in place of <to>")
	epicIfRevisionFlag(cmd)
	return cmd
}

// epicAnyOfArgs takes plain positional arguments, or one fewer when
// --any-of stands in for <to>.
func epicAnyOfArgs(plain int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(epicAnyOfFlag) {
			return cobra.ExactArgs(plain-1)(cmd, args)
		}
		return cobra.ExactArgs(plain)(cmd, args)
	}
}

// epicAnyOfMembers returns --any-of's members, or nil and false when the
// flag is not given.
func epicAnyOfMembers(cmd *cobra.Command) ([]string, bool) {
	if !cmd.Flags().Changed(epicAnyOfFlag) {
		return nil, false
	}
	csv, _ := cmd.Flags().GetString(epicAnyOfFlag)
	return engine.SplitEpicIDs(csv), true
}

// runEpicLink implements tp epic link (spec/1.4.0.md §1, spec/1.7.0.md §7).
func runEpicLink(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic link")
	var action string
	var mutate func(*model.EpicGraph) error
	if members, ok := epicAnyOfMembers(cmd); ok {
		mutate = engine.LinkEpicAnyOf(args[0], members, args[1], &action)
	} else {
		mutate = engine.LinkEpic(args[0], args[1], args[2], &action)
	}
	writeEpicGraphWith(epicWriteOpts{ifRevision: epicIfRevision(cmd), action: &action}, mutate)
	return nil
}

// runEpicUnlink implements tp epic unlink (spec/1.4.0.md §1, spec/1.7.0.md §7).
func runEpicUnlink(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic unlink")
	var action string
	var mutate func(*model.EpicGraph) error
	if members, ok := epicAnyOfMembers(cmd); ok {
		mutate = engine.UnlinkEpicAnyOf(args[0], members, &action)
	} else {
		mutate = engine.UnlinkEpic(args[0], args[1], &action)
	}
	writeEpicGraphWith(epicWriteOpts{ifRevision: epicIfRevision(cmd), action: &action}, mutate)
	return nil
}

// epicEdgeNotFoundHintOf answers an unlink of an absent edge: the generic
// hint, or, when the id is a member of the epic's groups, the unlink that
// removes each such group.
func epicEdgeNotFoundHintOf(e *engine.EpicEdgeNotFoundError) string {
	if len(e.Groups) == 0 {
		return epicEdgeNotFoundHint
	}
	var unlinks []string
	for _, members := range e.Groups {
		unlinks = append(unlinks, fmt.Sprintf("tp epic unlink %s --any-of %s", e.From, strings.Join(members, ",")))
	}
	return fmt.Sprintf("%q is only a member of an any-of group of %q, not a plain edge: %s removes the whole group", e.To, e.From, strings.Join(unlinks, " or "))
}
