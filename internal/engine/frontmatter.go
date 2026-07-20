package engine

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DomainSoftware is the default spec domain; only this exact value activates
// the software-specific prompt content.
const DomainSoftware = "software"

// LensRoleOrder lists the known lens keys in reporting order.
var LensRoleOrder = []string{"implementer", "tester", "architect", "all"}

// Frontmatter is the parsed state of a spec's YAML frontmatter. tp reads only
// the tp: mapping; every other top-level key is ignored.
type Frontmatter struct {
	Present  bool
	Lines    LineRange           // opening --- through closing ---, absolute 1-based
	Domain   string              // default "software"
	Lens     map[string][]string // known keys only: implementer, tester, architect, all
	Errors   []Finding           // structural/parse lint errors
	Warnings []Finding           // shape lint warnings
}

// ParseFrontmatter reads a spec file and parses its frontmatter. A missing or
// unreadable file, or a first line other than "---", yields the defaults
// (no frontmatter, domain software, no lens).
func ParseFrontmatter(specPath string) *Frontmatter {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return defaultFrontmatter()
	}
	return ParseFrontmatterBytes(data)
}

// ParseFrontmatterBytes parses frontmatter from raw spec bytes.
// Structural failures degrade safely: an unterminated block is treated as
// content with a lint error; a closed block whose YAML fails to parse stays
// excluded with defaults and a lint error.
func ParseFrontmatterBytes(data []byte) *Frontmatter {
	fm := defaultFrontmatter()

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm
	}

	closing := 0
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closing = i + 1 // 1-based line number
			break
		}
	}
	if closing == 0 {
		fm.Errors = append(fm.Errors, Finding{
			Severity: "error",
			Rule:     "frontmatter",
			Line:     1,
			Message:  "frontmatter opened with --- at line 1 but never closed; treating all lines as content",
		})
		return fm
	}

	fm.Present = true
	fm.Lines = LineRange{Start: 1, End: closing}

	body := strings.Join(lines[1:closing-1], "\n")
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		fm.Errors = append(fm.Errors, Finding{
			Severity: "error",
			Rule:     "frontmatter",
			Line:     1,
			Message:  fmt.Sprintf("frontmatter YAML parse failed: %v", err),
		})
		return fm
	}

	tpVal, ok := doc["tp"]
	if !ok {
		return fm
	}
	tpMap, ok := tpVal.(map[string]any)
	if !ok {
		fm.warn("tp is not a mapping; defaults apply (domain software, no lens)")
		return fm
	}

	if domainVal, exists := tpMap["domain"]; exists {
		if s, isStr := domainVal.(string); isStr {
			fm.Domain = s
		} else {
			fm.warn(fmt.Sprintf("tp.domain is not a string (got %T); default %q applies", domainVal, DomainSoftware))
		}
	}

	if lensVal, exists := tpMap["lens"]; exists {
		lensMap, isMap := lensVal.(map[string]any)
		if !isMap {
			fm.warn(fmt.Sprintf("tp.lens is not a mapping (got %T); no lens applies", lensVal))
			return fm
		}
		fm.parseLens(lensMap)
	}

	return fm
}

// parseLens validates each lens key and value, warning about and ignoring
// unknown keys, non-list values, and non-string list elements.
func (fm *Frontmatter) parseLens(lensMap map[string]any) {
	known := make(map[string]bool, len(LensRoleOrder))
	for _, k := range LensRoleOrder {
		known[k] = true
	}

	keys := make([]string, 0, len(lensMap))
	for k := range lensMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if !known[key] {
			fm.warn(fmt.Sprintf("tp.lens key %q is unknown (known: %s); ignored", key, strings.Join(LensRoleOrder, ", ")))
			continue
		}
		list, isList := lensMap[key].([]any)
		if !isList {
			if lensMap[key] == nil {
				fm.Lens[key] = []string{}
				continue
			}
			fm.warn(fmt.Sprintf("tp.lens.%s is not a list (got %T); ignored", key, lensMap[key]))
			continue
		}
		questions := make([]string, 0, len(list))
		for i, el := range list {
			s, isStr := el.(string)
			if !isStr {
				fm.warn(fmt.Sprintf("tp.lens.%s[%d] is not a string (got %T); ignored", key, i, el))
				continue
			}
			questions = append(questions, s)
		}
		fm.Lens[key] = questions
	}
}

func (fm *Frontmatter) warn(msg string) {
	fm.Warnings = append(fm.Warnings, Finding{
		Severity: "warning",
		Rule:     "frontmatter",
		Line:     1,
		Message:  msg,
	})
}

func defaultFrontmatter() *Frontmatter {
	return &Frontmatter{
		Domain:   DomainSoftware,
		Lens:     make(map[string][]string),
		Errors:   make([]Finding, 0),
		Warnings: make([]Finding, 0),
	}
}
