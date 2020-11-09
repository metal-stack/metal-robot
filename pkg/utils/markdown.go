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

func (m *Markdown) EnsureSection(level int, headlinePrefix *string, headline string, contentLines []string) *MarkdownSection {
	for _, s := range m.sections {
		if s.Level == level {
			if headlinePrefix == nil {
				return s
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
	m.sections = append(m.sections, s)
	return s
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