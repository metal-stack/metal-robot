package markdown

import (
	"fmt"
	"strings"
)

type MarkdownSection struct {
	Level        int
	Heading      string
	ContentLines []string
	SubSections  []*MarkdownSection
}

func (m *MarkdownSection) allSections() []*MarkdownSection {
	var result []*MarkdownSection

	result = append(result, m)

	for _, s := range m.SubSections {
		result = append(result, s.allSections()...)
	}

	return result
}

func (m *MarkdownSection) FindSectionByHeading(level int, headline string) *MarkdownSection {
	for _, s := range m.allSections() {
		if s.Level == level {
			if headline == s.Heading {
				return s
			}
		}
	}

	return nil
}

func (m *MarkdownSection) AppendContent(contentLines []string) {
	m.ContentLines = append(m.ContentLines, contentLines...)
}

func (m *MarkdownSection) PrependContent(contentLines []string) {
	m.ContentLines = append(contentLines, m.ContentLines...)
}

func (m *MarkdownSection) AppendChild(child *MarkdownSection) {
	m.SubSections = append(m.SubSections, child)
}

func (m *MarkdownSection) PrependChild(child *MarkdownSection) {
	m.SubSections = append([]*MarkdownSection{child}, m.SubSections...)
}

func (m *MarkdownSection) String() string {
	var result string

	if m.Level > 0 {
		for i := 0; i < m.Level; i++ {
			result += "#"
		}
		result += " " + m.Heading + "\n"
	}

	for _, l := range m.ContentLines {
		result += fmt.Sprintf("%s\n", l)
	}

	result = strings.Trim(result, "\n")

	for _, sub := range m.SubSections {
		result += "\n" + sub.String()
	}

	return result
}
