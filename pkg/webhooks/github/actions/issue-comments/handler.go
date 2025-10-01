package issue_comments

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/common"
	"github.com/mitchellh/mapstructure"
)

type IssueCommentsAction struct {
	client *clients.Github

	targetRepos map[string]bool
}

type Params struct {
	PullRequestNumber int
	RepositoryName    string
	RepositoryURL     string
	Comment           string
	CommentID         int64
	User              string
}

func New(client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
	var typedConfig config.IssueCommentsHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	targetRepos := make(map[string]bool)
	for name := range typedConfig.TargetRepos {
		targetRepos[name] = true
	}

	return &IssueCommentsAction{
		client:      client,
		targetRepos: targetRepos,
	}, nil
}

// Handle applies actions on issues comments, e.g. executes ad hoc commands
func (r *IssueCommentsAction) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	_, ok := r.targetRepos[p.RepositoryName]
	if !ok {
		log.Debug("skip handling issues comment action, repository not configured in metal-robot configuration")
		return nil
	}

	level, _, err := r.client.GetV3Client().Repositories.GetPermissionLevel(ctx, r.client.Organization(), p.RepositoryName, p.User)
	if err != nil {
		return fmt.Errorf("error determining collaborator status: %w", err)
	}

	switch *level.Permission {
	case "admin":
		// fallthrough
	default:
		log.Debug("skip handling issues comment action, author does not have admin permissions on this repo", "author", p.User)
		return nil
	}

	if _, ok := common.SearchForCommentCommand(p.Comment, common.CommentCommandBuildFork); ok {
		err := r.buildForkPR(ctx, log, p)
		if err != nil {
			return err
		}
	}

	if args, ok := common.SearchForCommentCommand(p.Comment, common.CommentCommandTag); ok {
		err := r.tag(ctx, log, p, args)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *IssueCommentsAction) buildForkPR(ctx context.Context, log *slog.Logger, p *Params) error {
	pullRequest, _, err := r.client.GetV3Client().PullRequests.Get(ctx, r.client.Organization(), p.RepositoryName, p.PullRequestNumber)
	if err != nil {
		return fmt.Errorf("error finding issue related pull request %w", err)
	}

	if pullRequest.Head.Repo.Fork == nil || !*pullRequest.Head.Repo.Fork {
		log.Debug("skip handling issues comment action, pull request is not from a fork")
		return nil
	}

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token: %w", err)
	}

	targetRepoURL, err := url.Parse(p.RepositoryURL)
	if err != nil {
		return err
	}
	targetRepoURL.User = url.UserPassword("x-access-token", token)

	var (
		commitMessage   = "Triggering fork build approved by maintainer"
		headRef         = *pullRequest.Head.Ref
		prNumber        = strconv.Itoa(*pullRequest.Number)
		forkBuildBranch = "fork-build/" + prNumber
		forkPrTitle     = "Fork build for #" + prNumber
	)

	err = git.PushToRemote(*pullRequest.Head.Repo.CloneURL, headRef, targetRepoURL.String(), forkBuildBranch, commitMessage)
	if err != nil {
		return fmt.Errorf("error pushing to target remote repository: %w", err)
	}

	forkPr, _, err := r.client.GetV3Client().PullRequests.Create(ctx, r.client.Organization(), p.RepositoryName, &github.NewPullRequest{
		Title:               github.Ptr(forkPrTitle),
		Head:                github.Ptr(forkBuildBranch),
		Base:                pullRequest.Base.Ref,
		Body:                github.Ptr("Fork build for #" + prNumber + " triggered by @" + p.User),
		MaintainerCanModify: github.Ptr(true),
		Draft:               github.Ptr(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return err
		}
	}

	log.Info("triggered fork build action by pushing to fork-build branch", "branch", forkBuildBranch, "pull-request-url", forkPr.GetURL())

	return nil
}

func (r *IssueCommentsAction) tag(ctx context.Context, log *slog.Logger, p *Params, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tag name given, skipping")
	}

	tag := args[0]

	pullRequest, _, err := r.client.GetV3Client().PullRequests.Get(ctx, r.client.Organization(), p.RepositoryName, p.PullRequestNumber)
	if err != nil {
		return fmt.Errorf("error finding issue related pull request %w", err)
	}

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token: %w", err)
	}

	targetRepoURL, err := url.Parse(p.RepositoryURL)
	if err != nil {
		return err
	}
	targetRepoURL.User = url.UserPassword("x-access-token", token)

	headRef := *pullRequest.Head.Ref
	err = git.CreateTag(targetRepoURL.String(), headRef, tag, p.User)
	if err != nil {
		return err
	}

	log.Info("pushed tag to repo", "branch", headRef, "tag", tag)

	return nil
}
