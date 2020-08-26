package actions

import (
	"testing"

	"github.com/blang/semver"
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
		// 			name: "creates nice release draft body",
		// 			releaseTemplate: `# $TITLE

		// ## $COMPONENT_RELEASE_INFO
		// `,
		// 			version:          "v0.1.0",
		// 			component:        "metal-robot",
		// 			componentVersion: semver.MustParse("0.2.4"),
		// 			componentBody: v3.String(`- Adding new feature
		// - Fixed a bug`),
		// 			priorBody: "## metal-robot v0.2.3",
		// 			want: `# v0.1.0

		// ## metal-robot v0.2.4

		// - Adding new feature
		// - Fixed a bug
		// `,
		// 		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReleaseDrafter{
				logger:          zaptest.NewLogger(t).Sugar(),
				client:          nil,
				releaseTemplate: tt.releaseTemplate,
			}
			if got := r.updateReleaseBody(tt.version, tt.priorBody, tt.component, tt.componentVersion, tt.componentBody); got != tt.want {
				t.Errorf("ReleaseDrafter.updateReleaseBody() = %v, want %v", got, tt.want)
			}
		})
	}
}
