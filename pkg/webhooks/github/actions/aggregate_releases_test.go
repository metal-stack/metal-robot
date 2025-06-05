package actions

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v3 "github.com/google/go-github/v72/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

func Test_sortComments(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		comments []*v3.IssueComment
		want     []*v3.IssueComment
	}{
		{
			name: "newest comment should appear first in the list",
			comments: []*v3.IssueComment{
				{
					ID:        pointer.Pointer(int64(1)),
					CreatedAt: &v3.Timestamp{Time: now.Add(-3 * time.Minute)},
				},
				{
					ID:        pointer.Pointer(int64(2)),
					CreatedAt: &v3.Timestamp{Time: now.Add(2 * time.Minute)},
				},
			},
			want: []*v3.IssueComment{
				{
					ID:        pointer.Pointer(int64(2)),
					CreatedAt: &v3.Timestamp{Time: now.Add(2 * time.Minute)},
				},
				{
					ID:        pointer.Pointer(int64(1)),
					CreatedAt: &v3.Timestamp{Time: now.Add(-3 * time.Minute)},
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
