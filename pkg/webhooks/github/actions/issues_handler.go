package actions

import (
	"context"
	"net/url"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type IssueCommentCommand string

const (
	IssueCommentCommandPrefix                     = "/"
	IssueCommentBuildFork     IssueCommentCommand = IssueCommentCommandPrefix + "ok-to-build"
)

var (
	IssueCommentCommands = map[IssueCommentCommand]bool{
		IssueCommentBuildFork: true,
	}

	AllowedAuthorAssociations = map[string]bool{
		"COLLABORATOR": true,
		"MEMBER":       true,
		"OWNER":        true,
	}
)

type IssuesAction struct {
	logger *zap.SugaredLogger
	client *clients.Github

	targetRepos map[string]targetRepo
}

type IssuesActionParams struct {
	PullRequestNumber int
	AuthorAssociation string
	RepositoryName    string
	Comment           string
	CommentID         int64
}

func NewIssuesAction(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*IssuesAction, error) {
	var typedConfig config.IssuesCommentHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	targetRepos := make(map[string]targetRepo)
	for _, t := range typedConfig.TargetRepos {
		targetRepos[t.RepositoryName] = targetRepo{
			url: t.RepositoryURL,
		}
	}

	return &IssuesAction{
		logger:      logger,
		client:      client,
		targetRepos: targetRepos,
	}, nil
}

func (r *IssuesAction) HandleIssueComment(ctx context.Context, p *IssuesActionParams) error {
	targetRepo, ok := r.targetRepos[p.RepositoryName]
	if !ok {
		r.logger.Debugw("skip handling issues comment action, not in list of target repositories", "source-repo", p.RepositoryName)
		return nil
	}

	_, ok = AllowedAuthorAssociations[p.AuthorAssociation]
	if !ok {
		r.logger.Debugw("skip handling issues comment action, author is not allowed", "source-repo", p.RepositoryName, "association", p.AuthorAssociation)
		return nil
	}

	comment := strings.TrimSpace(p.Comment)

	_, ok = IssueCommentCommands[IssueCommentCommand(comment)]
	if !ok {
		r.logger.Debugw("skip handling issues comment action, message does not contain a valid command", "source-repo", p.RepositoryName)
		return nil
	}

	switch IssueCommentCommand(comment) {
	case IssueCommentBuildFork:
		return r.buildForkPR(ctx, p, targetRepo)
	default:
		r.logger.Debugw("skip handling issues comment action, message does not contain a valid command", "source-repo", p.RepositoryName)
		return nil
	}
}

func (r *IssuesAction) buildForkPR(ctx context.Context, p *IssuesActionParams, targetRepo targetRepo) error {
	pullRequest, _, err := r.client.GetV3Client().PullRequests.Get(ctx, r.client.Organization(), p.RepositoryName, p.PullRequestNumber)
	if err != nil {
		return errors.Wrap(err, "error finding issue related pull request")
	}

	if pullRequest.Head.Repo.Fork == nil || !*pullRequest.Head.Repo.Fork {
		r.logger.Debugw("skip handling issues comment action, pull request is not from a fork", "source-repo", p.RepositoryName)
		return nil
	}

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating git token")
	}

	targetRepoURL, err := url.Parse(targetRepo.url)
	if err != nil {
		return err
	}
	targetRepoURL.User = url.UserPassword("x-access-token", token)

	headRef := *pullRequest.Head.Ref
	commitMessage := "Triggering fork build approved by maintainer"
	err = git.PushToRemote(*pullRequest.Head.Repo.CloneURL, headRef, targetRepoURL.String(), "fork-build/"+headRef, commitMessage)
	if err != nil {
		return errors.Wrap(err, "error pushing to target remote repository")
	}

	r.logger.Infow("triggered fork build action by pushing to fork-build branch", "source-repo", p.RepositoryName, "branch", headRef)

	_, _, err = r.client.GetV3Client().Reactions.CreateIssueCommentReaction(ctx, r.client.Organization(), p.RepositoryName, p.CommentID, "rocket")
	if err != nil {
		return errors.Wrap(err, "error creating issue comment reaction")
	}

	return nil
}
