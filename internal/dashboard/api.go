package dashboard

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// API produces the read models the page fetches by GET (spec/1.8.0.md §2),
// in-process, by the code the commands use. Each method answers the payload
// the command prints, or the error the command would exit with.
type API interface {
	// Export is tp export --file file's payload for a file tp export --list
	// lists, and an absent object's error for any other.
	Export(file string) (any, *Error)
	// History is tp epic review <slice> --history's payload for the slice and
	// rounds the query names.
	History(query url.Values) (any, *Error)
	// EpicHistory is tp epic review --history --of-epic epic's payload.
	EpicHistory(epic string) (any, *Error)
	// Live derives the payloads the server pushes on every tick: tp epic
	// export when a graph exists, tp export --list, and the summary.
	Live() []Payload
}

// Error is tp's error as an endpoint answers it: the exit code the command
// would exit with, its message and its hint, and the HTTP status they map to.
type Error struct {
	Status  int    `json:"-"`
	Message string `json:"error"`
	Code    int    `json:"code"`
	Hint    string `json:"hint"`
}

// NewError is the error a command exits with code on: a usage error (2) is a
// bad request, a missing file (3) or object (4) is not found, any other code
// an internal error.
func NewError(code int, msg, hint string) *Error {
	status := http.StatusInternalServerError
	switch code {
	case 2:
		status = http.StatusBadRequest
	case 3, 4:
		status = http.StatusNotFound
	}
	return &Error{Status: status, Message: msg, Code: code, Hint: hint}
}

// Forbidden is a request for a path outside the repository: refused with a
// usage error's code, since the request, not the repository, is at fault.
func Forbidden(msg, hint string) *Error {
	return &Error{Status: http.StatusForbidden, Message: msg, Code: 2, Hint: hint}
}

// apiHandlers maps each endpoint path to the handler answering it.
func apiHandlers(api API) map[string]func(url.Values) (any, *Error) {
	return map[string]func(url.Values) (any, *Error){
		"/api/export":  func(q url.Values) (any, *Error) { return api.Export(q.Get("file")) },
		"/api/history": func(q url.Values) (any, *Error) { return api.History(q) },
		"/api/epic-history": func(q url.Values) (any, *Error) {
			epic := q.Get("epic")
			if epic == "" {
				return nil, NewError(2, "/api/epic-history needs an epic", "pass ?epic=<id>, as tp epic review --history --of-epic <id> takes one")
			}
			return api.EpicHistory(epic)
		},
	}
}

// writeJSON writes v as the commands print it: two-space indented JSON and a
// trailing newline, so a body is byte-equal to the command's stdout.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// serveAPI answers one endpoint request with its payload or its error.
func serveAPI(w http.ResponseWriter, r *http.Request, handle func(url.Values) (any, *Error)) {
	payload, err := handle(r.URL.Query())
	if err != nil {
		writeJSON(w, err.Status, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}
