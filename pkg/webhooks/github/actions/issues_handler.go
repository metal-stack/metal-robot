package actions

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	"github.com/mitchellh/mapstructure"
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

	targetRepos map[string]bool
}

type IssuesActionParams struct {
	PullRequestNumber int
	AuthorAssociation string
	RepositoryName    string
	RepositoryURL     string
	Comment           string
	CommentID         int64
}

func NewIssuesAction(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]any) (*IssuesAction, error) {
	var typedConfig config.IssuesCommentHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	targetRepos := make(map[string]bool)
	for name := range typedConfig.TargetRepos {
		targetRepos[name] = true
	}

	return &IssuesAction{
		logger:      logger,
		client:      client,
		targetRepos: targetRepos,
	}, nil
}

func (r *IssuesAction) HandleIssueComment(ctx context.Context, p *IssuesActionParams) error {
	_, ok := r.targetRepos[p.RepositoryName]
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
		return r.buildForkPR(ctx, p)
	default:
		r.logger.Debugw("skip handling issues comment action, message does not contain a valid command", "source-repo", p.RepositoryName)
		return nil
	}
}

func (r *IssuesAction) buildForkPR(ctx context.Context, p *IssuesActionParams) error {
	pullRequest, _, err := r.client.GetV3Client().PullRequests.Get(ctx, r.client.Organization(), p.RepositoryName, p.PullRequestNumber)
	if err != nil {
		return fmt.Errorf("error finding issue related pull request %w", err)
	}

	if pullRequest.Head.Repo.Fork == nil || !*pullRequest.Head.Repo.Fork {
		r.logger.Debugw("skip handling issues comment action, pull request is not from a fork", "source-repo", p.RepositoryName)
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

	headRef := *pullRequest.Head.Ref
	commitMessage := "Triggering fork build approved by maintainer"
	err = git.PushToRemote(*pullRequest.Head.Repo.CloneURL, headRef, targetRepoURL.String(), "fork-build/"+headRef, commitMessage)
	if err != nil {
		return fmt.Errorf("error pushing to target remote repository %w", err)
	}

	r.logger.Infow("triggered fork build action by pushing to fork-build branch", "source-repo", p.RepositoryName, "branch", headRef)

	_, _, err = r.client.GetV3Client().Reactions.CreateIssueCommentReaction(ctx, r.client.Organization(), p.RepositoryName, p.CommentID, "rocket")
	if err != nil {
		return fmt.Errorf("error creating issue comment reaction %w", err)
	}

	err = git.DeleteBranch(targetRepoURL.String(), "fork-build/"+headRef)
	if err != nil {
		return err
	}

	return nil
}
