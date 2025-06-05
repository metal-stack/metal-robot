package actions

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_searchForCommandInBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		search   IssueCommentCommand
		want     bool
		wantArgs []string
	}{
		{
			name:     "find in single line",
			body:     "/freeze",
			search:   IssueCommentReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name:   "no match",
			body:   "/foo",
			search: IssueCommentReleaseFreeze,
			want:   false,
		},
		{
			name:     "find with strip",
			body:     "  /freeze  ",
			search:   IssueCommentReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name: "find in multi line",
			body: `Release is frozen now.
			/freeze
			`,
			search:   IssueCommentReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name: "with args",
			body: `Tagging.
			/tag v0.1.17-rc.0
			`,
			search:   IssueCommentTag,
			want:     true,
			wantArgs: []string{"v0.1.17-rc.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, got := searchForCommandInBody(tt.body, tt.search)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
			if diff := cmp.Diff(gotArgs, tt.wantArgs); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}
