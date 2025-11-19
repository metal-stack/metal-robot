package common

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

type CommentCommands string

const (
	CommentCommandPrefix                          = "/"
	CommentCommandBuildFork       CommentCommands = CommentCommandPrefix + "ok-to-build"
	CommentCommandReleaseFreeze   CommentCommands = CommentCommandPrefix + "freeze"
	CommentCommandReleaseUnfreeze CommentCommands = CommentCommandPrefix + "unfreeze"
	CommentCommandTag             CommentCommands = CommentCommandPrefix + "tag"
)

var (
	AllCommentCommands = map[CommentCommands]bool{
		CommentCommandBuildFork:       true,
		CommentCommandReleaseFreeze:   true,
		CommentCommandReleaseUnfreeze: true,
		CommentCommandTag:             true,
	}
)

func SearchForCommentCommand(data string, want CommentCommands) ([]string, bool) {
	for line := range strings.SplitSeq(strings.ReplaceAll(data, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		cmd, args := CommentCommands(fields[0]), fields[1:]

		_, ok := AllCommentCommands[cmd]
		if !ok {
			continue
		}

		if cmd == want {
			return args, true
		}
	}

	return nil, false
}

func FindOpenReleasePR(ctx context.Context, client *github.Client, owner, repo, branch, base string) (*github.PullRequest, error) {
	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State: "open",
		Head:  branch,
		Base:  base,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list pull requests: %w", err)
	}

	if len(prs) == 1 {
		return prs[0], nil
	}

	return nil, nil
}

func IsReleaseFreeze(ctx context.Context, client *github.Client, number int, owner, repo string) (bool, error) {
	comments, _, err := client.Issues.ListComments(ctx, owner, repo, number, &github.IssueListCommentsOptions{
		Direction: github.Ptr("desc"),
	})
	if err != nil {
		return true, fmt.Errorf("unable to list pull request comments: %w", err)
	}

	// somehow the direction parameter has no effect, it's always sorted in the same way?
	// therefore sorting manually:
	sort.Slice(comments, sortComments(comments))

	for _, comment := range comments {
		if _, ok := SearchForCommentCommand(pointer.SafeDeref(comment.Body), CommentCommandReleaseFreeze); ok {
			return true, nil
		}

		if _, ok := SearchForCommentCommand(pointer.SafeDeref(comment.Body), CommentCommandReleaseUnfreeze); ok {
			return false, nil
		}
	}

	return false, nil
}

func sortComments(comments []*github.IssueComment) func(i, j int) bool {
	return func(i, j int) bool {
		return comments[j].CreatedAt.Before(comments[i].CreatedAt.Time)
	}
}
