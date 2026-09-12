package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

// A quality gate is a shell string, and tp does not parse shell. What this file
// resolves is deliberately narrow: the first word of a command segment, after
// splitting on &&, ||, ;, | and newlines, skipping leading VAR=value
// assignments. Anything it cannot decide without a shell — a builtin, a
// keyword, a word carrying an expansion — counts as resolving, so the notice
// it drives can be missing on an exotic gate but is never raised over a
// working one.

// shellWords are the builtins and reserved words a segment may start with that
// no PATH lookup finds on every host (exit, cd, source) or that are grammar
// rather than commands (if, then, !).
var shellWords = map[string]bool{
	":": true, ".": true, "!": true, "[": true, "[[": true, "{": true, "}": true,
	"alias": true, "break": true, "case": true, "cd": true, "command": true,
	"continue": true, "do": true, "done": true, "echo": true, "elif": true,
	"else": true, "esac": true, "eval": true, "exec": true, "exit": true,
	"export": true, "false": true, "fi": true, "for": true, "function": true,
	"if": true, "printf": true, "pwd": true, "read": true, "readonly": true,
	"return": true, "set": true, "shift": true, "source": true, "test": true,
	"then": true, "time": true, "trap": true, "true": true, "type": true,
	"ulimit": true, "umask": true, "unset": true, "until": true, "wait": true,
	"while": true,
}

// envAssignment matches a leading VAR=value word, which the shell applies to
// the command after it rather than running.
var envAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// segmentSeparators splits a gate into command segments. && and || come first
// so a lone & inside a redirection (2>&1) never splits.
var segmentSeparators = strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")

// shellNotFound matches the line sh prints when it cannot run a command:
// bash's "sh: x: command not found" and "sh: line 1: x: ...", dash's
// "sh: 1: x: not found", and "Permission denied" for exit 126.
var shellNotFound = regexp.MustCompile(`([^\s:]+): (?:command not found|not found|Permission denied)\s*$`)

// gateSegmentHeads returns the first command word of every segment of gate.
func gateSegmentHeads(gate string) []string {
	heads := make([]string, 0)
	for seg := range strings.SplitSeq(segmentSeparators.Replace(gate), "\n") {
		if head := segmentHead(seg); head != "" {
			heads = append(heads, head)
		}
	}
	return heads
}

// segmentHead is the first word of seg that is not an environment assignment,
// with subshell or group brackets and surrounding quotes stripped.
func segmentHead(seg string) string {
	for word := range strings.FieldsSeq(seg) {
		if envAssignment.MatchString(word) {
			continue
		}
		if trimmed := strings.Trim(word, "(){}"); trimmed != "" {
			word = trimmed
		}
		return strings.Trim(word, `"'`)
	}
	return ""
}

// commandResolves reports whether word names something sh can run from dir:
// a builtin or keyword, a PATH entry, or an existing executable file when the
// word is a path. dir "" is the working directory, as it is for RunCommand.
func commandResolves(word, dir string) bool {
	if shellWords[word] || strings.ContainsAny(word, "$`*?~") {
		return true
	}
	if !strings.ContainsRune(word, '/') {
		_, err := exec.LookPath(word)
		return err == nil
	}
	if !filepath.IsAbs(word) {
		word = filepath.Join(dir, word)
	}
	info, err := os.Stat(word)
	if err != nil || info.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

// UnresolvedGateHead returns the first word of gate's first segment when it
// names nothing sh can run from dir, and "" otherwise (an empty gate
// included). Only the first segment is resolved: it is what init and import
// can check cheaply, and a gate whose first command is a typo is the case an
// agent otherwise meets only at close.
func UnresolvedGateHead(gate, dir string) string {
	heads := gateSegmentHeads(gate)
	if len(heads) == 0 || commandResolves(heads[0], dir) {
		return ""
	}
	return heads[0]
}

// GateMissingCommand names the command a gate that exited 126 or 127 could
// not run: the first segment head that does not resolve from dir, else the
// command sh named in the gate's output (which covers a script the gate calls
// failing on a command of its own), else "".
func GateMissingCommand(gate, dir string, outputTail []string) string {
	for _, head := range gateSegmentHeads(gate) {
		if !commandResolves(head, dir) {
			return head
		}
	}
	for _, o := range slices.Backward(outputTail) {
		if m := shellNotFound.FindStringSubmatch(o); m != nil {
			return m[1]
		}
	}
	return ""
}
