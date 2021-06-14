package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_parseMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []*MarkdownSection
	}{
		{
			name:    "empty document",
			content: "",
			want: []*MarkdownSection{
				{
					Level:   0,
					Heading: "",
					ContentLines: []string{
						"",
					},
				},
			},
		},
		{
			name:    "no sections",
			content: "test",
			want: []*MarkdownSection{
				{
					Level:   0,
					Heading: "",
					ContentLines: []string{
						"test",
					},
				},
			},
		},
		{
			name: "parses friendly sections",
			content: `pre-section
without a level

# section 1
content 1

## section 2
content 2
still content 2`,
			want: []*MarkdownSection{
				{
					Level:   0,
					Heading: "",
					ContentLines: []string{
						"pre-section",
						"without a level",
						"",
					},
				},
				{
					Level:   1,
					Heading: "section 1",
					ContentLines: []string{
						"content 1",
						"",
					},
				},
				{
					Level:   2,
					Heading: "section 2",
					ContentLines: []string{
						"content 2",
						"still content 2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		// regex := regexp.MustCompile("\n\n")
		t.Run(tt.name, func(t *testing.T) {
			m := ParseMarkdown(tt.content)
			if diff := cmp.Diff(m.sections, tt.want); diff != "" {
				t.Errorf("parseMarkdown(), differs in sections: %v", diff)
			}
			// clean := regex.ReplaceAllString(tt.content, "\n")
			if diff := cmp.Diff(tt.content, m.String()); diff != "" {
				t.Errorf("String(), content has changed: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.content, m.String())
			}
		})
	}
}

func TestMarkdown_EnsureSection(t *testing.T) {
	type section struct {
		level          int
		headlinePrefix *string
		headline       string
		contentLines   []string
	}
	tests := []struct {
		name    string
		content string
		prepend bool
		s       section
		want    string
	}{
		{
			name: "add new section",
			content: `# Markdown sample

Some introduction text.

## Next section

This is a next section.
`,
			s: section{
				level:          3,
				headlinePrefix: nil,
				headline:       "Third section",
				contentLines:   []string{"a", "b", "c"},
			},
			want: `# Markdown sample

Some introduction text.

## Next section

This is a next section.

### Third section
a
b
c`,
		},
		{
			name: "keep existing section",
			content: `# Markdown sample

Some introduction text.

## Next section

This is a next section.
`,
			s: section{
				level:          2,
				headlinePrefix: strPtr("Next"),
				headline:       "Next section",
				contentLines:   []string{"a", "b", "c"},
			},
			want: `# Markdown sample

Some introduction text.

## Next section

This is a next section.
`,
		},
		{
			name:    "prepend a section",
			prepend: true,
			content: `# PR Section

* A PR was merged
`,
			s: section{
				level:        1,
				headline:     "General",
				contentLines: []string{"a", "b", "c"},
			},

			want: `# General
a
b
c
# PR Section

* A PR was merged
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ParseMarkdown(tt.content)
			m.EnsureSection(tt.s.level, tt.s.headlinePrefix, tt.s.headline, tt.s.contentLines, tt.prepend)

			if diff := cmp.Diff(tt.want, m.String()); diff != "" {
				t.Errorf("String(), content was unexpected: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, m.String())
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
