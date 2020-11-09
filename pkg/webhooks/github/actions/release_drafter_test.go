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
		releaseTemplate  string
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
- Adding new feature
- Fixed a bug
* Fix (metal-stack/metal-robot#123) @Gerrit91`,
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
