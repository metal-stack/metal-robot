package utils

import "strings"

type Markdown struct {
	sections []*MarkdownSection
}

type MarkdownSection struct {
	Level        int
	Heading      string
	ContentLines []string
}

func ParseMarkdown(content string) *Markdown {
	var sections []*MarkdownSection
	lines := strings.Split(content, "\n")

	var currentSection *MarkdownSection
	for _, l := range lines {
		if strings.HasPrefix(l, "#") {
			level := 0
			for _, char := range l {
				if char != '#' {
					break
				}
				level++
			}
			currentSection = &MarkdownSection{
				Level:   level,
				Heading: strings.TrimSpace(l[level:]),
			}
			sections = append(sections, currentSection)
			continue
		}

		if currentSection == nil {
			currentSection = &MarkdownSection{}
			sections = append(sections, currentSection)
		}

		currentSection.ContentLines = append(currentSection.ContentLines, l)
	}

	return &Markdown{
		sections: sections,
	}
}

// EnsureSection ensures a section in the markdown and returns nil.
// If headlinePrefix is given and a headline with this prefix already exists, it returns the existing section.
func (m *Markdown) EnsureSection(level int, headlinePrefix *string, headline string, contentLines []string, prepend bool) *MarkdownSection {
	for _, s := range m.sections {
		if s.Level == level {
			if headline == s.Heading {
				return s
			}

			if headlinePrefix == nil {
				continue
			}
			if strings.HasPrefix(s.Heading, *headlinePrefix) {
				return s
			}
		}
	}
	s := &MarkdownSection{
		Level:        level,
		Heading:      headline,
		ContentLines: contentLines,
	}

	if prepend {
		m.sections = append([]*MarkdownSection{s}, m.sections...)
	} else {
		m.sections = append(m.sections, s)
	}

	return nil
}

func (m *Markdown) String() string {
	result := ""
	for i, s := range m.sections {
		if s.Level > 0 {
			for i := 0; i < s.Level; i++ {
				result += "#"
			}
			result += " " + s.Heading + "\n"
		}
		result += strings.Join(s.ContentLines, "\n")

		isLast := len(m.sections)-1 == i
		if !isLast {
			result += "\n"
		}
	}
	return result
}
