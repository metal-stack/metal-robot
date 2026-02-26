package issue_comments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	aggregate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/aggregate-releases"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/common"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
	"github.com/mitchellh/mapstructure"
)

type IssueCommentsAction struct {
	client *clients.Github

	aggregateReleaseHandlers map[string][]handlers.WebhookHandler[*aggregate_releases.Params]
}

type Params struct {
	PullRequestNumber int
	RepositoryName    string
	RepositoryURL     string
	Comment           string
	CommentID         int64
	User              string
}

func New(client *clients.Github, rawConfig map[string]any, aggregateReleaseHandlers map[string][]handlers.WebhookHandler[*aggregate_releases.Params]) (handlers.WebhookHandler[*Params], error) {
	var typedConfig config.IssueCommentsHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &IssueCommentsAction{
		client:                   client,
		aggregateReleaseHandlers: aggregateReleaseHandlers,
	}, nil
}

// Handle applies actions on issues comments, e.g. executes ad hoc commands
func (r *IssueCommentsAction) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	level, _, err := r.client.GetV3Client().Repositories.GetPermissionLevel(ctx, r.client.Organization(), p.RepositoryName, p.User)
	if err != nil {
		return fmt.Errorf("error determining collaborator status: %w", err)
	}

	switch perm := *level.Permission; perm {
	case "admin":
		// in this case we can continue
	default:
		return handlerrors.Skip("skip handling issues comment action, author %q does not have admin permissions on this repo (but only %q)", p.User, perm)
	}

	var (
		errs              []error
		executedSomething = false
		commandActions    = []struct {
			cmd common.CommentCommand
			fn  func(args []string) error
		}{
			{
				cmd: common.CommentCommandBuildFork,
				fn: func(_ []string) error {
					return r.buildForkPR(ctx, log, p)
				},
			},
			{
				cmd: common.CommentCommandTag,
				fn: func(args []string) error {
					return r.tag(ctx, log, p, args)
				},
			},
			{
				cmd: common.CommentCommandBumpRelease,
				fn: func(args []string) error {
					return r.bumpRelease(ctx, log, p, args)
				},
			},
		}
	)

	for _, action := range commandActions {
		if args, ok := common.SearchForCommentCommand(p.Comment, action.cmd); ok {
			log.Info("running issue comment command", "cmd", action.cmd, "args", args)

			executedSomething = true

			err := action.fn(args)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	if !executedSomething {
		return handlerrors.Skip("no comment command contained in issue command")
	}

	return errors.Join(errs...)
}

func (r *IssueCommentsAction) buildForkPR(ctx context.Context, log *slog.Logger, p *Params) error {
	pullRequest, _, err := r.client.GetV3Client().PullRequests.Get(ctx, r.client.Organization(), p.RepositoryName, p.PullRequestNumber)
	if err != nil {
		return fmt.Errorf("error finding issue related pull request: %w", err)
	}

	if pullRequest.Head.Repo.Fork == nil || !*pullRequest.Head.Repo.Fork {
		return handlerrors.Skip("skip handling issues comment action, pull request is not from a fork")
	}

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token: %w", err)
	}

	targetRepoURL, err := url.Parse(p.RepositoryURL)
	if err != nil {
		return fmt.Errorf("unable to parse repository url: %w", err)
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
		Title:               new(forkPrTitle),
		Head:                new(forkBuildBranch),
		Base:                pullRequest.Base.Ref,
		Body:                new("Fork build for #" + prNumber + " triggered by @" + p.User),
		MaintainerCanModify: new(true),
		Draft:               new(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return fmt.Errorf("unable to create pull request: %w", err)
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
		return fmt.Errorf("error finding issue related pull request: %w", err)
	}

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token: %w", err)
	}

	targetRepoURL, err := url.Parse(p.RepositoryURL)
	if err != nil {
		return fmt.Errorf("unable to parse repository url: %w", err)
	}
	targetRepoURL.User = url.UserPassword("x-access-token", token)

	headRef := *pullRequest.Head.Ref
	err = git.CreateTag(targetRepoURL.String(), headRef, tag, p.User)
	if err != nil {
		return fmt.Errorf("unable to create git tag: %w", err)
	}

	log.Info("pushed tag to repo", "branch", headRef, "tag", tag)

	return nil
}

func (r *IssueCommentsAction) bumpRelease(ctx context.Context, log *slog.Logger, p *Params, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no repo name given, skipping")
	}

	handlers, ok := r.aggregateReleaseHandlers[p.RepositoryName]
	if !ok || len(handlers) == 0 {
		return fmt.Errorf("no aggregate release handlers configured for %s, skipping issue comment action", p.RepositoryName)
	}

	var (
		repoName = args[0]
		version  string
		errs     []error
	)

	repo, _, err := r.client.GetV3AppClient().Repositories.Get(ctx, r.client.Organization(), repoName)
	if err != nil {
		return fmt.Errorf("unable to find given repository %s: %w", repoName, err)
	}

	if len(args) > 1 {
		version = args[1]
	} else {
		release, _, err := r.client.GetV3Client().Repositories.GetLatestRelease(ctx, r.client.Organization(), repoName)
		if err != nil {
			return fmt.Errorf("unable to figure out latest release version in repository %s: %w", repoName, err)
		}

		version = pointer.SafeDeref(release.TagName)
	}

	for _, handler := range handlers {
		log.Info("calling aggregate releases handler through issue comment action", "aggregation-repo", p.RepositoryName, "repo", repoName, "version", version)

		err := handler.Handle(ctx, log, &aggregate_releases.Params{
			RepositoryName: repoName,
			RepositoryURL:  pointer.SafeDeref(repo.HTMLURL),
			TagName:        version,
			Sender:         p.User,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}
