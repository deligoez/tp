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
	Present bool
	Lines   LineRange           // opening --- through closing ---, absolute 1-based
	Domain  string              // default "software"
	Lens    map[string][]string // known keys only: implementer, tester, architect, all
	// ReviewRoles and AuditRoles are the v0.25.0 spec-frontmatter role overrides
	// (tp.review_roles / tp.audit_roles), each keyed by role id (§10.2). The focus
	// questions are appended (additive) to the matching corpus role at emission;
	// resolution lives elsewhere — this struct only carries the parsed overrides.
	ReviewRoles map[string]RoleOverride
	AuditRoles  map[string]RoleOverride
	Errors      []Finding // structural/parse lint errors
	Warnings    []Finding // shape lint warnings
}

// RoleOverride is one parsed tp.review_roles / tp.audit_roles entry, keyed by
// role id in Frontmatter. Focus carries the override's focus questions, layered
// onto the matching corpus role's own focus at emission. Enabled is the optional
// per-spec activation toggle; nil means unset, which is the pre-v0.32.0 behavior.
type RoleOverride struct {
	Focus   []string
	Enabled *bool
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
		} else {
			fm.parseLens(lensMap)
		}
	}

	if rrVal, exists := tpMap["review_roles"]; exists {
		fm.ReviewRoles = fm.parseRoleOverrides("review_roles", rrVal)
	}
	if arVal, exists := tpMap["audit_roles"]; exists {
		fm.AuditRoles = fm.parseRoleOverrides("audit_roles", arVal)
	}

	return fm
}

// parseRoleOverrides parses a tp.review_roles or tp.audit_roles mapping (§10.2):
// each key is a role id, each value an object whose permitted keys are "focus"
// (a string array) and "enabled" (a boolean, parsed into RoleOverride.Enabled;
// enabled: true is a no-op that leaves the role active with its focus layered).
// Any other key inside an override is a lint warning
// and is ignored; the parsed overrides are returned keyed by role id for
// read-time layering onto the corpus role's focus.
func (fm *Frontmatter) parseRoleOverrides(field string, val any) map[string]RoleOverride {
	out := make(map[string]RoleOverride)
	rolesMap, isMap := val.(map[string]any)
	if !isMap {
		fm.warn(fmt.Sprintf("tp.%s is not a mapping (got %T); no role overrides apply", field, val))
		return out
	}

	ids := make([]string, 0, len(rolesMap))
	for id := range rolesMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		override, isOvMap := rolesMap[id].(map[string]any)
		if !isOvMap {
			fm.warn(fmt.Sprintf("tp.%s.%s is not a mapping (got %T); ignored", field, id, rolesMap[id]))
			continue
		}

		// The permitted keys are focus and enabled; warn about and ignore every other key.
		keys := make([]string, 0, len(override))
		for k := range override {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k != "focus" && k != "enabled" {
				fm.warn(fmt.Sprintf("tp.%s.%s.%s is not a permitted override key (only focus and enabled); ignored", field, id, k))
			}
		}

		// A boolean enabled is carried through as the per-spec activation toggle;
		// any other shape (absent, null, non-boolean) leaves it unset (nil).
		var enabled *bool
		if enabledVal, hasEnabled := override["enabled"]; hasEnabled {
			if b, isBool := enabledVal.(bool); isBool {
				enabled = &b
			}
		}

		focusVal, hasFocus := override["focus"]
		if !hasFocus {
			out[id] = RoleOverride{Focus: []string{}, Enabled: enabled}
			continue
		}
		list, isList := focusVal.([]any)
		if !isList {
			if focusVal == nil {
				out[id] = RoleOverride{Focus: []string{}, Enabled: enabled}
				continue
			}
			fm.warn(fmt.Sprintf("tp.%s.%s.focus is not a list (got %T); ignored", field, id, focusVal))
			continue
		}
		questions := make([]string, 0, len(list))
		for i, el := range list {
			s, isStr := el.(string)
			if !isStr {
				fm.warn(fmt.Sprintf("tp.%s.%s.focus[%d] is not a string (got %T); ignored", field, id, i, el))
				continue
			}
			questions = append(questions, s)
		}
		out[id] = RoleOverride{Focus: questions, Enabled: enabled}
	}
	return out
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
		Domain:      DomainSoftware,
		Lens:        make(map[string][]string),
		ReviewRoles: make(map[string]RoleOverride),
		AuditRoles:  make(map[string]RoleOverride),
		Errors:      make([]Finding, 0),
		Warnings:    make([]Finding, 0),
	}
}
