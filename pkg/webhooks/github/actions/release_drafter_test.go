package actions

import (
	"testing"

	"github.com/blang/semver"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap/zaptest"
)

func TestReleaseDrafter_updateReleaseBody(t *testing.T) {
	tests := []struct {
		name            string
		releaseTemplate string

		version          string
		priorBody        string
		component        string
		componentVersion semver.Version
		componentBody    *string

		want string
	}{
		// 		{
		// 			name:             "creating fresh release draft",
		// 			version:          "v0.1.0",
		// 			component:        "metal-robot",
		// 			componentVersion: semver.MustParse("0.2.4"),
		// 			componentBody: v3.String(`- Adding new feature
		// - Fixed a bug`),
		// 			priorBody: "",
		// 			want: `# v0.1.0

		// ## metal-robot v0.2.4
		// - Adding new feature
		// - Fixed a bug
		// `,
		// 		},
		// 		{
		// 			name:             "creating fresh release draft, no release body",
		// 			version:          "v0.1.0",
		// 			component:        "metal-robot",
		// 			componentVersion: semver.MustParse("0.2.4"),
		// 			componentBody:    nil,
		// 			priorBody:        "",
		// 			want: `
		// # v0.1.0

		// ## metal-robot v0.2.4
		// `,
		// 		},
		// 		{
		// 			name:             "creating fresh release draft, empty release body",
		// 			version:          "v0.1.0",
		// 			component:        "metal-robot",
		// 			componentVersion: semver.MustParse("0.2.4"),
		// 			componentBody:    v3.String(""),
		// 			priorBody:        "",
		// 			want: `
		// # v0.1.0

		// ## metal-robot v0.2.4
		// `,
		// 		},
		// 		{
		// 			name:             "updating release draft with completely new release",
		// 			version:          "v0.1.0",
		// 			component:        "metal-robot",
		// 			componentVersion: semver.MustParse("0.2.4"),
		// 			componentBody: v3.String(`- Adding new feature
		// - Fixed a bug`),
		// 			priorBody: `# v0.1.0
		// ## metal-test v0.1.0

		// - 42`,
		// 			want: `
		// # v0.1.0

		// ## metal-test v0.1.0
		// - 42
		// ## metal-robot v0.2.4
		// - Adding new feature
		// - Fixed a bug`,
		// 		},
		// 		{
		// 			name:             "updating release draft with another component release",
		// 			version:          "v0.1.0",
		// 			component:        "metal-robot",
		// 			componentVersion: semver.MustParse("0.2.5"),
		// 			componentBody:    v3.String(`- Fixed yet another bug`),
		// 			priorBody: `# v0.1.0

		// ## metal-test v0.1.0
		// - 42
		// ## metal-robot v0.2.4
		// - Adding new feature
		// - Fixed a bug`,
		// 			want: `
		// # v0.1.0

		// ## metal-test v0.1.0
		// - 42
		// ## metal-robot v0.2.5
		// - Adding new feature
		// - Fixed a bug
		// - Fixed yet another bug`,
		// 		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReleaseDrafter{
				logger: zaptest.NewLogger(t).Sugar(),
				client: nil,
			}
			res := r.updateReleaseBody(tt.version, tt.priorBody, tt.component, tt.componentVersion, tt.componentBody)
			if diff := cmp.Diff(tt.want, res); diff != "" {
				t.Errorf("ReleaseDrafter.updateReleaseBody(), diff: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
			}
			idempont := r.updateReleaseBody(tt.version, res, tt.component, tt.componentVersion, tt.componentBody)
			if diff := cmp.Diff(tt.want, idempont); diff != "" {
				t.Errorf("not idempotent: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
			}
		})
	}
}

func Test_parseMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []*markdownSection
	}{
		{
			name:    "no sections",
			content: "test",
			want: []*markdownSection{
				{
					Level:   0,
					Heading: "",
					Content: "test",
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
			want: []*markdownSection{
				{
					Level:   0,
					Heading: "",
					Content: "pre-section\nwithout a level\n",
				},
				{
					Level:   1,
					Heading: "section 1",
					Content: "content 1\n",
				},
				{
					Level:   2,
					Heading: "section 2",
					Content: "content 2\nstill content 2",
				},
			},
		},
	}
	for _, tt := range tests {
		// regex := regexp.MustCompile("\n\n")
		t.Run(tt.name, func(t *testing.T) {
			m := parseMarkdown(tt.content)
			if diff := cmp.Diff(m.sections, tt.want); diff != "" {
				t.Errorf("parseMarkdown(), differs in sections: %v", diff)
			}
			// clean := regex.ReplaceAllString(tt.content, "\n")
			if diff := cmp.Diff(m.String(), tt.content); diff != "" {
				t.Errorf("String(), content has changed: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.content, m.String())
			}
		})
	}
}
