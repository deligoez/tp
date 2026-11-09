package engine

import (
	"fmt"
	"strings"
)

// EpicBoundFileError is --epic's refusal (spec/1.6.0.md F8): the epic binds
// no task file, or binds several and not exactly one of them is short of
// release. Files are the candidates, absolute: the files not at release, or
// every bound file when all are at release.
type EpicBoundFileError struct {
	Epic  string
	Files []string
}

func (e *EpicBoundFileError) Error() string {
	return e.Message(e.Files)
}

// Message renders the refusal with files as the caller names them.
func (e *EpicBoundFileError) Message(files []string) string {
	if len(files) == 0 {
		return fmt.Sprintf("epic %q has no bound task file", e.Epic)
	}
	return fmt.Sprintf("epic %q binds %d task files and cannot choose one: %s", e.Epic, len(files), strings.Join(files, ", "))
}

// EpicBoundFile resolves epic id to one task file: its only bound file, or
// the only one not at the release phase tp resume reports, the phase
// DeriveEpicStates reads. Otherwise it is an *EpicBoundFileError naming the
// candidates. An unreadable task file is EpicBoundFiles' error.
func EpicBoundFile(start, id string) (string, error) {
	bound, err := EpicBoundFiles(start)
	if err != nil {
		return "", err
	}
	files := bound[id]
	if len(files) == 1 {
		return files[0], nil
	}
	var open []string
	for _, path := range files {
		f, err := readBoundFile(path)
		if err != nil {
			return "", err
		}
		if f.phase != PhaseRelease {
			open = append(open, path)
		}
	}
	switch len(open) {
	case 1:
		return open[0], nil
	case 0:
		return "", &EpicBoundFileError{Epic: id, Files: files}
	default:
		return "", &EpicBoundFileError{Epic: id, Files: open}
	}
}
