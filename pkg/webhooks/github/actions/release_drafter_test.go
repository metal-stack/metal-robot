package actions

import (
	"testing"

	"github.com/blang/semver"
	"github.com/google/go-cmp/cmp"
	v3 "github.com/google/go-github/v38/github"
	"go.uber.org/zap/zaptest"
)

func TestReleaseDrafter_updateReleaseBody(t *testing.T) {
	tests := []struct {
		name             string
		org              string
		headline         string
		priorBody        string
		component        string
		componentVersion semver.Version
		componentBody    *string
		releaseURL       string

		want string
	}{
		{
			name:             "creating fresh release draft",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody: v3.String(`- Adding new feature
- Fixed a bug`),
			priorBody: "",
			want: `# General
## Component Releases
### metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
		},
		{
			name:             "creating fresh release draft, no release body",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody:    nil,
			priorBody:        "",
			want: `# General
## Component Releases
### metal-robot v0.2.4`,
		},
		{
			name:             "creating fresh release draft, empty release body",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody:    v3.String(""),
			priorBody:        "",
			want: `# General
## Component Releases
### metal-robot v0.2.4`,
		},
		{
			name:             "adding new section to existing release draft",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody: v3.String(`- Adding new feature
- Fixed a bug`),
			priorBody: `# General
## Component Releases
### metal-test v0.1.0
- 42`,
			want: `# General
## Component Releases
### metal-test v0.1.0
- 42
### metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
		},
		{
			name:             "updating release draft with another component release",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.5"),
			componentBody:    v3.String(`## General Changes\r\n\r\n* Fix (#123) @Gerrit91\r\n`),
			priorBody: `# General
## Component Releases
### metal-test v0.1.0
- 42
### metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
			want: `# General
## Component Releases
### metal-test v0.1.0
- 42
### metal-robot v0.2.5
- Adding new feature
- Fixed a bug
* Fix (metal-stack/metal-robot#123) @Gerrit91`,
		},
		{
			name:             "updating release draft when there is a pull request summary",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.5"),
			componentBody:    v3.String(`## General Changes\r\n\r\n* Fix (#123) @Gerrit91\r\n`),
			priorBody: `# General
## Component Releases
### metal-test v0.1.0
- 42
# Merged Pull Requests
Some description
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
			want: `# General
## Component Releases
### metal-test v0.1.0
- 42
### metal-robot v0.2.5
* Fix (metal-stack/metal-robot#123) @Gerrit91
# Merged Pull Requests
Some description
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
		{
			name:             "extracting required actions",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.5"),
			componentBody:    v3.String("## General Changes\r\n\r\n* Fix (#123) @Gerrit91\r\n```ACTIONS_REQUIRED\r\nAPI has changed\r\n```"),
			priorBody: `# General
## Component Releases
### metal-test v0.1.0
- 42
# Merged Pull Requests
Some description
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
			want: `# General
## Required Actions
* API has changed
## Component Releases
### metal-test v0.1.0
- 42
### metal-robot v0.2.5
* Fix (metal-stack/metal-robot#123) @Gerrit91
# Merged Pull Requests
Some description
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
		{
			name:             "extracting required actions, empty release bidy",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.5"),
			componentBody:    v3.String("## General Changes\r\n\r\n* Fix (#123) @Gerrit91\r\n```ACTIONS_REQUIRED\r\nAPI has changed\r\n```"),
			releaseURL:       "https://some-url",
			want: `# General
## Required Actions
* API has changed ([release notes](https://some-url))
## Component Releases
### metal-robot v0.2.5
* Fix (metal-stack/metal-robot#123) @Gerrit91`,
		},
		{
			name:             "extracting breaking changes and required actions, empty release bidy",
			headline:         "General",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.5"),
			componentBody:    v3.String("## General Changes\r\n\r\n* Fix (#123) @Gerrit91\r\n```ACTIONS_REQUIRED\r\nAPI has changed\r\n```\r\n```BREAKING_CHANGE\r\nAPI has changed\r\n```"),
			releaseURL:       "https://some-url",
			want: `# General
## Breaking Changes
* API has changed ([release notes](https://some-url))
## Required Actions
* API has changed ([release notes](https://some-url))
## Component Releases
### metal-robot v0.2.5
* Fix (metal-stack/metal-robot#123) @Gerrit91`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r := &releaseDrafter{
				logger:        zaptest.NewLogger(t).Sugar(),
				client:        nil,
				draftHeadline: tt.headline,
			}
			res := r.updateReleaseBody(tt.org, tt.priorBody, tt.component, tt.componentVersion, tt.componentBody, tt.releaseURL)
			if diff := cmp.Diff(tt.want, res); diff != "" {
				t.Errorf("ReleaseDrafter.updateReleaseBody(), diff: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
			}
			idempotent := r.updateReleaseBody(tt.org, res, tt.component, tt.componentVersion, tt.componentBody, tt.releaseURL)
			if diff := cmp.Diff(tt.want, idempotent); diff != "" {
				t.Errorf("not idempotent: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
			}
		})
	}
}

func Test_releaseDrafter_appendPullRequest(t *testing.T) {
	tests := []struct {
		name        string
		org         string
		repo        string
		title       string
		number      int64
		author      string
		prBody      *string
		priorBody   string
		description string

		want string
	}{
		{
			name:      "creating fresh release draft",
			org:       "metal-stack",
			repo:      "metal-robot",
			title:     "Some new feature",
			number:    11,
			author:    "metal-robot",
			priorBody: "",
			want: `# Merged Pull Requests
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
		{
			name:        "updating release draft",
			org:         "metal-stack",
			repo:        "metal-robot",
			title:       "Some new feature",
			number:      11,
			author:      "metal-robot",
			description: "Some description",
			priorBody: `# General
## metal-test v0.1.0
- 42
## metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
			want: `# General
## metal-test v0.1.0
- 42
## metal-robot v0.2.4
- Adding new feature
- Fixed a bug
# Merged Pull Requests
Some description
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
		{
			name:   "append to release draft",
			org:    "metal-stack",
			repo:   "metal-robot",
			title:  "Second PR",
			number: 12,
			author: "metal-robot",
			priorBody: `# General
## metal-test v0.1.0
- 42
## metal-robot v0.2.4
- Adding new feature
- Fixed a bug
# Merged Pull Requests
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
			want: `# General
## metal-test v0.1.0
- 42
## metal-robot v0.2.4
- Adding new feature
- Fixed a bug
# Merged Pull Requests
* Some new feature (metal-stack/metal-robot#11) @metal-robot
* Second PR (metal-stack/metal-robot#12) @metal-robot`,
		},
		{
			name:      "creating fresh release draft with actions required",
			org:       "metal-stack",
			repo:      "metal-robot",
			title:     "Some new feature",
			number:    11,
			author:    "metal-robot",
			priorBody: "",
			prBody:    v3.String("This is a new feature\r\n```ACTIONS_REQUIRED\r\nAPI has changed\r\n```"),
			want: `# General
## Required Actions
* API has changed (metal-stack/metal-robot#11)
# Merged Pull Requests
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
		{
			name:      "creating fresh release draft with breaking change",
			org:       "metal-stack",
			repo:      "metal-robot",
			title:     "Some new feature",
			number:    11,
			author:    "metal-robot",
			priorBody: "",
			prBody:    v3.String("This is a new feature\r\n```BREAKING_CHANGE\r\nAPI has changed\r\n```"),
			want: `# General
## Breaking Changes
* API has changed (metal-stack/metal-robot#11)
# Merged Pull Requests
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				r := &releaseDrafter{
					logger:        zaptest.NewLogger(t).Sugar(),
					client:        nil,
					prHeadline:    "Merged Pull Requests",
					draftHeadline: "General",
				}
				if tt.description != "" {
					r.prDescription = &tt.description
				}
				res := r.appendPullRequest(tt.org, tt.priorBody, tt.repo, tt.title, tt.number, tt.author, tt.prBody)
				if diff := cmp.Diff(tt.want, res); diff != "" {
					t.Errorf("ReleaseDrafter.appendPullRequest(), diff: %v", diff)
					t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
				}
				idempotent := r.appendPullRequest(tt.org, tt.priorBody, tt.repo, tt.title, tt.number, tt.author, tt.prBody)
				if diff := cmp.Diff(tt.want, idempotent); diff != "" {
					t.Errorf("not idempotent: %v", diff)
					t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
				}
			})
		})
	}
}
