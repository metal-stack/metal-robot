package common

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

func Test_searchForCommandInBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		search   CommentCommands
		want     bool
		wantArgs []string
	}{
		{
			name:     "find in single line",
			body:     "/freeze",
			search:   CommentReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name:   "no match",
			body:   "/foo",
			search: CommentReleaseFreeze,
			want:   false,
		},
		{
			name:     "find with strip",
			body:     "  /freeze  ",
			search:   CommentReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name: "find in multi line",
			body: `Release is frozen now.
			/freeze
			`,
			search:   CommentReleaseFreeze,
			want:     true,
			wantArgs: []string{},
		},
		{
			name: "with args",
			body: `Tagging.
			/tag v0.1.17-rc.0
			`,
			search:   CommentTag,
			want:     true,
			wantArgs: []string{"v0.1.17-rc.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, got := SearchForCommand(tt.body, tt.search)
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
					ID:        pointer.Pointer(int64(1)),
					CreatedAt: &github.Timestamp{Time: now.Add(-3 * time.Minute)},
				},
				{
					ID:        pointer.Pointer(int64(2)),
					CreatedAt: &github.Timestamp{Time: now.Add(2 * time.Minute)},
				},
			},
			want: []*github.IssueComment{
				{
					ID:        pointer.Pointer(int64(2)),
					CreatedAt: &github.Timestamp{Time: now.Add(2 * time.Minute)},
				},
				{
					ID:        pointer.Pointer(int64(1)),
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
