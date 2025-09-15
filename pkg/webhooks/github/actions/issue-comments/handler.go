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
	logger *slog.Logger
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

func New(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
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
		logger:      logger,
		client:      client,
		targetRepos: targetRepos,
	}, nil
}

func (r *IssueCommentsAction) Handle(ctx context.Context, p *Params) error {
	_, ok := r.targetRepos[p.RepositoryName]
	if !ok {
		r.logger.Debug("skip handling issues comment action, not in list of target repositories", "source-repo", p.RepositoryName)
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
		r.logger.Debug("skip handling issues comment action, author does not have admin permissions on this repo", "source-repo", p.RepositoryName, "author", p.User)
		return nil
	}

	if _, ok := common.SearchForCommand(p.Comment, common.CommentBuildFork); ok {
		err := r.buildForkPR(ctx, p)
		if err != nil {
			return err
		}
	}

	if args, ok := common.SearchForCommand(p.Comment, common.CommentTag); ok {
		err := r.tag(ctx, p, args)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *IssueCommentsAction) buildForkPR(ctx context.Context, p *Params) error {
	pullRequest, _, err := r.client.GetV3Client().PullRequests.Get(ctx, r.client.Organization(), p.RepositoryName, p.PullRequestNumber)
	if err != nil {
		return fmt.Errorf("error finding issue related pull request %w", err)
	}

	if pullRequest.Head.Repo.Fork == nil || !*pullRequest.Head.Repo.Fork {
		r.logger.Debug("skip handling issues comment action, pull request is not from a fork", "source-repo", p.RepositoryName)
		return nil
	}

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token %w", err)
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
		return fmt.Errorf("error pushing to target remote repository %w", err)
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

	r.logger.Info("triggered fork build action by pushing to fork-build branch", "source-repo", p.RepositoryName, "branch", forkBuildBranch, "pull-request-url", forkPr.GetURL())

	return nil
}

func (r *IssueCommentsAction) tag(ctx context.Context, p *Params, args []string) error {
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
		return fmt.Errorf("error creating git token %w", err)
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

	r.logger.Info("pushed tag to repo", "repo", p.RepositoryName, "branch", headRef, "tag", tag)

	return nil
}
