package actions

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_searchForCommandInComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		search  IssueCommentCommand
		want    bool
	}{
		{
			name:    "find in single line",
			comment: "/freeze",
			search:  IssueCommentReleaseFreeze,
			want:    true,
		},
		{
			name:    "no match",
			comment: "/foo",
			search:  IssueCommentReleaseFreeze,
			want:    false,
		},
		{
			name:    "find with strip",
			comment: "  /freeze  ",
			search:  IssueCommentReleaseFreeze,
			want:    true,
		},
		{
			name: "find in multi line",
			comment: `Release is frozen now.
			/freeze
			`,
			search: IssueCommentReleaseFreeze,
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchForCommandInComment(tt.comment, tt.search)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}
