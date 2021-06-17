package utils

import (
	"fmt"
	"strings"
)

type Markdown struct {
	sections []*MarkdownSection
}

func (m *Markdown) allSections() []*MarkdownSection {
	var result []*MarkdownSection

	for _, s := range m.sections {
		result = append(result, s.allSections()...)
	}

	return result
}

func (m *MarkdownSection) allSections() []*MarkdownSection {
	var result []*MarkdownSection

	result = append(result, m)

	for _, s := range m.SubSections {
		result = append(result, s.allSections()...)
	}

	return result
}

type MarkdownSection struct {
	Level        int
	Heading      string
	ContentLines []string
	SubSections  []*MarkdownSection
}

func ParseMarkdown(content string) *Markdown {
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

func isHeading(l string) bool {
	return strings.HasPrefix(l, "#")
}

func headingLevel(l string) int {
	level := 0
	for _, char := range l {
		if char != '#' {
			break
		}
		level++
	}
	return level
}

func (m *Markdown) AppendSection(s *MarkdownSection) {
	m.sections = append(m.sections, s)
}

func (m *Markdown) PrependSection(s *MarkdownSection) {
	m.sections = append([]*MarkdownSection{s}, m.sections...)
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
		result += s.String()
	}
	return strings.TrimSpace(result)
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

	for _, sub := range m.SubSections {
		result += sub.String()
	}

	return result
}
