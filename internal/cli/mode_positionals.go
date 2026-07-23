package cli

import (
	"path/filepath"
	"strings"
)

// specLikeExtensions are filename extensions that identify a spec/markdown
// document rather than an NDJSON data file.
var specLikeExtensions = map[string]bool{
	".md":       true,
	".markdown": true,
}

// isSpecLookingPath reports whether path looks like a spec/markdown document
// (§4.1). The spec positional is never consumed as data by --merge or
// --resolve/--resolve-all, so a spec-looking positional among their inputs is
// rejected at entry with exit 2.
func isSpecLookingPath(path string) bool {
	return specLikeExtensions[strings.ToLower(filepath.Ext(path))]
}
