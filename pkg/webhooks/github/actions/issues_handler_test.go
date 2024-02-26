package actions

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_searchForCommandInBody(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		search IssueCommentCommand
		want   bool
	}{
		{
			name:   "find in single line",
			body:   "/freeze",
			search: IssueCommentReleaseFreeze,
			want:   true,
		},
		{
			name:   "no match",
			body:   "/foo",
			search: IssueCommentReleaseFreeze,
			want:   false,
		},
		{
			name:   "find with strip",
			body:   "  /freeze  ",
			search: IssueCommentReleaseFreeze,
			want:   true,
		},
		{
			name: "find in multi line",
			body: `Release is frozen now.
			/freeze
			`,
			search: IssueCommentReleaseFreeze,
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchForCommandInBody(tt.body, tt.search)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}
