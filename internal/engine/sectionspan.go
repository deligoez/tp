package engine

// sectionSpans maps each canonical heading to its line span: from the
// heading's line through the line before the next heading with level <= its
// level, or through the last line of the file. A section's span therefore
// includes its subsections. Duplicate canonical headings keep the first
// occurrence's span.
func sectionSpans(headings []*Heading, totalLines int) map[string]LineRange {
	spans := make(map[string]LineRange, len(headings))
	for i, h := range headings {
		end := totalLines
		for j := i + 1; j < len(headings); j++ {
			if headings[j].Level <= h.Level {
				end = headings[j].Line - 1
				break
			}
		}
		key := canonicalHeading(h.Level, h.Text)
		if _, exists := spans[key]; !exists {
			spans[key] = LineRange{Start: h.Line, End: end}
		}
	}
	return spans
}
