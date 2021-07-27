package markdown

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	realWorldMarkdown = `# General
## mini-lab v0.1.3
* Make firewalls use default disk layout as well. (metal-stack/mini-lab#69) @Gerrit91
* Fix using docker hub credential secrets in CI. (metal-stack/mini-lab#68) @Gerrit91
* Mep 8 (metal-stack/mini-lab#67) @majst01
* Use .nip.io instead of .xip.io. (metal-stack/mini-lab#66) @Gerrit91
* Adapt commands for retrieving console password. (metal-stack/mini-lab#65) @Gerrit91
* update machine image (metal-stack/mini-lab#64) @GrigoriyMikhalkin
* Fix make dev (metal-stack/mini-lab#60) @droid42
* Use new metal-images with cloud-init support (metal-stack/mini-lab#63) @mwindower
* Add comment to docs when running make route target (metal-stack/mini-lab#57) @LimKianAn
## metal-api v0.15.1
* Fix for preallocation leftovers on allocation failures (metal-stack/metal-api#196) @majst01
## metal-hammer v0.9.1
* add support for x11dpu motherboards (metal-stack/metal-hammer#55) @majst01
# Merged Pull Requests
This is a list of pull requests that were merged since the last release.
The list does not contain pull requests from release-vector-repositories.
* Bump releases to version v0.7.0 (metal-stack/docs#65) @metal-robot[bot]
* updates and linter fixes (metal-stack/updater#4) @majst01
* Add release drafter action (metal-stack/updater#5) @majst01
* Bump metal-api to version v0.15.1 (metal-stack/metal-python#39) @metal-robot[bot]
* WIP: emit proper event when signature doesn't match (metal-stack/firewall-controller#94) @GrigoriyMikhalkin
* Support x11dpu (metal-stack/go-hal#37) @majst01`
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
still content 2
# section 1b
content 1b`,
			want: []*MarkdownSection{
				{
					Level:   0,
					Heading: "",
					ContentLines: []string{
						"pre-section",
						"without a level",
					},
					SubSections: []*MarkdownSection{
						{
							Level:   1,
							Heading: "section 1",
							ContentLines: []string{
								"content 1",
							},
							SubSections: []*MarkdownSection{
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
						{
							Level:   1,
							Heading: "section 1b",
							ContentLines: []string{
								"content 1b",
							},
						},
					},
				},
			},
		},
		{
			name:    "real-world scenario",
			content: realWorldMarkdown,
			want: []*MarkdownSection{
				{
					Level:        1,
					Heading:      "General",
					ContentLines: nil,
					SubSections: []*MarkdownSection{
						{
							Heading: "mini-lab v0.1.3",
							Level:   2,
							ContentLines: []string{
								"* Make firewalls use default disk layout as well. (metal-stack/mini-lab#69) @Gerrit91",
								"* Fix using docker hub credential secrets in CI. (metal-stack/mini-lab#68) @Gerrit91",
								"* Mep 8 (metal-stack/mini-lab#67) @majst01",
								"* Use .nip.io instead of .xip.io. (metal-stack/mini-lab#66) @Gerrit91",
								"* Adapt commands for retrieving console password. (metal-stack/mini-lab#65) @Gerrit91",
								"* update machine image (metal-stack/mini-lab#64) @GrigoriyMikhalkin",
								"* Fix make dev (metal-stack/mini-lab#60) @droid42",
								"* Use new metal-images with cloud-init support (metal-stack/mini-lab#63) @mwindower",
								"* Add comment to docs when running make route target (metal-stack/mini-lab#57) @LimKianAn",
							},
						},
						{
							Heading: "metal-api v0.15.1",
							Level:   2,
							ContentLines: []string{
								"* Fix for preallocation leftovers on allocation failures (metal-stack/metal-api#196) @majst01",
							},
						},
						{
							Heading: "metal-hammer v0.9.1",
							Level:   2,
							ContentLines: []string{
								"* add support for x11dpu motherboards (metal-stack/metal-hammer#55) @majst01",
							},
						},
					},
				},
				{
					Level:   1,
					Heading: "Merged Pull Requests",
					ContentLines: []string{
						"This is a list of pull requests that were merged since the last release.",
						"The list does not contain pull requests from release-vector-repositories.",
						"* Bump releases to version v0.7.0 (metal-stack/docs#65) @metal-robot[bot]",
						"* updates and linter fixes (metal-stack/updater#4) @majst01",
						"* Add release drafter action (metal-stack/updater#5) @majst01",
						"* Bump metal-api to version v0.15.1 (metal-stack/metal-python#39) @metal-robot[bot]",
						"* WIP: emit proper event when signature doesn't match (metal-stack/firewall-controller#94) @GrigoriyMikhalkin",
						"* Support x11dpu (metal-stack/go-hal#37) @majst01",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		// regex := regexp.MustCompile("\n\n")
		t.Run(tt.name, func(t *testing.T) {
			m := Parse(tt.content)
			if diff := cmp.Diff(m.sections, tt.want); diff != "" {
				t.Errorf("parseMarkdown(), differs in sections: %v", diff)
			}
			// clean := regex.ReplaceAllString(tt.content, "\n")
			if diff := cmp.Diff(tt.content, m.String()); diff != "" {
				t.Errorf("String(), content has changed: %v", diff)
				t.Logf("\nwant\n=====\n%s\n\ngot\n=====\n%s", tt.content, m.String())
			}
		})
	}
}
