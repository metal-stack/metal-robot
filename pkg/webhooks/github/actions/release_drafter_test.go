package actions

import (
	"testing"

	"github.com/blang/semver"
	"github.com/google/go-cmp/cmp"
	v3 "github.com/google/go-github/v32/github"
	"go.uber.org/zap/zaptest"
)

func TestReleaseDrafter_updateReleaseBody(t *testing.T) {
	tests := []struct {
		name             string
		org              string
		version          string
		priorBody        string
		component        string
		componentVersion semver.Version
		componentBody    *string

		want string
	}{
		{
			name:             "creating fresh release draft",
			version:          "v0.1.0",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody: v3.String(`- Adding new feature
- Fixed a bug`),
			priorBody: "",
			want: `# v0.1.0


## metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
		},
		{
			name:             "creating fresh release draft, no release body",
			version:          "v0.1.0",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody:    nil,
			priorBody:        "",
			want: `# v0.1.0


## metal-robot v0.2.4`,
		},
		{
			name:             "creating fresh release draft, empty release body",
			version:          "v0.1.0",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody:    v3.String(""),
			priorBody:        "",
			want: `# v0.1.0


## metal-robot v0.2.4`,
		},
		{
			name:             "adding new section to existing release draft",
			version:          "v0.1.0",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.4"),
			componentBody: v3.String(`- Adding new feature
- Fixed a bug`),
			priorBody: `# v0.1.0


## metal-test v0.1.0
- 42
`,
			want: `# v0.1.0


## metal-test v0.1.0
- 42

## metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
		},
		{
			name:             "updating release draft with another component release",
			version:          "v0.1.0",
			org:              "metal-stack",
			component:        "metal-robot",
			componentVersion: semver.MustParse("0.2.5"),
			componentBody:    v3.String(`## General Changes\r\n\r\n* Fix (#123) @Gerrit91\r\n`),
			priorBody: `# v0.1.0


## metal-test v0.1.0
- 42

## metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
			want: `# v0.1.0


## metal-test v0.1.0
- 42

## metal-robot v0.2.5
* Fix (metal-stack/metal-robot#123) @Gerrit91
- Adding new feature
- Fixed a bug`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &releaseDrafter{
				logger: zaptest.NewLogger(t).Sugar(),
				client: nil,
			}
			res := r.updateReleaseBody(tt.version, tt.org, tt.priorBody, tt.component, tt.componentVersion, tt.componentBody)
			if diff := cmp.Diff(tt.want, res); diff != "" {
				t.Errorf("ReleaseDrafter.updateReleaseBody(), diff: %v", diff)
				t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
			}
			idempotent := r.updateReleaseBody(tt.version, tt.org, res, tt.component, tt.componentVersion, tt.componentBody)
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
			priorBody: `# v0.1.0

## metal-test v0.1.0
- 42

## metal-robot v0.2.4
- Adding new feature
- Fixed a bug`,
			want: `# v0.1.0

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
			priorBody: `# v0.1.0

## metal-test v0.1.0
- 42

## metal-robot v0.2.4
- Adding new feature
- Fixed a bug

# Merged Pull Requests
* Some new feature (metal-stack/metal-robot#11) @metal-robot
`,
			want: `# v0.1.0

## metal-test v0.1.0
- 42

## metal-robot v0.2.4
- Adding new feature
- Fixed a bug

# Merged Pull Requests
* Second PR (metal-stack/metal-robot#12) @metal-robot
* Some new feature (metal-stack/metal-robot#11) @metal-robot`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				r := &releaseDrafter{
					logger: zaptest.NewLogger(t).Sugar(),
					client: nil,
				}
				if tt.description != "" {
					r.prDescription = &tt.description
				}
				res := r.appendPullRequest("Merged Pull Requests", tt.org, tt.priorBody, tt.repo, tt.title, tt.number, tt.author)
				if diff := cmp.Diff(tt.want, res); diff != "" {
					t.Errorf("ReleaseDrafter.appendPullRequest(), diff: %v", diff)
					t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
				}
				idempotent := r.appendPullRequest("Merged Pull Requests", tt.org, tt.priorBody, tt.repo, tt.title, tt.number, tt.author)
				if diff := cmp.Diff(tt.want, idempotent); diff != "" {
					t.Errorf("not idempotent: %v", diff)
					t.Logf("want\n=====\n%s\n\ngot\n=====\n%s", tt.want, res)
				}
			})
		})
	}
}
