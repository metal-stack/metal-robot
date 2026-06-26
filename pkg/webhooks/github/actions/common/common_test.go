package common

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v79/github"
)

func Test_searchForCommandInBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		search   CommentCommand
		want     bool
		wantArgs []string
	}{
		{
			name:     "find in single line",
			body:     "/freeze",
			search:   CommentCommandReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name:   "no match",
			body:   "/foo",
			search: CommentCommandReleaseFreeze,
			want:   false,
		},
		{
			name:     "find with strip",
			body:     "  /freeze  ",
			search:   CommentCommandReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name: "find in multi line",
			body: `Release is frozen now.
			/freeze
			`,
			search:   CommentCommandReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name: "with args",
			body: `Tagging.
			/tag v0.1.17-rc.0
			`,
			search:   CommentCommandTag,
			want:     true,
			wantArgs: []string{"v0.1.17-rc.0"},
		},
		{
			name: "find bump release command",
			body: `I am gonna bump this manually.
			/bump-release metal-robot v0.1.0
			`,
			search:   CommentCommandBumpRelease,
			want:     true,
			wantArgs: []string{"metal-robot", "v0.1.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, got := SearchForCommentCommand(tt.body, tt.search)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
			if diff := cmp.Diff(gotArgs, tt.wantArgs); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}

func Test_sortComments(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		comments []*github.IssueComment
		want     []*github.IssueComment
	}{
		{
			name: "newest comment should appear first in the list",
			comments: []*github.IssueComment{
				{
					ID:        new(int64(1)),
					CreatedAt: &github.Timestamp{Time: now.Add(-3 * time.Minute)},
				},
				{
					ID:        new(int64(2)),
					CreatedAt: &github.Timestamp{Time: now.Add(2 * time.Minute)},
				},
			},
			want: []*github.IssueComment{
				{
					ID:        new(int64(2)),
					CreatedAt: &github.Timestamp{Time: now.Add(2 * time.Minute)},
				},
				{
					ID:        new(int64(1)),
					CreatedAt: &github.Timestamp{Time: now.Add(-3 * time.Minute)},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Slice(tt.comments, sortComments(tt.comments))
			if diff := cmp.Diff(tt.comments, tt.want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}
