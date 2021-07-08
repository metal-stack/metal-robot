package markdown

import (
	"strings"
)

type Markdown struct {
	sections []*MarkdownSection
}

func Parse(content string) *Markdown {
	m := &Markdown{}
	lines := strings.Split(content, "\n")

	var currentSection *MarkdownSection
	for _, l := range lines {
		if isHeading(l) {
			level := headingLevel(l)

			currentSection = &MarkdownSection{
				Level:   level,
				Heading: strings.TrimSpace(l[level:]),
			}

			isChild := false
			if level > 0 {
				allSections := m.allSections()
				for i := len(allSections) - 1; i >= 0; i-- {
					if allSections[i].Level < level {
						allSections[i].SubSections = append(allSections[i].SubSections, currentSection)
						isChild = true
						break
					}
				}
			}

			if !isChild {
				m.sections = append(m.sections, currentSection)
			}

			continue
		}

		if currentSection == nil {
			currentSection = &MarkdownSection{}
			m.sections = append(m.sections, currentSection)
		}

		currentSection.ContentLines = append(currentSection.ContentLines, l)
	}

	return m
}

func (m *Markdown) allSections() []*MarkdownSection {
	var result []*MarkdownSection

	for _, s := range m.sections {
		result = append(result, s.allSections()...)
	}

	return result
}

func (m *Markdown) AppendSection(s *MarkdownSection) {
	m.sections = append(m.sections, s)
}

func (m *Markdown) PrependSection(s *MarkdownSection) {
	m.sections = append([]*MarkdownSection{s}, m.sections...)
}

func (m *Markdown) FindSectionByHeading(level int, headline string) *MarkdownSection {
	for _, s := range m.allSections() {
		if s.Level == level {
			if headline == s.Heading {
				return s
			}
		}
	}

	return nil
}

func (m *Markdown) FindSectionByHeadingPrefix(level int, headlinePrefix string) *MarkdownSection {
	for _, s := range m.allSections() {
		if s.Level == level {
			if strings.HasPrefix(s.Heading, headlinePrefix) {
				return s
			}
		}
	}

	return nil
}

func (m *Markdown) String() string {
	var result string
	for _, s := range m.sections {
		result += "\n" + s.String()
		result = strings.Trim(result, "\n")
	}

	return strings.TrimSpace(result)
}
